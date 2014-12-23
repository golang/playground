// Copyright 2014 The Go Authors.  All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
	"unicode/utf8"
)

type Event struct {
	Message string
	Delay   time.Duration // time to wait before printing Message
}

// Decode takes an output string comprised of playback headers, and converts
// it to a sequence of Events.
// It sanitizes each Event's Message to ensure it is valid UTF-8.
//
// Playground programs precede all their writes with a header (described
// below) that describes the time the write occurred (in playground time) and
// the length of the data that will be written. If a non-header is
// encountered where a header is expected, the output is scanned for the next
// header and the intervening text string is added to the sequence an event
// occurring at the same time as the preceding event.
//
// A playback header has this structure:
// 	4 bytes: "\x00\x00PB", a magic header
// 	8 bytes: big-endian int64, unix time in nanoseconds
// 	4 bytes: big-endian int32, length of the next write
//
func Decode(output []byte) (seq []Event, err error) {
	var (
		magic     = []byte{0, 0, 'P', 'B'}
		headerLen = 8 + 4
		now       = time.Unix(1257894000, 0) // go epoch
	)

	add := func(t time.Time, b []byte) {
		e := Event{
			Message: string(sanitize(b)),
			Delay:   t.Sub(now),
		}
		if e.Delay == 0 && len(seq) > 0 {
			// Merge this event with previous event, to avoid
			// sending a lot of events for a big output with no
			// significant timing information.
			seq[len(seq)-1].Message += e.Message
		} else {
			seq = append(seq, e)
		}
		now = t
	}

	for i := 0; i < len(output); {
		if !bytes.HasPrefix(output[i:], magic) {
			// Not a header; find next header.
			j := bytes.Index(output[i:], magic)
			if j < 0 {
				// No more headers; bail.
				add(now, output[i:])
				break
			}
			add(now, output[i:i+j])
			i += j
		}
		i += len(magic)

		// Decode header.
		if len(output)-i < headerLen {
			return nil, errors.New("short header")
		}
		header := output[i : i+headerLen]
		nanos := int64(binary.BigEndian.Uint64(header[0:]))
		t := time.Unix(0, nanos)
		if t.Before(now) {
			// Force timestamps to be monotonic. (This could
			// be an encoding error, which we ignore now but will
			// will likely be picked up when decoding the length.)
			t = now
		}
		n := int(binary.BigEndian.Uint32(header[8:]))
		if n < 0 {
			return nil, fmt.Errorf("bad length: %v", n)
		}
		i += headerLen

		// Slurp output.
		// Truncated output is OK (probably caused by sandbox limits).
		end := i + n
		if end > len(output) {
			end = len(output)
		}
		add(t, output[i:end])
		i += n
	}
	return
}

// sanitize scans b for invalid utf8 code points. If found, it reconstructs
// the slice replacing the invalid codes with \uFFFD, properly encoded.
func sanitize(b []byte) []byte {
	if utf8.Valid(b) {
		return b
	}
	var buf bytes.Buffer
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		b = b[size:]
		buf.WriteRune(r)
	}
	return buf.Bytes()
}
