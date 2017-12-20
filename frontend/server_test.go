// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
package main

import "testing"

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
}
