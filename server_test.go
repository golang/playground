// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/google/go-cmp/cmp"
)

type testLogger struct {
	t *testing.T
}

func (l testLogger) Printf(format string, args ...interface{}) {
	l.t.Logf(format, args...)
}
func (l testLogger) Errorf(format string, args ...interface{}) {
	l.t.Errorf(format, args...)
}
func (l testLogger) Fatalf(format string, args ...interface{}) {
	l.t.Fatalf(format, args...)
}

func testingOptions(t *testing.T) func(s *server) error {
	return func(s *server) error {
		s.db = &inMemStore{}
		s.log = testLogger{t}
		var err error
		s.examples, err = newExamplesHandler(false, time.Now())
		if err != nil {
			return err
		}
		return nil
	}
}

func TestEdit(t *testing.T) {
	s, err := newServer(testingOptions(t))
	if err != nil {
		t.Fatalf("newServer(testingOptions(t)): %v", err)
	}
	id := "bar"
	barBody := []byte("Snippy McSnipface")
	snip := &snippet{Body: barBody}
	if err := s.db.PutSnippet(context.Background(), id, snip); err != nil {
		t.Fatalf("s.dbPutSnippet(context.Background(), %+v, %+v): %v", id, snip, err)
	}

	testCases := []struct {
		desc       string
		method     string
		url        string
		statusCode int
		headers    map[string]string
		respBody   []byte
	}{
		{"OPTIONS no-op", http.MethodOptions, "https://play.golang.org/p/foo", http.StatusOK, nil, nil},
		{"foo.play.golang.org to play.golang.org", http.MethodGet, "https://foo.play.golang.org", http.StatusFound, map[string]string{"Location": "https://play.golang.org"}, nil},
		{"Non-existent page", http.MethodGet, "https://play.golang.org/foo", http.StatusNotFound, nil, nil},
		{"Unknown snippet", http.MethodGet, "https://play.golang.org/p/foo", http.StatusNotFound, nil, nil},
		{"Existing snippet", http.MethodGet, "https://play.golang.org/p/" + id, http.StatusFound, nil, nil},
		{"Plaintext snippet", http.MethodGet, "https://play.golang.org/p/" + id + ".go", http.StatusOK, nil, barBody},
		{"Download snippet", http.MethodGet, "https://play.golang.org/p/" + id + ".go?download=true", http.StatusOK, map[string]string{"Content-Disposition": fmt.Sprintf(`attachment; filename="%s.go"`, id)}, barBody},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(tc.method, tc.url, nil)
		w := httptest.NewRecorder()
		s.handleEdit(w, req)
		resp := w.Result()
		corsHeader := "Access-Control-Allow-Origin"
		if got, want := resp.Header.Get(corsHeader), "*"; got != want {
			t.Errorf("%s: %q header: got %q; want %q", tc.desc, corsHeader, got, want)
		}
		if got, want := resp.StatusCode, tc.statusCode; got != want {
			t.Errorf("%s: got unexpected status code %d; want %d", tc.desc, got, want)
		}
		for k, v := range tc.headers {
			if got, want := resp.Header.Get(k), v; got != want {
				t.Errorf("Got header value %q of %q; want %q", k, got, want)
			}
		}
		if tc.respBody != nil {
			defer resp.Body.Close()
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("%s: io.ReadAll(resp.Body): %v", tc.desc, err)
			}
			if !bytes.Equal(b, tc.respBody) {
				t.Errorf("%s: got unexpected body %q; want %q", tc.desc, b, tc.respBody)
			}
		}
	}
}

func TestServer(t *testing.T) {
	s, err := newServer(testingOptions(t))
	if err != nil {
		t.Fatalf("newServer(testingOptions(t)): %v", err)
	}

	const shareURL = "https://play.golang.org/share"
	testCases := []struct {
		desc       string
		method     string
		url        string
		statusCode int
		reqBody    []byte
		respBody   []byte
	}{
		// Share tests.
		{"OPTIONS no-op", http.MethodOptions, shareURL, http.StatusOK, nil, nil},
		{"Non-POST request", http.MethodGet, shareURL, http.StatusMethodNotAllowed, nil, nil},
		{"Standard flow", http.MethodPost, shareURL, http.StatusOK, []byte("Snippy McSnipface"), []byte("N_M_YelfGeR")},
		{"Snippet too large", http.MethodPost, shareURL, http.StatusRequestEntityTooLarge, make([]byte, maxSnippetSize+1), nil},

		// Examples tests.
		{"Hello example", http.MethodGet, "https://play.golang.org/doc/play/hello.txt", http.StatusOK, nil, []byte("Hello")},
		{"HTTP example", http.MethodGet, "https://play.golang.org/doc/play/http.txt", http.StatusOK, nil, []byte("net/http")},
		// Gotip examples should not be available on the non-tip playground.
		{"Gotip example", http.MethodGet, "https://play.golang.org/doc/play/min.gotip.txt", http.StatusNotFound, nil, nil},

		{"Versions json", http.MethodGet, "https://play.golang.org/version", http.StatusOK, nil, []byte(runtime.Version())},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(tc.method, tc.url, bytes.NewReader(tc.reqBody))
		w := httptest.NewRecorder()
		s.mux.ServeHTTP(w, req)
		resp := w.Result()
		corsHeader := "Access-Control-Allow-Origin"
		if got, want := resp.Header.Get(corsHeader), "*"; got != want {
			t.Errorf("%s: %q header: got %q; want %q", tc.desc, corsHeader, got, want)
		}
		if got, want := resp.StatusCode, tc.statusCode; got != want {
			t.Errorf("%s: got unexpected status code %d; want %d", tc.desc, got, want)
		}
		if tc.respBody != nil {
			defer resp.Body.Close()
			b, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("%s: io.ReadAll(resp.Body): %v", tc.desc, err)
			}
			if !bytes.Contains(b, tc.respBody) {
				t.Errorf("%s: got unexpected body %q; want contains %q", tc.desc, b, tc.respBody)
			}
		}
	}
}

