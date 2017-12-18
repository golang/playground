// Copyright 2012 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"go/format"
	"net/http"

	"golang.org/x/tools/imports"
)

type fmtResponse struct {
	Body  string
	Error string
}

func fmtHandler(w http.ResponseWriter, r *http.Request) {
	var (
		in  = []byte(r.FormValue("body"))
		out []byte
		err error
	)
	if r.FormValue("imports") != "" {
		out, err = imports.Process("prog.go", in, nil)
	} else {
		out, err = format.Source(in)
	}
	var resp fmtResponse
	if err != nil {
		resp.Error = err.Error()
	} else {
		resp.Body = string(out)
	}
	json.NewEncoder(w).Encode(resp)
}
