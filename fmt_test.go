// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHandleFmt(t *testing.T) {
	for _, tt := range []struct {
		name    string
		method  string
		body    string
		imports bool
		want    string
		wantErr string
	}{
		{
			name:   "OPTIONS no-op",
			method: http.MethodOptions,
		},
		{
			name:   "classic",
			method: http.MethodPost,
			body:   " package main\n    func main( ) {  }\n",
			want:   "package main\n\nfunc main() {}\n",
		},
		{
			name:    "classic_goimports",
			method:  http.MethodPost,
			body:    " package main\nvar _ = fmt.Printf",
			imports: true,
			want:    "package main\n\nimport \"fmt\"\n\nvar _ = fmt.Printf\n",
		},
		{
			name:   "single_go_with_header",
			method: http.MethodPost,
			body:   "-- prog.go --\n  package main",
			want:   "-- prog.go --\npackage main\n",
		},
		{
			name:   "multi_go_with_header",
			method: http.MethodPost,
			body:   "-- prog.go --\n  package main\n\n\n-- two.go --\n   package main\n  var X = 5",
			want:   "-- prog.go --\npackage main\n-- two.go --\npackage main\n\nvar X = 5\n",
		},
		{
			name:   "multi_go_without_header",
			method: http.MethodPost,
			body:   "    package main\n\n\n-- two.go --\n   package main\n  var X = 5",
			want:   "package main\n-- two.go --\npackage main\n\nvar X = 5\n",
		},
		{
			name:   "single_go.mod_with_header",
			method: http.MethodPost,
			body:   "-- go.mod --\n   module   \"foo\"   ",
			want:   "-- go.mod --\nmodule foo\n",
		},
		{
			name:   "multi_go.mod_with_header",
			method: http.MethodPost,
			body:   "-- a/go.mod --\n  module foo\n\n\n-- b/go.mod --\n   module  \"bar\"",
			want:   "-- a/go.mod --\nmodule foo\n-- b/go.mod --\nmodule bar\n",
		},
		{
			name:   "only_format_go_and_go.mod",
			method: http.MethodPost,
			body: "    package   main   \n\n\n" +
				"-- go.mod --\n   module   foo   \n\n\n" +
				"-- plain.txt --\n   plain   text   \n\n\n",
			want: "package main\n-- go.mod --\nmodule foo\n-- plain.txt --\n   plain   text   \n\n\n",
		},
		{
			name:    "error_gofmt",
			method:  http.MethodPost,
			body:    "package 123\n",
			wantErr: "prog.go:1:9: expected 'IDENT', found 123",
		},
		{
			name:    "error_gofmt_with_header",
			method:  http.MethodPost,
			body:    "-- dir/one.go --\npackage 123\n",
			wantErr: "dir/one.go:1:9: expected 'IDENT', found 123",
		},
		{
			name:    "error_goimports",
			method:  http.MethodPost,
			body:    "package 123\n",
			imports: true,
			wantErr: "prog.go:1:9: expected 'IDENT', found 123",
		},
		{
			name:    "error_goimports_with_header",
			method:  http.MethodPost,
			body:    "-- dir/one.go --\npackage 123\n",
			imports: true,
			wantErr: "dir/one.go:1:9: expected 'IDENT', found 123",
		},
		{
			name:    "error_go.mod",
			method:  http.MethodPost,
			body:    "-- go.mod --\n123\n",
			wantErr: "go.mod:1: unknown directive: 123",
		},
		{
			name:    "error_go.mod_with_header",
			method:  http.MethodPost,
			body:    "-- dir/go.mod --\n123\n",
			wantErr: "dir/go.mod:1: unknown directive: 123",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			form := url.Values{}
			form.Set("body", tt.body)
			if tt.imports {
				form.Set("imports", "true")
			}
			req := httptest.NewRequest("POST", "/fmt", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			handleFmt(rec, req)
			resp := rec.Result()
			if resp.StatusCode != 200 {
				t.Fatalf("code = %v", resp.Status)
			}
			corsHeader := "Access-Control-Allow-Origin"
			if got, want := resp.Header.Get(corsHeader), "*"; got != want {
				t.Errorf("Header %q: got %q; want %q", corsHeader, got, want)
			}
			if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
				t.Fatalf("Content-Type = %q; want application/json", ct)
			}
			var got fmtResponse
			if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
				t.Fatal(err)
			}
			if got.Body != tt.want {
				t.Errorf("wrong output\n got: %q\nwant: %q\n", got.Body, tt.want)
			}
			if got.Error != tt.wantErr {
				t.Errorf("wrong error\n got err: %q\nwant err: %q\n", got.Error, tt.wantErr)
			}
		})
	}
}
