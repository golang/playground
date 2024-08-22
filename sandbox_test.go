// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// TestExperiments tests that experiment lines are recognized.
func TestExperiments(t *testing.T) {
	var tests = []struct {
		src string
		exp []string
	}{
		{"//GOEXPERIMENT=active\n\npackage main", []string{"active"}},
		{"   //   GOEXPERIMENT=   active   \n\npackage main", []string{"active"}},
		{"   //   GOEXPERIMENT=   active   \n\npackage main", []string{"active"}},
		{"   //   GOEXPERIMENT   =   active   \n\npackage main", []string{"active"}},
		{"//GOEXPERIMENT=foo\n\n// GOEXPERIMENT=bar\n\npackage main", []string{"foo", "bar"}},
		{"/* hello world */\n// GOEXPERIMENT=ignored\n", nil},
		{"package main\n// GOEXPERIMENT=ignored\n", nil},
	}

	for _, tt := range tests {
		if exp := experiments(tt.src); !reflect.DeepEqual(exp, tt.exp) {
			t.Errorf("experiments(%q) = %q, want %q", tt.src, exp, tt.exp)
		}
	}
}

// TestIsTest verifies that the isTest helper function matches
// exactly (and only) the names of functions recognized as tests.
func TestIsTest(t *testing.T) {
	// We must disable vet's "tests" analyzer which would otherwise cause
	// go test to fail due to the intentional problems in testdata/p's tests.
	cmd := exec.Command("go", "test", "./testdata/p", "-vet=off", "-test.list=.")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", strings.Join(cmd.Args, " "), err, out)
	}
	t.Logf("%s:\n%s", strings.Join(cmd.Args, " "), out)

	isTestFunction := map[string]bool{}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		// We want Test/Benchmark/Example/Fuzz functions.
		// Reject extraneous output such as "ok ...".
		if line == "" || !strings.Contains("TBEF", line[:1]) {
			continue
		}
		isTestFunction[strings.TrimSpace(line)] = true
	}

	for _, tc := range []struct {
		prefix string
		name   string // name of a Test (etc) in ./testdata/p
		want   bool
	}{
		{"Test", "Test", true},
		{"Test", "Test1IsATest", true},
		{"Test", "TestÑIsATest", true},

		{"Test", "TestisNotATest", false},

		{"Example", "Example", true},
		{"Example", "ExampleTest", true},
		{"Example", "Example_isAnExample", true},
		{"Example", "ExampleTest_isAnExample", true},

		// Example_noOutput has a valid example function name but lacks an output
		// declaration, but the isTest function operates only on the test name
		// so it cannot detect that the function is not a test.

		{"Example", "Example1IsAnExample", true},
		{"Example", "ExampleisNotAnExample", false},

		{"Benchmark", "Benchmark", true},
		{"Benchmark", "BenchmarkNop", true},
		{"Benchmark", "Benchmark1IsABenchmark", true},

		{"Benchmark", "BenchmarkisNotABenchmark", false},

		{"Fuzz", "Fuzz", true},
		{"Fuzz", "Fuzz1IsAFuzz", true},
		{"Fuzz", "FuzzÑIsAFuzz", true},

		{"Fuzz", "FuzzisNotAFuzz", false},
	} {
		name := tc.name
		t.Run(name, func(t *testing.T) {
			if tc.want != isTestFunction[name] {
				t.Fatalf(".want (%v) is inconsistent with -test.list", tc.want)
			}
			if !strings.HasPrefix(name, tc.prefix) {
				t.Fatalf("%q is not a prefix of %v", tc.prefix, name)
			}

			got := isTest(name, tc.prefix)
			if got != tc.want {
				t.Errorf(`isTest(%q, %q) = %v; want %v`, name, tc.prefix, got, tc.want)
			}
		})
	}
}
