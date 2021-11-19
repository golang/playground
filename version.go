// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"go/build"
	"net/http"
	"runtime"
)

func (s *server) handleVersion(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")

	tag := build.Default.ReleaseTags[len(build.Default.ReleaseTags)-1]
	var maj, min int
	if _, err := fmt.Sscanf(tag, "go%d.%d", &maj, &min); err != nil {
		code := http.StatusInternalServerError
		http.Error(w, http.StatusText(code), code)
		return
	}

	version := struct {
		Version, Release, Name string
	}{
		Version: runtime.Version(),
		Release: tag,
	}

	if s.gotip {
		version.Name = "Go dev branch"
	} else {
		version.Name = fmt.Sprintf("Go %d.%d", maj, min)
	}

	s.writeJSONResponse(w, version, http.StatusOK)
}
