// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

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
	"path"
)

type Request struct {
	Body string
}

type Response struct {
	Errors string
	Events []Event
}

func handleCompile(w http.ResponseWriter, r *http.Request) {
	var codeRequest Request
	if err := json.NewDecoder(r.Body).Decode(&codeRequest); err != nil {
		http.Error(w, fmt.Sprintf("request error: %v", err), http.StatusBadRequest)
		return
	}
	resp, err := compileAndRun(&codeRequest)
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

	in := path.Join(tmpDir, "main.go")
	if err := ioutil.WriteFile(in, []byte(req.Body), 0400); err != nil {
		return nil, fmt.Errorf("error creating temp file %q: %v", in, err)
	}
	exe := path.Join(tmpDir, "a.out")
	cmd := exec.Command("go", "build", "-o", exe, in)
	cmd.Env = []string{
		"GOOS=nacl",
		"GOARCH=amd64p32",
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			// build error
			return &Response{
				Errors: string(out),
			}, nil
		}
		return nil, fmt.Errorf("error building go source: %v", err)
	}
	// TODO(proppy): restrict run time and memory use.
	cmd = exec.Command("sel_ldr_x86_64", "-l", "/dev/null", "-S", "-e", exe)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return nil, fmt.Errorf("error running sandbox: %v", err)
		}
	}
	events, err := Decode(out)
	if err != nil {
		return nil, fmt.Errorf("error decoding events: %v", err)
	}
	return &Response{
		Events: events,
	}, nil
}

func main() {
	http.HandleFunc("/compile", handleCompile)
	http.HandleFunc("/_ah/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
