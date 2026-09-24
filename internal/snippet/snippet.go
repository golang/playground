// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package snippet defines the IDs of shared playground snippets.
package snippet

import (
	"crypto/sha256"
	"encoding/base64"
	"io"
)

// This salt is not meant to be kept secret (it’s checked in after all). It’s
// a tiny bit of paranoia to avoid whatever problems a collision may cause.
const salt = "Go playground salt\n"

// ID returns the ID of the shared snippet with the given body.
func ID(body []byte) string {
	h := sha256.New()
	io.WriteString(h, salt)
	h.Write(body)
	sum := h.Sum(nil)
	b := make([]byte, base64.URLEncoding.EncodedLen(len(sum)))
	base64.URLEncoding.Encode(b, sum)
	// Web sites don’t always linkify a trailing underscore, making it seem like
	// the link is broken. If there is an underscore at the end of the substring,
	// extend it until there is not.
	hashLen := 11
	for hashLen <= len(b) && b[hashLen-1] == '_' {
		hashLen++
	}
	return string(b)[:hashLen]
}
