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

	"golang.org/x/build/maintner/maintnerd/apipb"
	grpc "grpc.go4.org"
)

var prev = flag.Bool("prev", false, "whether to query the previous Go release, rather than the last (e.g. 1.17 versus 1.18)")

const maintnerURI = "https://maintner.golang.org"

func main() {
	flag.Parse()

	conn, err := grpc.NewClient(nil, maintnerURI)
	if err != nil {
		log.Fatalf("error creating grpc client for %q: %v", maintnerURI, err)
	}
	mc := apipb.NewMaintnerServiceClient(conn)

	resp, err := mc.ListGoReleases(context.Background(), &apipb.ListGoReleasesRequest{})
	if err != nil {
		log.Fatalln(err)
	}
	idx := 0
	if *prev {
		idx = 1
	}
	// On success, the maintner API always returns at least two releases.
	fmt.Print(resp.GetReleases()[idx].GetTagName())
}
