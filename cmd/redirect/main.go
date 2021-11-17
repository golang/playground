// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// redirect serves an http server that redirects to the URL specified by the
// environment variable PLAY_REDIRECT.
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	redirect := os.Getenv("PLAY_REDIRECT")
	if redirect == "" {
		redirect = "https://play.golang.org"
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, redirect+r.URL.Path, http.StatusFound)
	})

	log.Printf("Listening on :%v ...", port)
	log.Fatalf("Error listening on :%v: %v", port, http.ListenAndServe(":"+port, handler))
}
