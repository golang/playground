// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func modTidy(ctx context.Context, dir, goPath string) (output string, execErr error) {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOPATH="+goPath)
	cmd.Env = append(cmd.Env,
		"GO111MODULE=on",
		"GOPROXY="+playgroundGoproxy(),
	)
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

	return errs, nil
}
