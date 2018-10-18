// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"go/format"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/tools/imports"
)

type fmtResponse struct {
	Body  string
	Error string
}

func handleFmt(w http.ResponseWriter, r *http.Request) {
	var (
		in  = []byte(r.FormValue("body"))
		out []byte
		err error
	)
	if r.FormValue("imports") != "" {
		gopath := os.Getenv("GOPATH")
		tmpDir := filepath.Join(gopath, "src", "sandbox")
		err = exec.Command("mkdir", "-p", tmpDir).Run()
		sourcePath := filepath.Join(tmpDir, "main.go")
		ioutil.WriteFile(sourcePath, in, 0400)

		currentDir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
		err = os.Chdir(tmpDir)
		if err != nil {
			panic(err)
		}
		depInit := exec.Command("go", "get", ".")
		if result, err := depInit.CombinedOutput(); err != nil {
			fmt.Println(string(result))
			// Ignore error. コンパイル時エラーがあるときに失敗するので
		}

		os.Chdir(currentDir)

		out, err = imports.Process(sourcePath, nil, nil)
	} else {
		out, err = format.Source(in)
	}
	var resp fmtResponse
	if err != nil {
		resp.Error = err.Error()
		// Prefix the error returned by format.Source.
		if !strings.HasPrefix(resp.Error, progName) {
			resp.Error = fmt.Sprintf("%v:%v", progName, resp.Error)
		}
	} else {
		resp.Body = string(out)
	}
	json.NewEncoder(w).Encode(resp)
}
