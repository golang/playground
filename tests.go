// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Test tests are linked into the main binary and are run as part of
// the Docker build step.

package main

import (
	"context"
	"fmt"
	stdlog "log"
	"net"
	"os"
	"reflect"
	"strings"
	"time"
)

type compileTest struct {
	name               string // test name
	prog, want, errors string
	wantFunc           func(got string) error // alternative to want
	withVet            bool
	wantEvents         []Event
	wantVetErrors      string
}

func (s *server) test() {
	if _, err := net.ResolveIPAddr("ip", "sandbox_dev.sandnet."); err != nil {
		log.Fatalf("sandbox_dev.sandnet not available")
	}
	os.Setenv("DEBUG_FORCE_GVISOR", "1")
	os.Setenv("SANDBOX_BACKEND_URL", "http://sandbox_dev.sandnet/run")
	s.runTests()
}

func (s *server) runTests() {
	if err := s.healthCheck(context.Background()); err != nil {
		stdlog.Fatal(err)
	}

	// Enable module downloads for testing:
	defer func(old string) { os.Setenv("ALLOW_PLAY_MODULE_DOWNLOADS", old) }(os.Getenv("ALLOW_PLAY_MODULE_DOWNLOADS"))
	os.Setenv("ALLOW_PLAY_MODULE_DOWNLOADS", "true")

	failed := false
	for i, t := range tests {
		stdlog.Printf("testing case %d (%q)...\n", i, t.name)
		resp, err := compileAndRun(context.Background(), &request{Body: t.prog, WithVet: t.withVet})
		if err != nil {
			stdlog.Fatal(err)
		}
		if t.wantEvents != nil {
			if !reflect.DeepEqual(resp.Events, t.wantEvents) {
				stdlog.Printf("resp.Events = %q, want %q", resp.Events, t.wantEvents)
				failed = true
			}
			continue
		}
		if t.errors != "" {
			if resp.Errors != t.errors {
				stdlog.Printf("resp.Errors = %q, want %q", resp.Errors, t.errors)
				failed = true
			}
			continue
		}
		if resp.Errors != "" {
			stdlog.Printf("resp.Errors = %q, want %q", resp.Errors, t.errors)
			failed = true
			continue
		}
		if resp.VetErrors != t.wantVetErrors {
			stdlog.Printf("resp.VetErrs = %q, want %q", resp.VetErrors, t.wantVetErrors)
			failed = true
			continue
		}
		if t.withVet && (resp.VetErrors != "") == resp.VetOK {
			stdlog.Printf("resp.VetErrs & VetOK inconsistent; VetErrs = %q; VetOK = %v", resp.VetErrors, resp.VetOK)
			failed = true
			continue
		}
		if len(resp.Events) == 0 {
			stdlog.Printf("unexpected output: %q, want %q", "", t.want)
			failed = true
			continue
		}
		var b strings.Builder
		for _, e := range resp.Events {
			b.WriteString(e.Message)
		}
		if t.wantFunc != nil {
			if err := t.wantFunc(b.String()); err != nil {
				stdlog.Printf("%v\n", err)
				failed = true
			}
		} else {
			if !strings.Contains(b.String(), t.want) {
				stdlog.Printf("unexpected output: %q, want %q", b.String(), t.want)
				failed = true
			}
		}
	}
	if failed {
		stdlog.Fatalf("FAILED")
	}
	fmt.Println("OK")
}

