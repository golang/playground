// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// latestgo prints the latest Go release tag to stdout as a part of the playground deployment process.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"golang.org/x/build/gerrit"
	"golang.org/x/build/maintner/maintnerd/maintapi/version"
)

var prev = flag.Bool("prev", false, "whether to query the previous Go release, rather than the last (e.g. 1.17 versus 1.18)")

func main() {
	client := gerrit.NewClient("https://go-review.googlesource.com", gerrit.NoAuth)

	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	tagInfo, err := client.GetProjectTags(ctx, "go")
	if err != nil {
		log.Fatalf("error retrieving project tags for 'go': %v", err)
	}

	if len(tagInfo) == 0 {
		log.Fatalln("no project tags found for 'go'")
	}

	// Find the latest patch version for each major Go version.
	type majMin struct {
		maj, min int // maj, min in semver terminology, which corresponds to a major go release
	}
	type patchTag struct {
		patch int
		tag   string // Go repo tag for this version
	}
	latestPatches := make(map[majMin]patchTag) // (maj, min) -> latest patch info

	for _, tag := range tagInfo {
		tagName := strings.TrimPrefix(tag.Ref, "refs/tags/")
		maj, min, patch, ok := version.ParseTag(tagName)
		if !ok {
			continue
		}

		mm := majMin{maj, min}
		if latest, ok := latestPatches[mm]; !ok || latest.patch < patch {
			latestPatches[mm] = patchTag{patch, tagName}
		}
	}

	var mms []majMin
	for mm := range latestPatches {
		mms = append(mms, mm)
	}
	sort.Slice(mms, func(i, j int) bool {
		if mms[i].maj != mms[j].maj {
			return mms[i].maj < mms[j].maj
		}
		return mms[i].min < mms[j].min
	})

	var mm majMin
	if *prev && len(mms) > 1 {
		mm = mms[len(mms)-2] // latest patch of the previous Go release
	} else {
		mm = mms[len(mms)-1]
	}
	fmt.Print(latestPatches[mm].tag)
}
