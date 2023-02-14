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
	"strings"
	"time"

	"golang.org/x/build/gerrit"
	"golang.org/x/mod/semver"
)

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

	var versions []string             // semantic Go versions
	tagMap := make(map[string]string) // version -> tag

	for _, tag := range tagInfo {

		tagName := strings.TrimPrefix(tag.Ref, "refs/tags/")

		var maj, min, patch int // semver numbers corresponding to Go release
		var err error
		if _, err = fmt.Sscanf(tagName, "go%d.%d.%d", &maj, &min, &patch); err != nil {
			_, err = fmt.Sscanf(tagName, "go%d.%d", &maj, &min)
			patch = 0
		}

		if err != nil {
			continue
		}

		version := fmt.Sprintf("v%d.%d.%d", maj, min, patch)
		versions = append(versions, version)
		tagMap[version] = tagName

	}

	semver.Sort(versions)

	fmt.Print(tagMap[versions[len(versions)-1]])
}
