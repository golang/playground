// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/binary"
	"reflect"
	"testing"
	"time"
)

func TestDecode(t *testing.T) {
	r := new(Recorder)
	stdout := r.Stdout()
	stderr := r.Stderr()

	stdout.Write([]byte("head"))
	stdout.Write(pbWrite(0, "one"))
	stdout.Write(pbWrite(0, "two"))

	stderr.Write(pbWrite(1*time.Second, "three"))
	stderr.Write(pbWrite(2*time.Second, "five"))
	stdout.Write(pbWrite(2*time.Second-time.Nanosecond, "four"))
	stderr.Write(pbWrite(2*time.Second, "six"))

	stdout.Write([]byte("middle"))
	stdout.Write(pbWrite(3*time.Second, "seven"))
	stdout.Write([]byte("tail"))

	want := []Event{
		{"headonetwo", "stdout", 0},
		{"three", "stderr", time.Second},
		{"fourmiddle", "stdout", time.Second - time.Nanosecond},
		{"fivesix", "stderr", time.Nanosecond},
		{"seventail", "stdout", time.Second},
	}

	got, err := r.Events()
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got: \n%v,\nwant \n%v", got, want)
	}
}

func pbWrite(offset time.Duration, s string) []byte {
	out := make([]byte, 16)
	out[2] = 'P'
	out[3] = 'B'
	binary.BigEndian.PutUint64(out[4:], uint64(epoch.Add(offset).UnixNano()))
	binary.BigEndian.PutUint32(out[12:], uint32(len(s)))
	return append(out, s...)
}
