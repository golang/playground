// Copyright 2019 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// latestgo prints the latest Go release tag to stdout as a part of the playground deployment process.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"golang.org/x/build/gerrit"
	"golang.org/x/build/maintner/maintnerd/maintapi/version"
)

var (
	prev      = flag.Bool("prev", false, "if set, query the previous Go release rather than the last (e.g. 1.17 versus 1.18)")
	toolchain = flag.Bool("toolchain", false, "if set, query released toolchains, rather than gerrit tags; toolchains may lag behind gerrit")
)

func main() {
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var latest []string
	if *toolchain {
		latest = latestToolchainVersions(ctx)
	} else {
		client := gerrit.NewClient("https://go-review.googlesource.com", gerrit.NoAuth)
		latest = latestGerritVersions(ctx, client)
	}
	if len(latest) < 2 {
		log.Fatalf("found %d versions, need at least 2", len(latest))
	}

	if *prev {
		fmt.Println(latest[1])
	} else {
		fmt.Println(latest[0])
	}
}

// latestGerritVersions queries the latest versions for each major Go release,
// among Gerrit tags.
func latestGerritVersions(ctx context.Context, client *gerrit.Client) []string {
	tagInfo, err := client.GetProjectTags(ctx, "go")
	if err != nil {
		log.Fatalf("error retrieving project tags for 'go': %v", err)
	}

	if len(tagInfo) == 0 {
		log.Fatalln("no project tags found for 'go'")
	}

	var tags []string
	for _, tag := range tagInfo {
		tags = append(tags, strings.TrimPrefix(tag.Ref, "refs/tags/"))
	}
	return latestPatches(tags)
}

// latestToolchainVersions queries the latest versions for each major Go
// release, among published toolchains. It may have fewer versions than
// [latestGerritVersions], because not all toolchains may be published.
func latestToolchainVersions(ctx context.Context) []string {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://go.dev/dl/?mode=json", nil)
	if err != nil {
		log.Fatalf("NewRequest: %v", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("fetching toolchains: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		log.Fatalf("fetching toolchains: got status %d, want 200", res.StatusCode)
	}
	data, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("reading body: %v", err)
	}

	type release struct {
		Version string `json:"version"`
	}
	var releases []release
	if err := json.Unmarshal(data, &releases); err != nil {
		log.Fatalf("unmarshaling releases JSON: %v", err)
	}
	var all []string
	for _, rel := range releases {
		all = append(all, rel.Version)
	}
	return latestPatches(all)
}

// latestPatches returns the latest minor release of each major Go version,
// among the set of tag or tag-like strings. The result is in descending
// order, such that later versions are sorted first.
//
// Tags that aren't of the form goX, goX.Y, or goX.Y.Z are ignored.
func latestPatches(tags []string) []string {
	// Find the latest patch version for each major Go version.
	type majMin struct {
		maj, min int // maj, min in semver terminology, which corresponds to a major go release
	}
	type patchTag struct {
		patch int
		tag   string // Go repo tag for this version
	}
	latestPatches := make(map[majMin]patchTag) // (maj, min) -> latest patch info

	for _, tag := range tags {
		maj, min, patch, ok := version.ParseTag(tag)
		if !ok {
			continue
		}
		mm := majMin{maj, min}
		if latest, ok := latestPatches[mm]; !ok || latest.patch < patch {
			latestPatches[mm] = patchTag{patch, tag}
		}
	}

	var mms []majMin
	for mm := range latestPatches {
		mms = append(mms, mm)
	}
	// Sort by descending semantic ordering, so that later versions are first.
	sort.Slice(mms, func(i, j int) bool {
		if mms[i].maj != mms[j].maj {
			return mms[i].maj > mms[j].maj
		}
		return mms[i].min > mms[j].min
	})

	var latest []string
	for _, mm := range mms {
		latest = append(latest, latestPatches[mm].tag)
	}
	return latest
}