func TestNoTrailingUnderscore(t *testing.T) {
	const trailingUnderscoreSnip = `package main

import "unsafe"

type T struct{}

func (T) m1()                         {}
func (T) m2([unsafe.Sizeof(T.m1)]int) {}

func main() {}
`
	snip := &snippet{[]byte(trailingUnderscoreSnip)}
	if got, want := snip.ID(), "WCktUidLyc_3"; got != want {
		t.Errorf("got %q; want %q", got, want)
	}
}

func TestCommandHandler(t *testing.T) {
	s, err := newServer(func(s *server) error {
		s.db = &inMemStore{}
		// testLogger makes tests fail.
		// Should we verify that s.log.Errorf was called
		// instead of just printing or failing the test?
		s.log = newStdLogger()
		s.cache = new(inMemCache)
		var err error
		s.examples, err = newExamplesHandler(false, time.Now())
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("newServer(testingOptions(t)): %v", err)
	}
	testHandler := s.commandHandler("test", func(_ context.Context, r *request) (*response, error) {
		if r.Body == "fail" {
			return nil, fmt.Errorf("non recoverable")
		}
		if r.Body == "error" {
			return &response{Errors: "errors"}, nil
		}
		if r.Body == "oom-error" {
			// To throw an oom in a local playground instance, increase the server timeout
			// to 20 seconds (within sandbox.go), spin up the Docker instance and run
			// this code: https://play.golang.org/p/aaCv86m0P14.
			return &response{Events: []Event{{"out of memory", "stderr", 0}}}, nil
		}
		if r.Body == "allocate-memory-error" {
			return &response{Events: []Event{{"cannot allocate memory", "stderr", 0}}}, nil
		}
		if r.Body == "oom-compile-error" {
			return &response{Errors: "out of memory"}, nil
		}
		if r.Body == "allocate-memory-compile-error" {
			return &response{Errors: "cannot allocate memory"}, nil
		}
		if r.Body == "build-timeout-error" {
			return &response{Errors: goBuildTimeoutError}, nil
		}
		if r.Body == "run-timeout-error" {
			return &response{Errors: runTimeoutError}, nil
		}
		resp := &response{Events: []Event{{r.Body, "stdout", 0}}}
		return resp, nil
	})

	testCases := []struct {
		desc        string
		method      string
		statusCode  int
		reqBody     []byte
		respBody    []byte
		shouldCache bool
	}{
		{"OPTIONS request", http.MethodOptions, http.StatusOK, nil, nil, false},
		{"GET request", http.MethodGet, http.StatusBadRequest, nil, nil, false},
		{"Empty POST", http.MethodPost, http.StatusBadRequest, nil, nil, false},
		{"Failed cmdFunc", http.MethodPost, http.StatusInternalServerError, []byte(`{"Body":"fail"}`), nil, false},
		{"Standard flow", http.MethodPost, http.StatusOK,
			[]byte(`{"Body":"ok"}`),
			[]byte(`{"Errors":"","Events":[{"Message":"ok","Kind":"stdout","Delay":0}],"Status":0,"IsTest":false,"TestsFailed":0}
`),
			true},
		{"Cache-able Errors in response", http.MethodPost, http.StatusOK,
			[]byte(`{"Body":"error"}`),
			[]byte(`{"Errors":"errors","Events":null,"Status":0,"IsTest":false,"TestsFailed":0}
`),
			true},
		{"Out of memory error in response body event message", http.MethodPost, http.StatusInternalServerError,
			[]byte(`{"Body":"oom-error"}`), nil, false},
		{"Cannot allocate memory error in response body event message", http.MethodPost, http.StatusInternalServerError,
			[]byte(`{"Body":"allocate-memory-error"}`), nil, false},
		{"Out of memory error in response errors", http.MethodPost, http.StatusInternalServerError,
			[]byte(`{"Body":"oom-compile-error"}`), nil, false},
		{"Cannot allocate memory error in response errors", http.MethodPost, http.StatusInternalServerError,
			[]byte(`{"Body":"allocate-memory-compile-error"}`), nil, false},
		{
			desc:       "Build timeout error",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			reqBody:    []byte(`{"Body":"build-timeout-error"}`),
			respBody:   []byte(fmt.Sprintln(`{"Errors":"timeout running go build","Events":null,"Status":0,"IsTest":false,"TestsFailed":0}`)),
		},
		{
			desc:       "Run timeout error",
			method:     http.MethodPost,
			statusCode: http.StatusOK,
			reqBody:    []byte(`{"Body":"run-timeout-error"}`),
			respBody:   []byte(fmt.Sprintln(`{"Errors":"timeout running program","Events":null,"Status":0,"IsTest":false,"TestsFailed":0}`)),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, "/compile", bytes.NewReader(tc.reqBody))
			w := httptest.NewRecorder()
			testHandler(w, req)
			resp := w.Result()
			corsHeader := "Access-Control-Allow-Origin"
			if got, want := resp.Header.Get(corsHeader), "*"; got != want {
				t.Errorf("%s: %q header: got %q; want %q", tc.desc, corsHeader, got, want)
			}
			if got, want := resp.StatusCode, tc.statusCode; got != want {
				t.Errorf("%s: got unexpected status code %d; want %d", tc.desc, got, want)
			}
			if tc.respBody != nil {
				defer resp.Body.Close()
				b, err := io.ReadAll(resp.Body)
				if err != nil {
					t.Errorf("%s: io.ReadAll(resp.Body): %v", tc.desc, err)
				}
				if !bytes.Equal(b, tc.respBody) {
					t.Errorf("%s: got unexpected body %q; want %q", tc.desc, b, tc.respBody)
				}
			}

			// Test caching semantics.
			sbreq := new(request)             // A sandbox request, used in the cache key.
			json.Unmarshal(tc.reqBody, sbreq) // Ignore errors, request may be empty.
			gotCache := new(response)
			if err := s.cache.Get(cacheKey("test", sbreq.Body), gotCache); (err == nil) != tc.shouldCache {
				t.Errorf("s.cache.Get(%q, %v) = %v, shouldCache: %v", cacheKey("test", sbreq.Body), gotCache, err, tc.shouldCache)
			}
			wantCache := new(response)
			if tc.shouldCache {
				if err := json.Unmarshal(tc.respBody, wantCache); err != nil {
					t.Errorf("json.Unmarshal(%q, %v) = %v, wanted no error", tc.respBody, wantCache, err)
				}
			}
			if diff := cmp.Diff(wantCache, gotCache); diff != "" {
				t.Errorf("s.Cache.Get(%q) mismatch (-want +got):\n%s", cacheKey("test", sbreq.Body), diff)
			}
		})
	}
}

