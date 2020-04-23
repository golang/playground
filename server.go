// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/tools/godoc/static"
)

type server struct {
	mux   *http.ServeMux
	db    store
	log   logger
	cache responseCache

	// When the executable was last modified. Used for caching headers of compiled assets.
	modtime time.Time
}

func newServer(options ...func(s *server) error) (*server, error) {
	s := &server{mux: http.NewServeMux()}
	for _, o := range options {
		if err := o(s); err != nil {
			return nil, err
		}
	}
	if s.db == nil {
		return nil, fmt.Errorf("must provide an option func that specifies a datastore")
	}
	if s.log == nil {
		return nil, fmt.Errorf("must provide an option func that specifies a logger")
	}
	execpath, _ := os.Executable()
	if execpath != "" {
		if fi, _ := os.Stat(execpath); fi != nil {
			s.modtime = fi.ModTime()
		}
	}
	s.init()
	return s, nil
}

func (s *server) init() {
	s.mux.HandleFunc("/", s.handleEdit)
	s.mux.HandleFunc("/fmt", handleFmt)
	s.mux.HandleFunc("/vet", s.commandHandler("vet", vetCheck))
	s.mux.HandleFunc("/compile", s.commandHandler("prog", compileAndRun))
	s.mux.HandleFunc("/share", s.handleShare)
	s.mux.HandleFunc("/playground.js", s.handlePlaygroundJS)
	s.mux.HandleFunc("/favicon.ico", handleFavicon)
	s.mux.HandleFunc("/_ah/health", s.handleHealthCheck)

	staticHandler := http.StripPrefix("/static/", http.FileServer(http.Dir("./static")))
	s.mux.Handle("/static/", staticHandler)

	examplesHandler := http.StripPrefix("/doc/play/", http.FileServer(http.Dir("./examples")))
	s.mux.Handle("/doc/play/", examplesHandler)
}

func (s *server) handlePlaygroundJS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/javascript; charset=utf-8")
	rd := strings.NewReader(static.Files["playground.js"])
	http.ServeContent(w, r, "playground.js", s.modtime, rd)
}

func handleFavicon(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/favicon.ico")
}

func (s *server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	if err := s.healthCheck(r.Context()); err != nil {
		http.Error(w, "Health check failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Fprint(w, "ok")
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Forwarded-Proto") == "http" {
		r.URL.Scheme = "https"
		r.URL.Host = r.Host
		http.Redirect(w, r, r.URL.String(), http.StatusFound)
		return
	}
	if r.Header.Get("X-Forwarded-Proto") == "https" {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; preload")
	}
	s.mux.ServeHTTP(w, r)
}
