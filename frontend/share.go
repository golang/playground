// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"

	"cloud.google.com/go/datastore"
)

const salt = "[replace this with something unique]"

const maxSnippetSize = 64 * 1024

type snippet struct {
	Body []byte
}

func (s *snippet) ID() string {
	h := sha1.New()
	io.WriteString(h, salt)
	h.Write(s.Body)
	sum := h.Sum(nil)
	b := make([]byte, base64.URLEncoding.EncodedLen(len(sum)))
	base64.URLEncoding.Encode(b, sum)
	return string(b)[:10]
}

func share(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" {
		// This is likely a pre-flight CORS request.
		return
	}
	if r.Method != "POST" {
		status := http.StatusMethodNotAllowed
		http.Error(w, http.StatusText(status), status)
		return
	}
	if !allowShare(r) {
		status := http.StatusUnavailableForLegalReasons
		http.Error(w, http.StatusText(status), status)
		return
	}
	ctx := r.Context()

	var body bytes.Buffer
	_, err := io.Copy(&body, io.LimitReader(r.Body, maxSnippetSize+1))
	r.Body.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading Body: %v", err)
		http.Error(w, "Server Error", http.StatusInternalServerError)
		return
	}
	if body.Len() > maxSnippetSize {
		http.Error(w, "Snippet is too large", http.StatusRequestEntityTooLarge)
		return
	}

	snip := &snippet{Body: body.Bytes()}
	id := snip.ID()
	key := datastore.NameKey("Snippet", id, nil)
	_, err = datastoreClient.Put(ctx, key, snip)
	if err != nil {
		fmt.Fprintf(os.Stderr, "putting Snippet: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, id)
}

func allowShare(r *http.Request) bool {
	if os.Getenv("GAE_INSTANCE") == "" {
		return true
	}
	switch r.Header.Get("X-AppEngine-Country") {
	case "", "ZZ", "CN":
		return false
	}
	return true
}
