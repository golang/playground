// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"cloud.google.com/go/compute/metadata"
	"golang.org/x/playground/internal/metrics"
	"google.golang.org/api/idtoken"
)

var log = newStdLogger()

var (
	runtests   = flag.Bool("runtests", false, "Run integration tests instead of Playground server.")
	backendURL = flag.String("backend-url", "", "URL for sandbox backend that runs Go binaries.")
)

func main() {
	flag.Parse()
	s, err := newServer(func(s *server) error {
		pid := projectID()
		if pid == "" {
			s.db = &inMemStore{}
		} else {
			u, err := snippetStoreURL(context.Background())
			if err != nil {
				return fmt.Errorf("could not determine snippet store URL: %v", err)
			}
			hc, err := idtoken.NewClient(context.Background(), u)
			if err != nil {
				return fmt.Errorf("could not create snippet store client: %v", err)
			}
			hc.Timeout = 10 * time.Second
			s.db = remoteStore{baseURL: u, hc: hc}
		}
		if caddr := os.Getenv("MEMCACHED_ADDR"); caddr != "" {
			s.cache = newGobCache(caddr)
			log.Printf("App (project ID: %q) is caching results", pid)
		} else {
			s.cache = (*gobCache)(nil) // Use a no-op cache implementation.
			log.Printf("App (project ID: %q) is NOT caching results", pid)
		}
		s.log = log
		if gotip := os.Getenv("GOTIP"); gotip == "true" {
			s.gotip = true
		}
		execpath, _ := os.Executable()
		if execpath != "" {
			if fi, _ := os.Stat(execpath); fi != nil {
				s.modtime = fi.ModTime()
			}
		}
		eh, err := newExamplesHandler(s.gotip, s.modtime)
		if err != nil {
			return err
		}
		s.examples = eh
		return nil
	}, enableMetrics)
	if err != nil {
		log.Fatalf("Error creating server: %v", err)
	}

	if *runtests {
		s.test()
		return
	}
	if *backendURL != "" {
		// TODO(golang.org/issue/25224) - Remove environment variable and use a flag.
		os.Setenv("SANDBOX_BACKEND_URL", *backendURL)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Get the backend dialer warmed up. This starts
	// RegionInstanceGroupDialer queries and health checks.
	go sandboxBackendClient()

	log.Printf("Listening on :%v ...", port)
	log.Fatalf("Error listening on :%v: %v", port, http.ListenAndServe(":"+port, s))
}

func enableMetrics(s *server) error {
	gr, err := metrics.GAEResource(context.Background())
	if err != nil {
		s.log.Printf("metrics.GAEResource() = _, %q", err)
	}
	ms, err := metrics.NewService(gr, views)
	if err != nil {
		s.log.Printf("Failed to initialize metrics: metrics.NewService() = _, %q. (not on GCP?)", err)
	}
	if ms != nil && !metadata.OnGCE() {
		s.mux.Handle("/metrics", ms)
	}
	return nil
}

func projectID() string {
	id, err := metadata.ProjectID()
	if err != nil && os.Getenv("GAE_INSTANCE") != "" {
		log.Fatalf("Could not determine the project ID: %v", err)
	}
	return id
}

// snippetStoreURL returns the URL of the snippetstore Cloud Run service
// deployed by deploy/deploy_snippetstore.json. SNIPPET_STORE_URL overrides it.
func snippetStoreURL(ctx context.Context) (string, error) {
	if v := os.Getenv("SNIPPET_STORE_URL"); v != "" {
		return v, nil
	}
	projectID, err := metadata.NumericProjectIDWithContext(ctx)
	if err != nil {
		return "", err
	}
	return "https://snippetstore-" + projectID + ".us-central1.run.app", nil
}
