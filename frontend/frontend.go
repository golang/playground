// Copyright 2013 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"io"
	"net/http"

	"golang.org/x/tools/godoc/static"
	"google.golang.org/appengine"
)

func main() {
	http.Handle("/", hstsHandler(edit))
	http.Handle("/compile", hstsHandler(compile))
	http.Handle("/fmt", hstsHandler(fmtHandler))
	http.Handle("/share", hstsHandler(share))
	http.Handle("/playground.js", hstsHandler(play))
	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("./static")))
	http.Handle("/static/", hstsHandler(staticHandler.(http.HandlerFunc)))
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/favicon.ico")
	})
	appengine.Main()
}

func play(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/javascript")
	io.WriteString(w, static.Files["playground.js"])
}
