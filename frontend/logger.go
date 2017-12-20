// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	stdlog "log"
	"os"
)

type logger interface {
	Printf(format string, args ...interface{})
	Errorf(format string, args ...interface{})
	Fatalf(format string, args ...interface{})
}

// stdLogger implements the logger interface using the log package.
// There is no need to specify a date/time prefix since stdout and stderr
// are logged in StackDriver with those values already present.
type stdLogger struct {
	stderr *stdlog.Logger
	stdout *stdlog.Logger
}

func newStdLogger() *stdLogger {
	return &stdLogger{
		stdout: stdlog.New(os.Stdout, "", 0),
		stderr: stdlog.New(os.Stderr, "", 0),
	}
}

func (l *stdLogger) Printf(format string, args ...interface{}) {
	l.stdout.Printf(format, args...)
}

func (l *stdLogger) Errorf(format string, args ...interface{}) {
	l.stderr.Printf(format, args...)
}

func (l *stdLogger) Fatalf(format string, args ...interface{}) {
	l.stderr.Fatalf(format, args...)
}
