// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// vetCheck runs the "vet" tool on the source code in req.Body.
// In case of no errors it returns an empty, non-nil *response.
// Otherwise &response.Errors contains found errors.
func vetCheck(req *request) (*response, error) {
	tmpDir, err := ioutil.TempDir("", "vet")
	if err != nil {
		return nil, fmt.Errorf("error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	in := filepath.Join(tmpDir, "main.go")
	if err := ioutil.WriteFile(in, []byte(req.Body), 0400); err != nil {
		return nil, fmt.Errorf("error creating temp file %q: %v", in, err)
	}

	cmd := exec.Command("go", "vet", in)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return &response{}, nil
	}

	if _, ok := err.(*exec.ExitError); !ok {
		return nil, fmt.Errorf("error vetting go source: %v", err)
	}

	// Rewrite compiler errors to refer to progName
	// instead of '/tmp/sandbox1234/main.go'.
	errs := strings.Replace(string(out), in, progName, -1)

	// "go vet", invoked with a file name, puts this odd
	// message before any compile errors; strip it.
	errs = strings.Replace(errs, "# command-line-arguments\n", "", 1)

	return &response{Errors: errs}, nil
}
