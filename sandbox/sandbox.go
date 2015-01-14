// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// TODO(adg): add logging

// Command sandbox is an HTTP server that takes requests containing go
// source files, and builds and executes them in a NaCl sanbox.
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

type Request struct {
	Body string
}

type Response struct {
	Errors string
	Events []Event
}

func compileHandler(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("request error: %v", err), http.StatusBadRequest)
		return
	}
	resp, err := compileAndRun(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("sandbox error: %v", err), http.StatusInternalServerError)
		return
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, fmt.Sprintf("response error: %v", err), http.StatusInternalServerError)
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
	exe := filepath.Join(tmpDir, "a.out")
	cmd := exec.Command("go", "build", "-o", exe, in)
	cmd.Env = []string{"GOOS=nacl", "GOARCH=amd64p32"}
	if out, err := cmd.CombinedOutput(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// Build error.
			return &Response{Errors: string(out)}, nil
		}
		return nil, fmt.Errorf("error building go source: %v", err)
	}
	// TODO(proppy): restrict run time and memory use.
	cmd = exec.Command("sel_ldr_x86_64", "-l", "/dev/null", "-S", "-e", exe)
	rec := new(Recorder)
	cmd.Stdout = rec.Stdout()
	cmd.Stderr = rec.Stderr()
	if err := cmd.Run(); err != nil {
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

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if err := healthCheck(); err != nil {
		http.Error(w, "Health check failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, "ok")
}

const healthProg = `package main;import "fmt";func main(){fmt.Print("ok")}`

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

func main() {
	http.HandleFunc("/compile", compileHandler)
	http.HandleFunc("/_ah/health", healthHandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