func TestPlaygroundGoproxy(t *testing.T) {
	const envKey = "PLAY_GOPROXY"
	defer os.Setenv(envKey, os.Getenv(envKey))

	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "missing", env: "", want: "https://proxy.golang.org"},
		{name: "set_to_default", env: "https://proxy.golang.org", want: "https://proxy.golang.org"},
		{name: "changed", env: "https://company.intranet", want: "https://company.intranet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != "" {
				if err := os.Setenv(envKey, tt.env); err != nil {
					t.Errorf("unable to set environment variable for test: %s", err)
				}
			} else {
				if err := os.Unsetenv(envKey); err != nil {
					t.Errorf("unable to unset environment variable for test: %s", err)
				}
			}
			got := playgroundGoproxy()
			if got != tt.want {
				t.Errorf("playgroundGoproxy = %s; want %s; env: %s", got, tt.want, tt.env)
			}
		})
	}
}

// inMemCache is a responseCache backed by a map. It is only suitable for testing.
type inMemCache struct {
	l sync.Mutex
	m map[string]*response
}

// Set implements the responseCache interface.
// Set stores a *response in the cache. It panics for other types to ensure test failure.
func (i *inMemCache) Set(key string, v interface{}) error {
	i.l.Lock()
	defer i.l.Unlock()
	if i.m == nil {
		i.m = make(map[string]*response)
	}
	i.m[key] = v.(*response)
	return nil
}

// Get implements the responseCache interface.
// Get fetches a *response from the cache, or returns a memcache.ErrcacheMiss.
// It panics for other types to ensure test failure.
func (i *inMemCache) Get(key string, v interface{}) error {
	i.l.Lock()
	defer i.l.Unlock()
	target := v.(*response)
	got, ok := i.m[key]
	if !ok {
		return memcache.ErrCacheMiss
	}
	*target = *got
	return nil
}
