package snippet

import (
	"crypto/sha256"
	"encoding/base64"
	"io"
)

const (
	// This salt is not meant to be kept secret (it’s checked in after all). It’s
	// a tiny bit of paranoia to avoid whatever problems a collision may cause.
	salt = "Go playground salt\n"
)

type Snippet struct {
	Body []byte `datastore:",noindex"` // golang.org/issues/23253
}

func (s *Snippet) ID() string {
	h := sha256.New()
	io.WriteString(h, salt)
	h.Write(s.Body)
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

func Decode(b []byte) *Snippet {
	return &Snippet{Body: b}
}

func Encode(s *Snippet) []byte {
	return s.Body
}
