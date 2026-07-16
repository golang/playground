// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"go.opencensus.io/stats/view"
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

func TestSandboxRunMetrics(t *testing.T) {
	// Mock backend
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"Success": true, "Events": []}`))
	}))
	defer server.Close()

	t.Setenv("SANDBOX_BACKEND_URL", server.URL)

	// Register views
	if err := view.Register(views...); err != nil {
		// Ignore error if already registered, but fail on other errors
		if !strings.Contains(err.Error(), "already registered") {
			t.Fatalf("view.Register: %v", err)
		}
	}

	tmpFile, err := os.CreateTemp("", "test-exe")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write([]byte("dummy exe content")); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	ctx := t.Context()
	if _, err = sandboxRun(ctx, tmpFile.Name(), ""); err != nil {
		t.Fatalf("sandboxRun failed: %v", err)
	}

	rows, err := view.RetrieveData("go-playground/frontend/go_run_count")
	if err != nil {
		t.Fatalf("RetrieveData failed: %v", err)
	}

	found := false
	for _, row := range rows {
		for _, tg := range row.Tags {
			if tg.Key == kGoRunSuccess && tg.Value == "success" {
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
		t.Errorf("metric go-playground/frontend/go_run_count with tag go_run_success=success was not recorded. Rows: %v", rows)
	}
}
