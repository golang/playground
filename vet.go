// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/tag"
)

// vetCheck runs the "vet" tool on the source code in req.Body.
// In case of no errors it returns an empty, non-nil *response.
// Otherwise &response.Errors contains found errors.
//
// Deprecated: this is the handler for the legacy /vet endpoint; use
// the /compile (compileAndRun) handler instead with the WithVet
// boolean set. This code path doesn't support modules and only exists
// as a temporary compatibility bridge to older javascript clients.
func vetCheck(ctx context.Context, req *request) (*response, error) {
	tmpDir, err := os.MkdirTemp("", "vet")
	if err != nil {
		return nil, fmt.Errorf("error creating temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	in := filepath.Join(tmpDir, progName)
	if err := os.WriteFile(in, []byte(req.Body), 0400); err != nil {
		return nil, fmt.Errorf("error creating temp file %q: %v", in, err)
	}
	vetOutput, err := vetCheckInDir(ctx, tmpDir, os.Getenv("GOPATH"), nil)
	if err != nil {
		// This is about errors running vet, not vet returning output.
		return nil, err
	}
	return &response{Errors: vetOutput}, nil
}

// vetCheckInDir runs go vet in the provided directory, using the
// provided GOPATH value. The returned error is only about whether
// go vet was able to run, not whether vet reported problem. The
// returned value is ("", nil) if vet successfully found nothing,
// and (non-empty, nil) if vet ran and found issues.
func vetCheckInDir(ctx context.Context, dir, goPath string, experiments []string) (output string, execErr error) {
	start := time.Now()
	defer func() {
		status := "success"
		if execErr != nil {
			status = "error"
		}
		// Ignore error. The only error can be invalid tag key or value
		// length, which we know are safe.
		stats.RecordWithTags(ctx, []tag.Mutator{tag.Upsert(kGoVetSuccess, status)},
			mGoVetLatency.M(float64(time.Since(start))/float64(time.Millisecond)))
	}()

	cmd := exec.Command("go", "vet", "--tags=faketime", "--mod=mod")
	cmd.Dir = dir
	// Linux go binary is not built with CGO_ENABLED=0.
	// Prevent vet to compile packages in cgo mode.
	// See #26307.
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOPATH="+goPath)
	cmd.Env = append(cmd.Env,
		"GO111MODULE=on",
		"GOPROXY="+playgroundGoproxy(),
	)
	if len(experiments) > 0 {
		cmd.Env = append(cmd.Env, "GOEXPERIMENT="+strings.Join(experiments, ","))
	}
	out, err := cmd.CombinedOutput()
	if err == nil {
		return "", nil
	}
	if _, ok := err.(*exec.ExitError); !ok {
		return "", fmt.Errorf("error vetting go source: %v", err)
	}

	// Rewrite compiler errors to refer to progName
	// instead of '/tmp/sandbox1234/main.go'.
	errs := strings.Replace(string(out), dir, "", -1)
	errs = removeBanner(errs)
	return errs, nil
}
