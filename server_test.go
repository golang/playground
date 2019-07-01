// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
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
		url        string
		statusCode int
		headers    map[string]string
		respBody   []byte
	}{
		{"foo.play.golang.org to play.golang.org", "https://foo.play.golang.org", http.StatusFound, map[string]string{"Location": "https://play.golang.org"}, nil},
		{"Non-existent page", "https://play.golang.org/foo", http.StatusNotFound, nil, nil},
		{"Unknown snippet", "https://play.golang.org/p/foo", http.StatusNotFound, nil, nil},
		{"Existing snippet", "https://play.golang.org/p/" + id, http.StatusOK, nil, nil},
		{"Plaintext snippet", "https://play.golang.org/p/" + id + ".go", http.StatusOK, nil, barBody},
		{"Download snippet", "https://play.golang.org/p/" + id + ".go?download=true", http.StatusOK, map[string]string{"Content-Disposition": fmt.Sprintf(`attachment; filename="%s.go"`, id)}, barBody},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(http.MethodGet, tc.url, nil)
		w := httptest.NewRecorder()
		s.handleEdit(w, req)
		resp := w.Result()
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
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("%s: ioutil.ReadAll(resp.Body): %v", tc.desc, err)
			}
			if !bytes.Equal(b, tc.respBody) {
				t.Errorf("%s: got unexpected body %q; want %q", tc.desc, b, tc.respBody)
			}
		}
	}
}

func TestShare(t *testing.T) {
	s, err := newServer(testingOptions(t))
	if err != nil {
		t.Fatalf("newServer(testingOptions(t)): %v", err)
	}

	const url = "https://play.golang.org/share"
	testCases := []struct {
		desc       string
		method     string
		statusCode int
		reqBody    []byte
		respBody   []byte
	}{
		{"OPTIONS no-op", http.MethodOptions, http.StatusOK, nil, nil},
		{"Non-POST request", http.MethodGet, http.StatusMethodNotAllowed, nil, nil},
		{"Standard flow", http.MethodPost, http.StatusOK, []byte("Snippy McSnipface"), []byte("N_M_YelfGeR")},
		{"Snippet too large", http.MethodPost, http.StatusRequestEntityTooLarge, make([]byte, maxSnippetSize+1), nil},
	}

	for _, tc := range testCases {
		req := httptest.NewRequest(tc.method, url, bytes.NewReader(tc.reqBody))
		w := httptest.NewRecorder()
		s.handleShare(w, req)
		resp := w.Result()
		if got, want := resp.StatusCode, tc.statusCode; got != want {
			t.Errorf("%s: got unexpected status code %d; want %d", tc.desc, got, want)
		}
		if tc.respBody != nil {
			defer resp.Body.Close()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("%s: ioutil.ReadAll(resp.Body): %v", tc.desc, err)
			}
			if !bytes.Equal(b, tc.respBody) {
				t.Errorf("%s: got unexpected body %q; want %q", tc.desc, b, tc.respBody)
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
		return nil
	})
	if err != nil {
		t.Fatalf("newServer(testingOptions(t)): %v", err)
	}
	testHandler := s.commandHandler("test", func(r *request) (*response, error) {
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
		resp := &response{Events: []Event{{r.Body, "stdout", 0}}}
		return resp, nil
	})

	testCases := []struct {
		desc       string
		method     string
		statusCode int
		reqBody    []byte
		respBody   []byte
	}{
		{"OPTIONS request", http.MethodOptions, http.StatusOK, nil, nil},
		{"GET request", http.MethodGet, http.StatusBadRequest, nil, nil},
		{"Empty POST", http.MethodPost, http.StatusBadRequest, nil, nil},
		{"Failed cmdFunc", http.MethodPost, http.StatusInternalServerError, []byte(`{"Body":"fail"}`), nil},
		{"Standard flow", http.MethodPost, http.StatusOK,
			[]byte(`{"Body":"ok"}`),
			[]byte(`{"Errors":"","Events":[{"Message":"ok","Kind":"stdout","Delay":0}],"Status":0,"IsTest":false,"TestsFailed":0}
`),
		},
		{"Errors in response", http.MethodPost, http.StatusOK,
			[]byte(`{"Body":"error"}`),
			[]byte(`{"Errors":"errors","Events":null,"Status":0,"IsTest":false,"TestsFailed":0}
`),
		},
		{"Out of memory error in response body event message", http.MethodPost, http.StatusInternalServerError,
			[]byte(`{"Body":"oom-error"}`), nil},
		{"Cannot allocate memory error in response body event message", http.MethodPost, http.StatusInternalServerError,
			[]byte(`{"Body":"allocate-memory-error"}`), nil},
		{"Out of memory error in response errors", http.MethodPost, http.StatusInternalServerError,
			[]byte(`{"Body":"oom-compile-error"}`), nil},
		{"Cannot allocate memory error in response errors", http.MethodPost, http.StatusInternalServerError,
			[]byte(`{"Body":"allocate-memory-compile-error"}`), nil},
	}

	for _, tc := range testCases {
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
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				t.Errorf("%s: ioutil.ReadAll(resp.Body): %v", tc.desc, err)
			}
			if !bytes.Equal(b, tc.respBody) {
				t.Errorf("%s: got unexpected body %q; want %q", tc.desc, b, tc.respBody)
			}
		}
	}
}

func TestAllowModuleDownloads(t *testing.T) {
	const envKey = "ALLOW_PLAY_MODULE_DOWNLOADS"
	defer func(old string) { os.Setenv(envKey, old) }(os.Getenv(envKey))

	tests := []struct {
		src  string
		env  string
		want bool
	}{
		{src: "package main", want: true},
		{src: "package main", env: "false", want: false},
		{src: `import "code.google.com/p/go-tour/"`, want: false},
	}
	for i, tt := range tests {
		if tt.env != "" {
			os.Setenv(envKey, tt.env)
		} else {
			os.Setenv(envKey, "true")
		}
		files, err := splitFiles([]byte(tt.src))
		if err != nil {
			t.Errorf("%d. splitFiles = %v", i, err)
			continue
		}
		got := allowModuleDownloads(files)
		if got != tt.want {
			t.Errorf("%d. allow = %v; want %v; files:\n%s", i, got, tt.want, filesAsString(files))
		}
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
