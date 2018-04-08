// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
	"unicode/utf8"
)

// When sandbox time begins.
var epoch = time.Unix(1257894000, 0)

// Recorder records the standard and error outputs of a sandbox program
// (comprised of playback headers) and converts it to a sequence of Events.
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
type Recorder struct {
	stdout, stderr recorderWriter
}

func (r *Recorder) Stdout() io.Writer { return &r.stdout }
func (r *Recorder) Stderr() io.Writer { return &r.stderr }

type recorderWriter struct {
	mu     sync.Mutex
	writes []byte
}

func (w *recorderWriter) bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.writes[0:len(w.writes):len(w.writes)]
}

func (w *recorderWriter) Write(b []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.writes = append(w.writes, b...)
	return len(b), nil
}

type Event struct {
	Message string
	Kind    string        // "stdout" or "stderr"
	Delay   time.Duration // time to wait before printing Message
}

func (r *Recorder) Events() ([]Event, error) {
	stdout, stderr := r.stdout.bytes(), r.stderr.bytes()

	evOut, err := decode("stdout", stdout)
	if err != nil {
		return nil, err
	}
	evErr, err := decode("stderr", stderr)
	if err != nil {
		return nil, err
	}

	events := sortedMerge(evOut, evErr)

	var (
		out []Event
		now = epoch
	)

	for _, e := range events {
		delay := e.time.Sub(now)
		if delay < 0 {
			delay = 0
		}
		out = append(out, Event{
			Message: string(sanitize(e.msg)),
			Kind:    e.kind,
			Delay:   delay,
		})
		if delay > 0 {
			now = e.time
		}
	}
	return out, nil
}

type event struct {
	msg  []byte
	kind string
	time time.Time
}

func decode(kind string, output []byte) ([]event, error) {
	var (
		magic     = []byte{0, 0, 'P', 'B'}
		headerLen = 8 + 4
		last      = epoch
		events    []event
	)
	add := func(t time.Time, b []byte) {
		var prev *event
		if len(events) > 0 {
			prev = &events[len(events)-1]
		}
		if prev != nil && t.Equal(prev.time) {
			// Merge this event with previous event, to avoid
			// sending a lot of events for a big output with no
			// significant timing information.
			prev.msg = append(prev.msg, b...)
		} else {
			e := event{msg: b, kind: kind, time: t}
			events = append(events, e)
		}
		last = t
	}
	for i := 0; i < len(output); {
		if !bytes.HasPrefix(output[i:], magic) {
			// Not a header; find next header.
			j := bytes.Index(output[i:], magic)
			if j < 0 {
				// No more headers; bail.
				add(last, output[i:])
				break
			}
			add(last, output[i:i+j])
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
		if t.Before(last) {
			// Force timestamps to be monotonic. (This could
			// be an encoding error, which we ignore now but will
			// will likely be picked up when decoding the length.)
			t = last
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
	return events, nil
}

// Sorted merge of two slices of events into one slice.
func sortedMerge(a, b []event) []event {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}

	sorted := make([]event, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i].time.Before(b[j].time) {
			sorted = append(sorted, a[i])
			i++
		} else {
			sorted = append(sorted, b[j])
			j++
		}
	}
	sorted = append(sorted, a[i:]...)
	sorted = append(sorted, b[j:]...)
	return sorted
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
