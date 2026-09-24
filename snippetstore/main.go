// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Snippetstore stores shared playground snippets in Datastore on behalf of the
// playground frontend, which has no Datastore access of its own.
//
// It computes snippet IDs itself and never overwrites an existing snippet,
// so a misbehaving frontend can add new snippets but cannot modify existing
// ones or reach any other data. It serves:
//
//	POST /snippets       stores the request body and responds with its ID
//	GET  /snippets/{id}  responds with the body of the snippet
//
// It runs on Cloud Run; see deploy/deploy_snippetstore.json.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/datastore"
)

func main() {
	c, err := datastore.NewClient(context.Background(), datastore.DetectProjectID)
	if err != nil {
		log.Fatalf("creating datastore client: %v", err)
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Listening on :%v ...", port)
	log.Fatal(http.ListenAndServe(":"+port, newHandler(datastoreBackend{client: c})))
}
