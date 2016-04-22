// Copyright 2016 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goplay

import (
	"net/http"

	"appengine"
)

// httpsOnlyHandler redirects requests to "http://example.com/foo?bar"
// to "https://example.com/foo?bar"
type httpsOnlyHandler struct {
	fn http.HandlerFunc
}

func (h httpsOnlyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !appengine.IsDevAppServer() && r.TLS == nil {
		r.URL.Scheme = "https"
		r.URL.Host = r.Host
		http.Redirect(w, r, r.URL.String(), http.StatusFound)
		return
	}
	h.fn(w, r)
}
