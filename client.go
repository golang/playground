//go:build ignore
// +build ignore

// Copyright 2014 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"io"
	"log"
	"os"
)

func main() {
	body, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("error reading stdin: %v", err)
	}
	json.NewEncoder(os.Stdout).Encode(struct {
		Body string
	}{
		Body: string(body),
	})
}
