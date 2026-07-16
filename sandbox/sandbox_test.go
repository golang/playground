// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.opencensus.io/stats/view"
)

func TestLimitedWriter(t *testing.T) {
	cases := []struct {
		desc          string
		lw            *limitedWriter
		in            []byte
		want          []byte
		wantN         int64
		wantRemaining int64
		err           error
	}{
		{
			desc:          "simple",
			lw:            &limitedWriter{dst: &bytes.Buffer{}, n: 10},
			in:            []byte("hi"),
			want:          []byte("hi"),
			wantN:         2,
			wantRemaining: 8,
		},
		{
			desc:          "writing nothing",
			lw:            &limitedWriter{dst: &bytes.Buffer{}, n: 10},
			in:            []byte(""),
			want:          []byte(""),
			wantN:         0,
			wantRemaining: 10,
		},
		{
			desc:          "writing exactly enough",
			lw:            &limitedWriter{dst: &bytes.Buffer{}, n: 6},
			in:            []byte("enough"),
			want:          []byte("enough"),
			wantN:         6,
			wantRemaining: 0,
			err:           nil,
		},
		{
			desc:          "writing too much",
			lw:            &limitedWriter{dst: &bytes.Buffer{}, n: 10},
			in:            []byte("this is much longer than 10"),
			want:          []byte("this is mu"),
			wantN:         10,
			wantRemaining: -1,
			err:           errTooMuchOutput,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			n, err := io.Copy(c.lw, iotest.OneByteReader(bytes.NewReader(c.in)))
			if err != c.err {
				t.Errorf("c.lw.Write(%q) = %d, %q, wanted %d, %q", c.in, n, err, c.wantN, c.err)
			}
			if n != c.wantN {
				t.Errorf("c.lw.Write(%q) = %d, %q, wanted %d, %q", c.in, n, err, c.wantN, c.err)
			}
			if c.lw.n != c.wantRemaining {
				t.Errorf("c.lw.n = %d, wanted %d", c.lw.n, c.wantRemaining)
			}
			if string(c.lw.dst.Bytes()) != string(c.want) {
				t.Errorf("c.lw.dst.Bytes() = %q, wanted %q", c.lw.dst.Bytes(), c.want)
			}
		})
	}
}

func TestSwitchWriter(t *testing.T) {
	cases := []struct {
		desc      string
		sw        *switchWriter
		in        []byte
		want1     []byte
		want2     []byte
		wantN     int64
		wantFound bool
		err       error
	}{
		{
			desc:      "not found",
			sw:        &switchWriter{switchAfter: []byte("UNIQUE")},
			in:        []byte("hi"),
			want1:     []byte("hi"),
			want2:     []byte(""),
			wantN:     2,
			wantFound: false,
		},
		{
			desc:      "writing nothing",
			sw:        &switchWriter{switchAfter: []byte("UNIQUE")},
			in:        []byte(""),
			want1:     []byte(""),
			want2:     []byte(""),
			wantN:     0,
			wantFound: false,
		},
		{
			desc:      "writing exactly switchAfter",
			sw:        &switchWriter{switchAfter: []byte("UNIQUE")},
			in:        []byte("UNIQUE"),
			want1:     []byte("UNIQUE"),
			want2:     []byte(""),
			wantN:     6,
			wantFound: true,
		},
		{
			desc:      "writing before and after switchAfter",
			sw:        &switchWriter{switchAfter: []byte("UNIQUE")},
			in:        []byte("this is before UNIQUE and this is after"),
			want1:     []byte("this is before UNIQUE"),
			want2:     []byte(" and this is after"),
			wantN:     39,
			wantFound: true,
		},
	}
	for _, c := range cases {
		t.Run(c.desc, func(t *testing.T) {
			dst1, dst2 := &bytes.Buffer{}, &bytes.Buffer{}
			c.sw.dst1, c.sw.dst2 = dst1, dst2
			n, err := io.Copy(c.sw, iotest.OneByteReader(bytes.NewReader(c.in)))
			if err != c.err {
				t.Errorf("c.sw.Write(%q) = %d, %q, wanted %d, %q", c.in, n, err, c.wantN, c.err)
			}
			if n != c.wantN {
				t.Errorf("c.sw.Write(%q) = %d, %q, wanted %d, %q", c.in, n, err, c.wantN, c.err)
			}
			if c.sw.found != c.wantFound {
				t.Errorf("c.sw.found = %v, wanted %v", c.sw.found, c.wantFound)
			}
			if string(dst1.Bytes()) != string(c.want1) {
				t.Errorf("dst1.Bytes() = %q, wanted %q", dst1.Bytes(), c.want1)
			}
			if string(dst2.Bytes()) != string(c.want2) {
				t.Errorf("dst2.Bytes() = %q, wanted %q", dst2.Bytes(), c.want2)
			}
		})
	}
}

