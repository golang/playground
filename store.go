// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
)

var errSnippetNotFound = errors.New("snippet not found")

type store interface {
	PutSnippet(ctx context.Context, id string, snip *snippet) error
	GetSnippet(ctx context.Context, id string, snip *snippet) error
}

// remoteStore is a store backed by the snippetstore server
// (see the snippetstore directory).
type remoteStore struct {
	baseURL string
	hc      *http.Client // adds any authentication the server requires
}

func (s remoteStore) PutSnippet(ctx context.Context, id string, snip *snippet) error {
	req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/snippets", bytes.NewReader(snip.Body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	got, err := s.do(req)
	if err != nil {
		return err
	}
	if string(got) != id {
		return fmt.Errorf("snippet store returned ID %q, want %q", got, id)
	}
	return nil
}

func (s remoteStore) GetSnippet(ctx context.Context, id string, snip *snippet) error {
	req, err := http.NewRequestWithContext(ctx, "GET", s.baseURL+"/snippets/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	body, err := s.do(req)
	if err != nil {
		return err
	}
	snip.Body = body
	return nil
}

func (s remoteStore) do(req *http.Request) ([]byte, error) {
	resp, err := s.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode {
	case http.StatusOK:
		return b, nil
	case http.StatusNotFound:
		return nil, errSnippetNotFound
	}
	return nil, fmt.Errorf("snippet store: %s %s: %s: %s", req.Method, req.URL.Path, resp.Status, bytes.TrimSpace(b))
}

// inMemStore is a store backed by a map that should only be used for testing.
type inMemStore struct {
	sync.RWMutex
	m map[string]*snippet // key -> snippet
}

func (s *inMemStore) PutSnippet(_ context.Context, id string, snip *snippet) error {
	s.Lock()
	if s.m == nil {
		s.m = map[string]*snippet{}
	}
	b := make([]byte, len(snip.Body))
	copy(b, snip.Body)
	s.m[id] = &snippet{Body: b}
	s.Unlock()
	return nil
}

func (s *inMemStore) GetSnippet(_ context.Context, id string, snip *snippet) error {
	s.RLock()
	defer s.RUnlock()
	v, ok := s.m[id]
	if !ok {
		return errSnippetNotFound
	}
	*snip = *v
	return nil
}
