// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"net/http"
)

const runURL = "https://golang.org/compile?output=json"

func (s *server) handleCompile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if err := s.passThru(w, r); err != nil {
		http.Error(w, "Compile server error.", http.StatusInternalServerError)
		return
	}
}

func (s *server) passThru(w io.Writer, req *http.Request) error {
	defer req.Body.Close()
	r, err := http.Post(runURL, req.Header.Get("Content-type"), req.Body)
	if err != nil {
		s.log.Errorf("error making POST request: %v", err)
		return err
	}
	defer r.Body.Close()
	if _, err := io.Copy(w, r.Body); err != nil {
		s.log.Errorf("error copying response Body: %v", err)
		return err
	}
	return nil
}