func TestSwitchWriterMultipleWrites(t *testing.T) {
	dst1, dst2 := &bytes.Buffer{}, &bytes.Buffer{}
	sw := &switchWriter{
		dst1:        dst1,
		dst2:        dst2,
		switchAfter: []byte("GOPHER"),
	}
	n, err := io.Copy(sw, iotest.OneByteReader(strings.NewReader("this is before GO")))
	if err != nil || n != 17 {
		t.Errorf("sw.Write(%q) = %d, %q, wanted %d, no error", "this is before GO", n, err, 17)
	}
	if sw.found {
		t.Errorf("sw.found = %v, wanted %v", sw.found, false)
	}
	if string(dst1.Bytes()) != "this is before GO" {
		t.Errorf("dst1.Bytes() = %q, wanted %q", dst1.Bytes(), "this is before GO")
	}
	if string(dst2.Bytes()) != "" {
		t.Errorf("dst2.Bytes() = %q, wanted %q", dst2.Bytes(), "")
	}
	n, err = io.Copy(sw, iotest.OneByteReader(strings.NewReader("PHER and this is after")))
	if err != nil || n != 22 {
		t.Errorf("sw.Write(%q) = %d, %q, wanted %d, no error", "this is before GO", n, err, 22)
	}
	if !sw.found {
		t.Errorf("sw.found = %v, wanted %v", sw.found, true)
	}
	if string(dst1.Bytes()) != "this is before GOPHER" {
		t.Errorf("dst1.Bytes() = %q, wanted %q", dst1.Bytes(), "this is before GOPHEr")
	}
	if string(dst2.Bytes()) != " and this is after" {
		t.Errorf("dst2.Bytes() = %q, wanted %q", dst2.Bytes(), " and this is after")
	}
}

