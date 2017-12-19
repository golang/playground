// Copyright 2013 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/datastore"
	"golang.org/x/tools/godoc/static"
)

var datastoreClient *datastore.Client

func main() {
	flag.Parse()

	var err error
	datastoreClient, err = datastore.NewClient(context.Background(), projectID())
	if err != nil {
		log.Fatal(err)
	}

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
	http.HandleFunc("/_ah/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok")
	})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Listening on :%v ...", port)
	http.ListenAndServe(":"+port, nil)
}

func projectID() string {
	id := os.Getenv("DATASTORE_PROJECT_ID")
	if id != "" {
		return id
	}
	id, err := metadata.ProjectID()
	if err != nil {
		log.Fatalf("Could not determine the project ID (%v); If running locally, ensure DATASTORE_PROJECT_ID is set.", err)
	}
	return id
}

func play(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-type", "text/javascript")
	io.WriteString(w, static.Files["playground.js"])
}
