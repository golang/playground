// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	snippetid "golang.org/x/playground/internal/snippet"
)

const maxSnippetSize = 64 * 1024

type snippet struct {
	Body []byte `datastore:",noindex"` // golang.org/issues/23253
}

func (s *snippet) ID() string {
	return snippetid.ID(s.Body)
}

func (s *server) handleShare(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == "OPTIONS" {
		// This is likely a pre-flight CORS request.
		return
	}
	if r.Method != "POST" {
		http.Error(w, "Requires POST", http.StatusMethodNotAllowed)
		return
	}

	var body bytes.Buffer
	_, err := io.Copy(&body, io.LimitReader(r.Body, maxSnippetSize+1))
	r.Body.Close()
	if err != nil {
		s.log.Errorf("reading Body: %v", err)
		http.Error(w, "Server Error", http.StatusInternalServerError)
		return
	}
	if body.Len() > maxSnippetSize {
		http.Error(w, "Snippet is too large", http.StatusRequestEntityTooLarge)
		return
	}

	snip := &snippet{Body: body.Bytes()}
	id := snip.ID()
	if err := s.db.PutSnippet(r.Context(), id, snip); err != nil {
		s.log.Errorf("putting Snippet: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, id)
}
