// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(andybons): add logging
// TODO(andybons): restrict memory use
// TODO(andybons): send exit code to user

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	stdlog "log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

const (
	maxRunTime = 2 * time.Second

	// progName is the program name in compiler errors
	progName = "prog.go"
)

type request struct {
	Body string
}

type response struct {
	Errors string
	Events []Event
}

func (s *server) handleCompile(w http.ResponseWriter, r *http.Request) {
	var req request
	// Until programs that depend on golang.org/x/tools/godoc/static/playground.js
	// are updated to always send JSON, this check is in place.
	if b := r.FormValue("body"); b != "" {
		req.Body = b
	} else if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("error decoding request: %v", err), http.StatusBadRequest)
		return
	}

	resp := &response{}
	key := cacheKey(req.Body)
	if err := s.cache.Get(key, resp); err != nil {
		if err != memcache.ErrCacheMiss {
			s.log.Errorf("s.cache.Get(%q, &response): %v", key, err)
		}
		var err error
		resp, err = s.compileAndRun(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if err := s.cache.Set(key, resp); err != nil {
			s.log.Errorf("cache.Set(%q, resp): %v", key, err)
		}
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		http.Error(w, fmt.Sprintf("error encoding response: %v", err), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(w, &buf); err != nil {
		s.log.Errorf("io.Copy(w, %+v): %v", buf, err)
		return
	}
}

func cacheKey(body string) string {
	h := sha256.New()
	io.WriteString(h, body)
	return fmt.Sprintf("prog-%s-%x", runtime.Version(), h.Sum(nil))
}

// isTestFunc tells whether fn has the type of a testing function.
func isTestFunc(fn *ast.FuncDecl) bool {
	if fn.Type.Results != nil && len(fn.Type.Results.List) > 0 ||
		fn.Type.Params.List == nil ||
		len(fn.Type.Params.List) != 1 ||
		len(fn.Type.Params.List[0].Names) > 1 {
		return false
	}
	ptr, ok := fn.Type.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return false
	}
	// We can't easily check that the type is *testing.T
	// because we don't know how testing has been imported,
	// but at least check that it's *T or *something.T.
	if name, ok := ptr.X.(*ast.Ident); ok && name.Name == "T" {
		return true
	}
	if sel, ok := ptr.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "T" {
		return true
	}
	return false
}

// isTest tells whether name looks like a test (or benchmark, according to prefix).
// It is a Test (say) if there is a character after Test that is not a lower-case letter.
// We don't want TesticularCancer.
func isTest(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if len(name) == len(prefix) { // "Test" is ok
		return true
	}
	return ast.IsExported(name[len(prefix):])
}

// getTestMain returns sources with main function which runs all found tests in src.
// This happens if the main function is not present and there are appropriate test functions.
// Otherwise it returns nil.
// Examples are not supported yet. Benchmarks will never be supported because of sandboxing.
func getTestMain(src []byte) []byte {
	fset := token.NewFileSet()
	// Early bail for most cases.
	f, err := parser.ParseFile(fset, "main.go", src, parser.ImportsOnly)
	if err != nil || f.Name.Name != "main" {
		return nil
	}
	var testing bool
	for _, s := range f.Imports {
		if s.Path.Value == `"testing"` && s.Name == nil {
			testing = true
			break
		}
	}
	if !testing {
		return nil
	}

	// Parse everything and extract test names
	f, err = parser.ParseFile(fset, "main.go", src, parser.ParseComments)
	if err != nil {
		return nil
	}

	var tests []string
	for _, d := range f.Decls {
		n, ok := d.(*ast.FuncDecl)
		if !ok {
			continue
		}
		name := n.Name.Name
		switch {
		case name == "main":
			// main declared a method will not obstruct creation of our main function.
			if n.Recv == nil {
				return nil
			}
		case isTest(name, "Test") && isTestFunc(n):
			tests = append(tests, name)
		}
	}

	if len(tests) == 0 {
		return nil
	}
	code := new(bytes.Buffer)
	if err := testTmpl.Execute(code, tests); err != nil {
		panic(err)
	}
	return code.Bytes()
}

var testTmpl = template.Must(template.New("main").Parse(`
func main() {
	matchAll := func(t string, pat string) (bool, error) { return true, nil }
	tests := []testing.InternalTest{
{{range .}}
		{"{{.}}", {{.}}},
{{end}}
	}
	testing.Main(matchAll, tests, nil, nil)
}
`))

func (s *server) compileAndRun(req *request) (*response, error) {
	// TODO(andybons): Add semaphore to limit number of running programs at once.
	tmpDir, err := ioutil.TempDir("", "sandbox")
	if err != nil {
		return nil, fmt.Errorf("error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	src := []byte(req.Body)
	in := filepath.Join(tmpDir, "main.go")
	if err := ioutil.WriteFile(in, src, 0400); err != nil {
		return nil, fmt.Errorf("error creating temp file %q: %v", in, err)
	}

	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, in, nil, parser.PackageClauseOnly)
	if err == nil && f.Name.Name != "main" {
		return &response{Errors: "package name must be main"}, nil
	}

	var testParam string
	if code := getTestMain(src); code != nil {
		testParam = "-test.v"
		src = append(src, code...)
		if err := ioutil.WriteFile(in, src, 0400); err != nil {
			return nil, fmt.Errorf("error creating temp file %q: %v", in, err)
		}
	}

	exe := filepath.Join(tmpDir, "a.out")
	cmd := exec.Command("go", "build", "-o", exe, in)
	cmd.Env = []string{"GOOS=nacl", "GOARCH=amd64p32", "GOPATH=" + os.Getenv("GOPATH")}
	if out, err := cmd.CombinedOutput(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// Return compile errors to the user.

			// Rewrite compiler errors to refer to progName
			// instead of '/tmp/sandbox1234/main.go'.
			errs := strings.Replace(string(out), in, progName, -1)

			// "go build", invoked with a file name, puts this odd
			// message before any compile errors; strip it.
			errs = strings.Replace(errs, "# command-line-arguments\n", "", 1)

			return &response{Errors: errs}, nil
		}
		return nil, fmt.Errorf("error building go source: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), maxRunTime)
	defer cancel()
	cmd = exec.CommandContext(ctx, "sel_ldr_x86_64", "-l", "/dev/null", "-S", "-e", exe, testParam)
	rec := new(Recorder)
	cmd.Stdout = rec.Stdout()
	cmd.Stderr = rec.Stderr()
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return &response{Errors: "process took too long"}, nil
		}
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, fmt.Errorf("error running sandbox: %v", err)
		}
	}
	events, err := rec.Events()
	if err != nil {
		return nil, fmt.Errorf("error decoding events: %v", err)
	}
	return &response{Events: events}, nil
}

