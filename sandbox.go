// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(andybons): add logging
// TODO(andybons): restrict memory use

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/token"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"text/template"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
)

const (
	maxRunTime = 2 * time.Second

	// progName is the implicit program name written to the temp
	// dir and used in compiler and vet errors.
	progName = "prog.go"
)

// Responses that contain these strings will not be cached due to
// their non-deterministic nature.
var nonCachingErrors = []string{"out of memory", "cannot allocate memory"}

type request struct {
	Body    string
	WithVet bool // whether client supports vet response in a /compile request (Issue 31970)
}

type response struct {
	Errors      string
	Events      []Event
	Status      int
	IsTest      bool
	TestsFailed int

	// VetErrors, if non-empty, contains any vet errors. It is
	// only populated if request.WithVet was true.
	VetErrors string `json:",omitempty"`
	// VetOK reports whether vet ran & passsed. It is only
	// populated if request.WithVet was true. Only one of
	// VetErrors or VetOK can be non-zero.
	VetOK bool `json:",omitempty"`
}

// commandHandler returns an http.HandlerFunc.
// This handler creates a *request, assigning the "Body" field a value
// from the "body" form parameter or from the HTTP request body.
// If there is no cached *response for the combination of cachePrefix and request.Body,
// handler calls cmdFunc and in case of a nil error, stores the value of *response in the cache.
// The handler returned supports Cross-Origin Resource Sharing (CORS) from any domain.
func (s *server) commandHandler(cachePrefix string, cmdFunc func(*request) (*response, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == "OPTIONS" {
			// This is likely a pre-flight CORS request.
			return
		}

		var req request
		// Until programs that depend on golang.org/x/tools/godoc/static/playground.js
		// are updated to always send JSON, this check is in place.
		if b := r.FormValue("body"); b != "" {
			req.Body = b
			req.WithVet, _ = strconv.ParseBool(r.FormValue("withVet"))
		} else if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			s.log.Errorf("error decoding request: %v", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}

		resp := &response{}
		key := cacheKey(cachePrefix, req.Body)
		if err := s.cache.Get(key, resp); err != nil {
			if err != memcache.ErrCacheMiss {
				s.log.Errorf("s.cache.Get(%q, &response): %v", key, err)
			}
			resp, err = cmdFunc(&req)
			if err != nil {
				s.log.Errorf("cmdFunc error: %v", err)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			for _, e := range nonCachingErrors {
				if strings.Contains(resp.Errors, e) {
					s.log.Errorf("cmdFunc compilation error: %q", resp.Errors)
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
			}
			for _, el := range resp.Events {
				if el.Kind != "stderr" {
					continue
				}
				for _, e := range nonCachingErrors {
					if strings.Contains(el.Message, e) {
						s.log.Errorf("cmdFunc runtime error: %q", el.Message)
						http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
						return
					}
				}
			}
			if err := s.cache.Set(key, resp); err != nil {
				s.log.Errorf("cache.Set(%q, resp): %v", key, err)
			}
		}

		var buf bytes.Buffer
		if err := json.NewEncoder(&buf).Encode(resp); err != nil {
			s.log.Errorf("error encoding response: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if _, err := io.Copy(w, &buf); err != nil {
			s.log.Errorf("io.Copy(w, &buf): %v", err)
			return
		}
	}
}

func cacheKey(prefix, body string) string {
	h := sha256.New()
	io.WriteString(h, body)
	return fmt.Sprintf("%s-%s-%x", prefix, runtime.Version(), h.Sum(nil))
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

// getTestProg returns source code that executes all valid tests and examples in src.
// If the main function is present or there are no tests or examples, it returns nil.
// getTestProg emulates the "go test" command as closely as possible.
// Benchmarks are not supported because of sandboxing.
func getTestProg(src []byte) []byte {
	fset := token.NewFileSet()
	// Early bail for most cases.
	f, err := parser.ParseFile(fset, progName, src, parser.ImportsOnly)
	if err != nil || f.Name.Name != "main" {
		return nil
	}

	// importPos stores the position to inject the "testing" import declaration, if needed.
	importPos := fset.Position(f.Name.End()).Offset

	var testingImported bool
	for _, s := range f.Imports {
		if s.Path.Value == `"testing"` && s.Name == nil {
			testingImported = true
			break
		}
	}

	// Parse everything and extract test names.
	f, err = parser.ParseFile(fset, progName, src, parser.ParseComments)
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
			// main declared as a method will not obstruct creation of our main function.
			if n.Recv == nil {
				return nil
			}
		case isTest(name, "Test") && isTestFunc(n):
			tests = append(tests, name)
		}
	}

	// Tests imply imported "testing" package in the code.
	// If there is no import, bail to let the compiler produce an error.
	if !testingImported && len(tests) > 0 {
		return nil
	}

	// We emulate "go test". An example with no "Output" comment is compiled,
	// but not executed. An example with no text after "Output:" is compiled,
	// executed, and expected to produce no output.
	var ex []*doc.Example
	// exNoOutput indicates whether an example with no output is found.
	// We need to compile the program containing such an example even if there are no
	// other tests or examples.
	exNoOutput := false
	for _, e := range doc.Examples(f) {
		if e.Output != "" || e.EmptyOutput {
			ex = append(ex, e)
		}
		if e.Output == "" && !e.EmptyOutput {
			exNoOutput = true
		}
	}

	if len(tests) == 0 && len(ex) == 0 && !exNoOutput {
		return nil
	}

	if !testingImported && (len(ex) > 0 || exNoOutput) {
		// In case of the program with examples and no "testing" package imported,
		// add import after "package main" without modifying line numbers.
		importDecl := []byte(`;import "testing";`)
		src = bytes.Join([][]byte{src[:importPos], importDecl, src[importPos:]}, nil)
	}

	data := struct {
		Tests    []string
		Examples []*doc.Example
	}{
		tests,
		ex,
	}
	code := new(bytes.Buffer)
	if err := testTmpl.Execute(code, data); err != nil {
		panic(err)
	}
	src = append(src, code.Bytes()...)
	return src
}

var testTmpl = template.Must(template.New("main").Parse(`
func main() {
	matchAll := func(t string, pat string) (bool, error) { return true, nil }
	tests := []testing.InternalTest{
{{range .Tests}}
		{"{{.}}", {{.}}},
{{end}}
	}
	examples := []testing.InternalExample{
{{range .Examples}}
		{"Example{{.Name}}", Example{{.Name}}, {{printf "%q" .Output}}, {{.Unordered}}},
{{end}}
	}
	testing.Main(matchAll, tests, nil, examples)
}
`))

var failedTestPattern = "--- FAIL"

// compileAndRun tries to build and run a user program.
// The output of successfully ran program is returned in *response.Events.
// If a program cannot be built or has timed out,
// *response.Errors contains an explanation for a user.
func compileAndRun(req *request) (*response, error) {
	// TODO(andybons): Add semaphore to limit number of running programs at once.
	tmpDir, err := ioutil.TempDir("", "sandbox")
	if err != nil {
		return nil, fmt.Errorf("error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	files, err := splitFiles([]byte(req.Body))
	if err != nil {
		return &response{Errors: err.Error()}, nil
	}

	var testParam string
	var buildPkgArg = "."
	if files.Num() == 1 && len(files.Data(progName)) > 0 {
		buildPkgArg = progName
		src := files.Data(progName)
		if code := getTestProg(src); code != nil {
			testParam = "-test.v"
			files.AddFile(progName, code)
		}
	}

	useModules := allowModuleDownloads(files)
	if !files.Contains("go.mod") && useModules {
		files.AddFile("go.mod", []byte("module play\n"))
	}

	for f, src := range files.m {
		// Before multi-file support we required that the
		// program be in package main, so continue to do that
		// for now. But permit anything in subdirectories to have other
		// packages.
		if !strings.Contains(f, "/") {
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, f, src, parser.PackageClauseOnly)
			if err == nil && f.Name.Name != "main" {
				return &response{Errors: "package name must be main"}, nil
			}
		}

		in := filepath.Join(tmpDir, f)
		if strings.Contains(f, "/") {
			if err := os.MkdirAll(filepath.Dir(in), 0755); err != nil {
				return nil, err
			}
		}
		if err := ioutil.WriteFile(in, src, 0644); err != nil {
			return nil, fmt.Errorf("error creating temp file %q: %v", in, err)
		}
	}

	exe := filepath.Join(tmpDir, "a.out")
	goCache := filepath.Join(tmpDir, "gocache")
	cmd := exec.Command("go", "build", "-o", exe, buildPkgArg)
	cmd.Dir = tmpDir
	var goPath string
	cmd.Env = []string{"GOOS=nacl", "GOARCH=amd64p32", "GOCACHE=" + goCache}
	if useModules {
		// Create a GOPATH just for modules to be downloaded
		// into GOPATH/pkg/mod.
		goPath, err = ioutil.TempDir("", "gopath")
		if err != nil {
			return nil, fmt.Errorf("error creating temp directory: %v", err)
		}
		defer os.RemoveAll(goPath)
		cmd.Env = append(cmd.Env, "GO111MODULE=on", "GOPROXY="+playgroundGoproxy())
	} else {
		goPath = os.Getenv("GOPATH")                 // contains old code.google.com/p/go-tour, etc
		cmd.Env = append(cmd.Env, "GO111MODULE=off") // in case it becomes on by default later
	}
	cmd.Env = append(cmd.Env, "GOPATH="+goPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// Return compile errors to the user.

			// Rewrite compiler errors to strip the tmpDir name.
			errs := strings.Replace(string(out), tmpDir+"/", "", -1)

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
	var status int
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			// Send what was captured before the timeout.
			events, err := rec.Events()
			if err != nil {
				return nil, fmt.Errorf("error decoding events: %v", err)
			}
			return &response{Errors: "process took too long", Events: events}, nil
		}
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			return nil, fmt.Errorf("error running sandbox: %v", err)
		}
		if ws, ok := exitErr.Sys().(syscall.WaitStatus); ok {
			status = ws.ExitStatus()
		}
	}
	events, err := rec.Events()
	if err != nil {
		return nil, fmt.Errorf("error decoding events: %v", err)
	}
	var fails int
	if testParam != "" {
		// In case of testing the TestsFailed field contains how many tests have failed.
		for _, e := range events {
			fails += strings.Count(e.Message, failedTestPattern)
		}
	}
	var vetOut string
	if req.WithVet {
		vetOut, err = vetCheckInDir(tmpDir, goPath, useModules)
		if err != nil {
			return nil, fmt.Errorf("running vet: %v", err)
		}
	}
	return &response{
		Events:      events,
		Status:      status,
		IsTest:      testParam != "",
		TestsFailed: fails,
		VetErrors:   vetOut,
		VetOK:       req.WithVet && vetOut == "",
	}, nil
}

// allowModuleDownloads reports whether the code snippet in src should be allowed
// to download modules.
func allowModuleDownloads(files *fileSet) bool {
	if files.Num() == 1 && bytes.Contains(files.Data(progName), []byte(`"code.google.com/p/go-tour/`)) {
		// This domain doesn't exist anymore but we want old snippets using
		// these packages to still run, so the Dockerfile adds these packages
		// at this name in $GOPATH. Any snippets using this old name wouldn't
		// have expected (or been able to use) third-party packages anyway,
		// so disabling modules and proxy fetches is acceptable.
		return false
	}
	v, _ := strconv.ParseBool(os.Getenv("ALLOW_PLAY_MODULE_DOWNLOADS"))
	return v
}

// playgroundGoproxy returns the GOPROXY environment config the playground should use.
// It is fetched from the environment variable PLAY_GOPROXY. A missing or empty
// value for PLAY_GOPROXY returns the default value of https://proxy.golang.org.
func playgroundGoproxy() string {
	proxypath := os.Getenv("PLAY_GOPROXY")
	if proxypath != "" {
		return proxypath
	}
	return "https://proxy.golang.org"
}

func (s *server) healthCheck() error {
	resp, err := compileAndRun(&request{Body: healthProg})
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