var tests = []compileTest{
	{
		name: "timezones_available",
		prog: `
package main

import "time"

func main() {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		panic(err.Error())
	}
	println(loc.String())
}
`, want: "America/New_York"},

	{
		name: "faketime_works",
		prog: `
package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println(time.Now())
}
`, want: "2009-11-10 23:00:00 +0000 UTC"},

	{
		name: "faketime_tickers",
		prog: `
package main

import (
	"fmt"
	"time"
)

func main() {
	t1 := time.Tick(time.Second * 3)
	t2 := time.Tick(time.Second * 7)
	t3 := time.Tick(time.Second * 11)
	end := time.After(time.Second * 19)
	want := "112131211"
	var got []byte
	for {
		var c byte
		select {
		case <-t1:
			c = '1'
		case <-t2:
			c = '2'
		case <-t3:
			c = '3'
		case <-end:
			if g := string(got); g != want {
				fmt.Printf("got %q, want %q\n", g, want)
			} else {
				fmt.Println("timers fired as expected")
			}
			return
		}
		got = append(got, c)
	}
}
`, want: "timers fired as expected"},
	{
		name: "must_be_package_main",
		prog: `
package test

func main() {
	println("test")
}
`, want: "", errors: "package name must be main"},
	{
		name: "filesystem_contents",
		prog: `
package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	filepath.Walk("/", func(path string, info os.FileInfo, err error) error {
		if path == "/proc" || path == "/sys" {
			return filepath.SkipDir
		}
		fmt.Println(path)
		return nil
	})
}
`, wantFunc: func(got string) error {
			// The environment for the old nacl sandbox:
			if strings.TrimSpace(got) == `/
/dev
/dev/null
/dev/random
/dev/urandom
/dev/zero
/etc
/etc/group
/etc/hosts
/etc/passwd
/etc/resolv.conf
/tmp
/usr
/usr/local
/usr/local/go
/usr/local/go/lib
/usr/local/go/lib/time
/usr/local/go/lib/time/zoneinfo.zip` {
				return nil
			}
			have := map[string]bool{}
			for _, f := range strings.Split(got, "\n") {
				have[f] = true
			}
			for _, expect := range []string{
				"/.dockerenv",
				"/etc/hostname",
				"/dev/zero",
				"/lib/ld-linux-x86-64.so.2",
				"/lib/libc.so.6",
				"/etc/nsswitch.conf",
				"/bin/env",
				"/tmpfs",
			} {
				if !have[expect] {
					return fmt.Errorf("missing expected sandbox file %q; got:\n%s", expect, got)
				}
			}
			return nil
		},
	},
	{
		name: "stdout_stderr_merge",
		prog: `
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stdout, "A")
	fmt.Fprintln(os.Stderr, "B")
	fmt.Fprintln(os.Stdout, "A")
	fmt.Fprintln(os.Stdout, "A")
}
`, want: "A\nB\nA\nA\n"},

	// Integration test for runtime.write fake timestamps.
	{
		name: "faketime_write_interaction",
		prog: `
package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	fmt.Fprintln(os.Stdout, "A")
	fmt.Fprintln(os.Stderr, "B")
	fmt.Fprintln(os.Stdout, "A")
	fmt.Fprintln(os.Stdout, "A")
	time.Sleep(time.Second)
	fmt.Fprintln(os.Stderr, "B")
	time.Sleep(time.Second)
	fmt.Fprintln(os.Stdout, "A")
}
`, wantEvents: []Event{
			{"A\n", "stdout", 0},
			{"B\n", "stderr", time.Nanosecond},
			{"A\nA\n", "stdout", time.Nanosecond},
			{"B\n", "stderr", time.Second - 2*time.Nanosecond},
			{"A\n", "stdout", time.Second},
		}},
	{
		name:          "compile_with_vet",
		withVet:       true,
		wantVetErrors: "./prog.go2:5: Printf format %v reads arg #1, but call has 0 args\n",
		prog: `
package main
import "fmt"
func main() {
	fmt.Printf("hi %v")
}
`,
	},

	{
		name:    "compile_without_vet",
		withVet: false,
		prog: `
package main
import "fmt"
func main() {
	fmt.Printf("hi %v")
}
`,
	},
	{
		name: "timeouts_handled_gracefully",
		prog: `
package main

import (
	"time"
)

func main() {
	c := make(chan struct{})

	go func() {
		defer close(c)
		for {
			time.Sleep(10 * time.Millisecond)
		}
	}()

	<-c
}
`, want: "timeout running program"},
	{
		name: "timezone_info_exists",
		prog: `
package main

import (
	"fmt"
	"time"
)

func main() {
	loc, _ := time.LoadLocation("Europe/Berlin")

	// This will look for the name CEST in the Europe/Berlin time zone.
	const longForm = "Jan 2, 2006 at 3:04pm (MST)"
	t, _ := time.ParseInLocation(longForm, "Jul 9, 2012 at 5:02am (CEST)", loc)
	fmt.Println(t)

	// Note: without explicit zone, returns time in given location.
	const shortForm = "2006-Jan-02"
	t, _ = time.ParseInLocation(shortForm, "2012-Jul-09", loc)
	fmt.Println(t)

}
`, want: "2012-07-09 05:02:00 +0200 CEST\n2012-07-09 00:00:00 +0200 CEST\n"},
}