func (s *server) healthCheck() error {
	resp, err := s.compileAndRun(&request{Body: healthProg})
	if err != nil {
		return err
	}
	if resp.Errors != "" {
		return fmt.Errorf("compile error: %v", resp.Errors)
	}
	if len(resp.Events) != 1 || resp.Events[0].Message != "ok" {
		return fmt.Errorf("unexpected output: %v", resp.Events)
	}
	return nil
}

const healthProg = `
package main

import "fmt"

func main() { fmt.Print("ok") }
`

func (s *server) test() {
	if err := s.healthCheck(); err != nil {
		stdlog.Fatal(err)
	}
	for _, t := range tests {
		resp, err := s.compileAndRun(&request{Body: t.prog})
		if err != nil {
			stdlog.Fatal(err)
		}
		if t.errors != "" {
			if resp.Errors != t.errors {
				stdlog.Fatalf("resp.Errors = %q, want %q", resp.Errors, t.errors)
			}
			continue
		}
		if resp.Errors != "" {
			stdlog.Fatal(resp.Errors)
		}
		if len(resp.Events) != 1 || !strings.Contains(resp.Events[0].Message, t.want) {
			stdlog.Fatalf("unexpected output: %v, want %q", resp.Events, t.want)
		}
	}
	fmt.Println("OK")
}

var tests = []struct {
	prog, want, errors string
}{
	{prog: `
package main

import "time"

func main() {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err.Error())
	}
	println(loc.String())
}
`, want: "America/New_York"},

	{prog: `
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println(time.Now())
}
`, want: "2009-11-10 23:00:00 +0000 UTC"},

	{prog: `
package main

import (
	"fmt"
	"time"
)

func main() {
	t1 := time.Tick(time.Second * 3)
	t2 := time.Tick(time.Second * 7)
	t3 := time.Tick(time.Second * 11)
	end := time.After(time.Second * 19)
	want := "112131211"
	var got []byte
	for {
		var c byte
		select {
		case <-t1:
			c = '1'
		case <-t2:
			c = '2'
		case <-t3:
			c = '3'
		case <-end:
			if g := string(got); g != want {
				fmt.Printf("got %q, want %q\n", g, want)
			} else {
				fmt.Println("timers fired as expected")
			}
			return
		}
		got = append(got, c)
	}
}
`, want: "timers fired as expected"},

	{prog: `
package main

import (
	"code.google.com/p/go-tour/pic"
	"code.google.com/p/go-tour/reader"
	"code.google.com/p/go-tour/tree"
	"code.google.com/p/go-tour/wc"
)

var (
	_ = pic.Show
	_ = reader.Validate
	_ = tree.New
	_ = wc.Test
)

func main() {
	println("ok")
}
`, want: "ok"},
	{prog: `
package test

func main() {
	println("test")
}
`, want: "", errors: "package name must be main"},
	{prog: `
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	filepath.Walk("/", func(path string, info os.FileInfo, err error) error {
		fmt.Println(path)
		return nil
	})
}
`, want: `/
/dev
/dev/null
/dev/random
/dev/urandom
/dev/zero
/etc
/etc/group
/etc/hosts
/etc/passwd
/etc/resolv.conf
/tmp
/usr
/usr/local
/usr/local/go
/usr/local/go/lib
/usr/local/go/lib/time
/usr/local/go/lib/time/zoneinfo.zip`},
	{prog: `
package main

import "testing"

func TestSanity(t *testing.T) {
	if 1+1 != 2 {
		t.Error("uhh...")
	}
}
`, want: `=== RUN   TestSanity
--- PASS: TestSanity (0.00s)
PASS`},

	{prog: `
package main

func TestSanity(t *testing.T) {
	t.Error("uhh...")
}
`, want: "", errors: "prog.go:4:20: undefined: testing\n"},

	{prog: `
package main

import (
	"fmt"
	"testing"
)

func TestSanity(t *testing.T) {
	t.Error("uhh...")
}

func main() {
	fmt.Println("test")
}
`, want: "test"},
}