func TestParseDockerContainers(t *testing.T) {
	cases := []struct {
		desc    string
		output  string
		want    []dockerContainer
		wantErr bool
	}{
		{
			desc: "normal output (container per line)",
			output: `{"Command":"\"/usr/local/bin/play…\"","CreatedAt":"2020-04-23 17:44:02 -0400 EDT","ID":"f7f170fde076","Image":"gcr.io/golang-org/playground-sandbox-gvisor:latest","Labels":"","LocalVolumes":"0","Mounts":"","Names":"play_run_a02cfe67","Networks":"none","Ports":"","RunningFor":"8 seconds ago","Size":"0B","Status":"Up 7 seconds"}
{"Command":"\"/usr/local/bin/play…\"","CreatedAt":"2020-04-23 17:44:02 -0400 EDT","ID":"af872e55a773","Image":"gcr.io/golang-org/playground-sandbox-gvisor:latest","Labels":"","LocalVolumes":"0","Mounts":"","Names":"play_run_0a69c3e8","Networks":"none","Ports":"","RunningFor":"8 seconds ago","Size":"0B","Status":"Up 7 seconds"}`,
			want: []dockerContainer{
				{ID: "f7f170fde076", Image: "gcr.io/golang-org/playground-sandbox-gvisor:latest", Names: "play_run_a02cfe67"},
				{ID: "af872e55a773", Image: "gcr.io/golang-org/playground-sandbox-gvisor:latest", Names: "play_run_0a69c3e8"},
			},
			wantErr: false,
		},
		{
			desc:    "empty output",
			wantErr: false,
		},
		{
			desc:    "malformatted output",
			output:  `xyzzy{}`,
			wantErr: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			cs, err := parseDockerContainers([]byte(tc.output))
			if (err != nil) != tc.wantErr {
				t.Errorf("parseDockerContainers(_) = %v, %v, wantErr: %v", cs, err, tc.wantErr)
			}
			if diff := cmp.Diff(tc.want, cs); diff != "" {
				t.Errorf("parseDockerContainers() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestStartContainer_TimeoutNoHang(t *testing.T) {
	oldExecCommand := execCommand
	oldStartTimeout := startTimeout
	defer func() {
		execCommand = oldExecCommand
		startTimeout = oldStartTimeout
	}()

	// Set a short timeout for the test to run quickly
	startTimeout = 100 * time.Millisecond

	// Mock execCommand to run the test binary itself, behaving as a sleep command.
	// This avoids depending on external "sleep" binary which is not available on Windows.
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cmd := exec.Command(os.Args[0], "-test.run=TestHelperProcess")
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	// Call startContainer. It should timeout and return an error.
	// If the hang bug is present, this call will block forever.
	c, err := startContainer(t.Context())
	if err == nil {
		t.Fatalf("startContainer succeeded unexpectedly; want timeout error")
		c.Close()
	}
	if !strings.Contains(err.Error(), "timeout starting container") {
		t.Errorf("startContainer error = %v; want timeout error", err)
	}
}

// TestHelperProcess is a helper process used to simulate long-running/stuck commands.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	var buf [1]byte
	os.Stdin.Read(buf[:])
	os.Exit(0)
}

func TestGetContainerMetrics(t *testing.T) {
	// Initialize readyContainer channel if not already done (it is usually done in main)
	if readyContainer == nil {
		readyContainer = make(chan *Container)
	}

	// Register views
	if err := view.Register(views...); err != nil {
		if !strings.Contains(err.Error(), "already registered") {
			t.Fatalf("view.Register: %v", err)
		}
	}

	// Test case 1: Canceled context
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	_, err := getContainer(ctx)
	if err == nil {
		t.Fatal("expected error from getContainer with canceled context")
	}

	rows, err := view.RetrieveData("go-playground/sandbox/container_wait_count")
	if err != nil {
		t.Fatalf("RetrieveData failed: %v", err)
	}

	found := false
	for _, row := range rows {
		for _, tg := range row.Tags {
			if tg.Key == kContainerWaitStatus && tg.Value == "canceled" {
				found = true
				countData, ok := row.Data.(*view.CountData)
				if !ok {
					t.Fatalf("unexpected data type: %T", row.Data)
				}
				if countData.Value < 1 {
					t.Errorf("expected count >= 1, got %v", countData.Value)
				}
			}
		}
	}
	if !found {
		t.Errorf("metric container_wait_count with tag status=canceled was not recorded. Rows: %v", rows)
	}

	// Test case 2: Success (container available)
	c := &Container{
		name:      "test-container",
		cancelCmd: func() {},
	}
	ctx2 := context.Background()
	go func() {
		readyContainer <- c
	}()

	gotC, err := getContainer(ctx2)
	if err != nil {
		t.Fatalf("getContainer failed: %v", err)
	}
	if gotC != c {
		t.Errorf("got container %v, want %v", gotC, c)
	}

	rows, err = view.RetrieveData("go-playground/sandbox/container_wait_count")
	if err != nil {
		t.Fatalf("RetrieveData failed: %v", err)
	}

	found = false
	for _, row := range rows {
		for _, tg := range row.Tags {
			if tg.Key == kContainerWaitStatus && tg.Value == "success" {
				found = true
				countData, ok := row.Data.(*view.CountData)
				if !ok {
					t.Fatalf("unexpected data type: %T", row.Data)
				}
				if countData.Value < 1 {
					t.Errorf("expected count >= 1, got %v", countData.Value)
				}
			}
		}
	}
	if !found {
		t.Errorf("metric container_wait_count with tag status=success was not recorded. Rows: %v", rows)
	}
}
