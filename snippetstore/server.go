// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"

	"golang.org/x/playground/internal/snippet"
)

const maxSnippetSize = 64 * 1024

var errNotFound = errors.New("snippet not found")

// backend stores snippet bodies by ID.
type backend interface {
	// get returns the body of the snippet with the given ID,
	// or errNotFound if there is none.
	get(ctx context.Context, id string) ([]byte, error)

	// insert stores a snippet with the given ID and body.
	// It must never modify an existing snippet: if a snippet with
	// the given ID already exists, insert leaves it alone and returns nil.
	insert(ctx context.Context, id string, body []byte) error
}

func newHandler(b backend) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /snippets", func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxSnippetSize))
		if err != nil {
			if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
				http.Error(w, "snippet is too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "error reading request body", http.StatusBadRequest)
			return
		}
		// The ID is always derived from the body here, never taken from
		// the caller, so a caller cannot store content under an ID of
		// its choosing.
		id := snippet.ID(body)
		if err := b.insert(r.Context(), id, body); err != nil {
			log.Printf("inserting snippet %s: %v", id, err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		io.WriteString(w, id)
	})
	mux.HandleFunc("GET /snippets/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		body, err := b.get(r.Context(), id)
		if errors.Is(err, errNotFound) {
			http.NotFound(w, r)
			return
		}
		if err != nil {
			log.Printf("getting snippet %s: %v", id, err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(body)
	})
	return mux
}
