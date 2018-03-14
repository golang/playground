// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bradfitz/gomemcache/memcache"
)

// handleVet performs a vet check on source code, trying cache for results first.
// It serves an empty response if no errors were found by the "vet" tool.
func (s *server) handleVet(w http.ResponseWriter, r *http.Request) {
	// TODO(ysmolsky): refactor common code in this function and handleCompile.
	// See golang.org/issue/24535.
	var req request
	// Until programs that depend on golang.org/x/tools/godoc/static/playground.js
	// are updated to always send JSON, this check is in place.
	if b := r.FormValue("body"); b != "" {
		req.Body = b
	} else if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.log.Errorf("error decoding request: %v", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	resp := &response{}
	key := cacheKey("vet", req.Body)
	if err := s.cache.Get(key, resp); err != nil {
		if err != memcache.ErrCacheMiss {
			s.log.Errorf("s.cache.Get(%q, &response): %v", key, err)
		}
		resp, err = s.vetCheck(&req)
		if err != nil {
			s.log.Errorf("error checking vet: %v", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if err := s.cache.Set(key, resp); err != nil {
			s.log.Errorf("cache.Set(%q, resp): %v", key, err)
		}
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(resp); err != nil {
		s.log.Errorf("error encoding response: %v", err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(w, &buf); err != nil {
		s.log.Errorf("io.Copy(w, &buf): %v", err)
		return
	}
}

// vetCheck runs the "vet" tool on the source code in req.Body.
// In case of no errors it returns an empty, non-nil *response.
// Otherwise &response.Errors contains found errors.
func (s *server) vetCheck(req *request) (*response, error) {
	tmpDir, err := ioutil.TempDir("", "vet")
	if err != nil {
		return nil, fmt.Errorf("error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	in := filepath.Join(tmpDir, "main.go")
	if err := ioutil.WriteFile(in, []byte(req.Body), 0400); err != nil {
		return nil, fmt.Errorf("error creating temp file %q: %v", in, err)
	}

	cmd := exec.Command("go", "vet", in)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return &response{}, nil
	}

	if _, ok := err.(*exec.ExitError); !ok {
		return nil, fmt.Errorf("error vetting go source: %v", err)
	}

	// Rewrite compiler errors to refer to progName
	// instead of '/tmp/sandbox1234/main.go'.
	errs := strings.Replace(string(out), in, progName, -1)

	// "go vet", invoked with a file name, puts this odd
	// message before any compile errors; strip it.
	errs = strings.Replace(errs, "# command-line-arguments\n", "", 1)

	return &response{Errors: errs}, nil
}
