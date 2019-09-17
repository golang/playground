// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The sandboxtypes package contains the shared types
// to communicate between the different sandbox components.
package sandboxtypes

// Response is the response from the x/playground/sandbox backend to
// the x/playground frontend.
//
// The stdout/stderr are base64 encoded which isn't ideal but is good
// enough for now. Maybe we'll move to protobufs later.
type Response struct {
	// Error, if non-empty, means we failed to run the binary.
	// It's meant to be user-visible.
	Error string `json:"error,omitempty"`

	ExitCode int    `json:"exitCode"`
	Stdout   []byte `json:"stdout"`
	Stderr   []byte `json:"stderr"`
}
