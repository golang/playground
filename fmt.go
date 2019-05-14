// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"go/format"
	"net/http"
	"strings"

	"golang.org/x/tools/imports"
)

type fmtResponse struct {
	Body  string
	Error string
}

func handleFmt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	fs, err := splitFiles([]byte(r.FormValue("body")))
	if err != nil {
		json.NewEncoder(w).Encode(fmtResponse{Error: err.Error()})
		return
	}

	fixImports := r.FormValue("imports") != ""
	for _, f := range fs.files {
		if !strings.HasSuffix(f, ".go") {
			continue
		}
		var out []byte
		var err error
		in := fs.m[f]
		if fixImports {
			// TODO: pass options to imports.Process so it
			// can find symbols in sibling files.
			out, err = imports.Process(progName, in, nil)
		} else {
			out, err = format.Source(in)
		}
		if err != nil {
			errMsg := err.Error()
			// Prefix the error returned by format.Source.
			if !strings.HasPrefix(errMsg, f) {
				errMsg = fmt.Sprintf("%v:%v", f, errMsg)
			}
			json.NewEncoder(w).Encode(fmtResponse{Error: errMsg})
			return
		}
		fs.AddFile(f, out)
	}

	json.NewEncoder(w).Encode(fmtResponse{Body: string(fs.Format())})
}
