// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goplay

import (
	"fmt"
	"io"
	"net/http"

	"google.golang.org/appengine"
	"google.golang.org/appengine/log"
	"google.golang.org/appengine/urlfetch"
)

const runURL = "http://golang.org/compile?output=json"

func init() {
	http.Handle("/compile", hstsHandler(compile))
}

func compile(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		code := http.StatusMethodNotAllowed
		http.Error(w, http.StatusText(code), code)
		return
	}
	if err := passThru(w, r); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "Compile server error.")
	}
}

func passThru(w io.Writer, req *http.Request) error {
	c := appengine.NewContext(req)
	client := urlfetch.Client(c)
	defer req.Body.Close()
	r, err := client.Post(runURL, req.Header.Get("Content-type"), req.Body)
	if err != nil {
		log.Errorf(c, "making POST request: %v", err)
		return err
	}
	defer r.Body.Close()
	if _, err := io.Copy(w, r.Body); err != nil {
		log.Errorf(c, "copying response Body: %v", err)
		return err
	}
	return nil
}
