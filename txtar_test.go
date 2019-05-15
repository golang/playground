// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func newFileSet(kv ...string) *fileSet {
	fs := new(fileSet)
	if kv[0] == "prog.go!implicit" {
		fs.noHeader = true
		kv[0] = "prog.go"
	}
	for len(kv) > 0 {
		fs.AddFile(kv[0], []byte(kv[1]))
		kv = kv[2:]
	}
	return fs
}

func TestSplitFiles(t *testing.T) {
	for _, tt := range []struct {
		name    string
		in      string
		want    *fileSet
		wantErr string
	}{
		{
			name: "classic",
			in:   "package main",
			want: newFileSet("prog.go!implicit", "package main\n"),
		},
		{
			name: "implicit prog.go",
			in:   "package main\n-- two.go --\nsecond",
			want: newFileSet(
				"prog.go!implicit", "package main\n",
				"two.go", "second\n",
			),
		},
		{
			name: "basic txtar",
			in:   "-- main.go --\npackage main\n-- foo.go --\npackage main\n",
			want: newFileSet(
				"main.go", "package main\n",
				"foo.go", "package main\n",
			),
		},
		{
			name:    "reject dotdot 1",
			in:      "-- ../foo --\n",
			wantErr: `invalid file name "../foo"`,
		},
		{
			name:    "reject dotdot 2",
			in:      "-- .. --\n",
			wantErr: `invalid file name ".."`,
		},
		{
			name:    "reject dotdot 3",
			in:      "-- bar/../foo --\n",
			wantErr: `invalid file name "bar/../foo"`,
		},
		{
			name:    "reject long",
			in:      "-- xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx --\n",
			wantErr: `file name too long`,
		},
		{
			name:    "reject deep",
			in:      "-- x/x/x/x/x/x/x/x/x/x/x/x/x/x/x/x/x/x/x/x/x --\n",
			wantErr: `file name "x/x/x/x/x/x/x/x/x/x/x/x/x/x/x/x/x/x/x/x/x" too deep`,
		},
		{
			name:    "reject abs",
			in:      "-- /etc/passwd --\n",
			wantErr: `invalid file name "/etc/passwd"`,
		},
		{
			name:    "reject backslash",
			in:      "-- foo\\bar --\n",
			wantErr: `invalid file name "foo\\bar"`,
		},
		{
			name:    "reject binary null",
			in:      "-- foo\x00bar --\n",
			wantErr: `invalid file name "foo\x00bar"`,
		},
		{
			name:    "reject binary low",
			in:      "-- foo\x1fbar --\n",
			wantErr: `invalid file name "foo\x1fbar"`,
		},
		{
			name:    "reject dup",
			in:      "-- foo.go --\n-- foo.go --\n",
			wantErr: `duplicate file name "foo.go"`,
		},
		{
			name:    "reject implicit dup",
			in:      "package main\n-- prog.go --\n",
			wantErr: `duplicate file name "prog.go"`,
		},
		{
			name: "skip leading whitespace comment",
			in:   "\n    \n\n   \n\n-- f.go --\ncontents",
			want: newFileSet("f.go", "contents\n"),
		},
		{
			name:    "reject many files",
			in:      strings.Repeat("-- x.go --\n", 50),
			wantErr: `too many files in txtar archive (50 exceeds limit of 20)`,
		},
	} {
		got, err := splitFiles([]byte(tt.in))
		var gotErr string
		if err != nil {
			gotErr = err.Error()
		}
		if gotErr != tt.wantErr {
			if tt.wantErr == "" {
				t.Errorf("%s: unexpected error: %v", tt.name, err)
				continue
			}
			t.Errorf("%s: error = %#q; want error %#q", tt.name, err, tt.wantErr)
			continue
		}
		if err != nil {
			continue
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%s: wrong files\n got:\n%s\nwant:\n%s", tt.name, filesAsString(got), filesAsString(tt.want))
		}
	}
}

func filesAsString(fs *fileSet) string {
	var sb strings.Builder
	for i, f := range fs.files {
		var implicit string
		if i == 0 && f == progName && fs.noHeader {
			implicit = " (implicit)"
		}
		fmt.Fprintf(&sb, "[file %q%s]: %q\n", f, implicit, fs.Data(f))
	}
	return sb.String()
}
