// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(adg): add logging
// TODO(proppy): restrict memory use
// TODO(adg): send exit code to user

// Command sandbox is an HTTP server that takes requests containing go
// source files, and builds and executes them in a NaCl sanbox.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const maxRunTime = 500 * time.Millisecond

type Request struct {
	Body string
}

type Response struct {
	Errors string
	Events []Event
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "test" {
		test()
		return
	}
	http.HandleFunc("/compile", compileHandler)
	http.HandleFunc("/_ah/health", healthHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func compileHandler(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("error decoding request: %v", err), http.StatusBadRequest)
		return
	}
	resp, err := compileAndRun(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, fmt.Sprintf("error encoding response: %v", err), http.StatusInternalServerError)
		return
	}
}

func compileAndRun(req *Request) (*Response, error) {
	tmpDir, err := ioutil.TempDir("", "sandbox")
	if err != nil {
		return nil, fmt.Errorf("error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	in := filepath.Join(tmpDir, "main.go")
	if err := ioutil.WriteFile(in, []byte(req.Body), 0400); err != nil {
		return nil, fmt.Errorf("error creating temp file %q: %v", in, err)
	}

	fset := token.NewFileSet()

	f, err := parser.ParseFile(fset, in, nil, parser.PackageClauseOnly)
	if err == nil && f.Name.Name != "main" {
		return &Response{Errors: "package name must be main"}, nil
	}

	exe := filepath.Join(tmpDir, "a.out")
	cmd := exec.Command("go", "build", "-o", exe, in)
	cmd.Env = []string{"GOOS=nacl", "GOARCH=amd64p32", "GOPATH=" + os.Getenv("GOPATH")}
	if out, err := cmd.CombinedOutput(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// Return compile errors to the user.

			// Rewrite compiler errors to refer to 'prog.go'
			// instead of '/tmp/sandbox1234/main.go'.
			errs := strings.Replace(string(out), in, "prog.go", -1)

			// "go build", invoked with a file name, puts this odd
			// message before any compile errors; strip it.
			errs = strings.Replace(errs, "# command-line-arguments\n", "", 1)

			return &Response{Errors: errs}, nil
		}
		return nil, fmt.Errorf("error building go source: %v", err)
	}
	cmd = exec.Command("sel_ldr_x86_64", "-l", "/dev/null", "-S", "-e", exe)
	rec := new(Recorder)
	cmd.Stdout = rec.Stdout()
	cmd.Stderr = rec.Stderr()
	if err := runTimeout(cmd, maxRunTime); err != nil {
		if err == timeoutErr {
			return &Response{Errors: "process took too long"}, nil
		}
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, fmt.Errorf("error running sandbox: %v", err)
		}
	}
	events, err := rec.Events()
	if err != nil {
		return nil, fmt.Errorf("error decoding events: %v", err)
	}
	return &Response{Events: events}, nil
}

var timeoutErr = errors.New("process timed out")

func runTimeout(cmd *exec.Cmd, d time.Duration) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	errc := make(chan error, 1)
	go func() {
		errc <- cmd.Wait()
	}()
	t := time.NewTimer(d)
	select {
	case err := <-errc:
		t.Stop()
		return err
	case <-t.C:
		cmd.Process.Kill()
		return timeoutErr
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := healthCheck(); err != nil {
		http.Error(w, "Health check failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, "ok")
}

func healthCheck() error {
	resp, err := compileAndRun(&Request{Body: healthProg})
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

func test() {
	if err := healthCheck(); err != nil {
		log.Fatal(err)
	}
	for _, t := range tests {
		resp, err := compileAndRun(&Request{Body: t.prog})
		if err != nil {
			log.Fatal(err)
		}
		if t.errors != "" {
			if resp.Errors != t.errors {
				log.Fatalf("resp.Errors = %q, want %q", resp.Errors, t.errors)
			}
			continue
		}
		if resp.Errors != "" {
			log.Fatal(resp.Errors)
		}
		if len(resp.Events) != 1 || !strings.Contains(resp.Events[0].Message, t.want) {
			log.Fatalf("unexpected output: %v, want %q", resp.Events, t.want)
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
}
