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
	var (
		in  = []byte(r.FormValue("body"))
		out []byte
		err error
	)
	if r.FormValue("imports") != "" {
		out, err = imports.Process(progName, in, nil)
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
