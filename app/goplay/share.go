// Copyright 2011 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package goplay

import (
	"appengine"
	"appengine/datastore"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
)

const salt = "[replace this with something unique]"

type Snippet struct {
	Body []byte
}

func (s *Snippet) Id() string {
	h := sha1.New()
	io.WriteString(h, salt)
	h.Write(s.Body)
	sum := h.Sum(nil)
	b := make([]byte, base64.URLEncoding.EncodedLen(len(sum)))
	base64.URLEncoding.Encode(b, sum)
	return string(b)[:10]
}

func init() {
	http.HandleFunc("/share", share)
}

func share(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}
	c := appengine.NewContext(r)

	var body bytes.Buffer
	_, err := body.ReadFrom(r.Body)
	if err != nil {
		c.Errorf("reading Body: %v", err)
		http.Error(w, "Server Error", http.StatusInternalServerError)
		return
	}
	r.Body.Close()

	snip := &Snippet{Body: body.Bytes()}
	id := snip.Id()
	key := datastore.NewKey(c, "Snippet", id, 0, nil)
	_, err = datastore.Put(c, key, snip)
	if err != nil {
		c.Errorf("putting Snippet: %v", err)
		http.Error(w, "Server Error", http.StatusInternalServerError)
		return
	}

	fmt.Fprint(w, id)
}
