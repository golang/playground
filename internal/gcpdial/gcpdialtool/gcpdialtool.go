// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The gcpdialtool command is an interactive validation tool for the
// gcpdial packge.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"golang.org/x/playground/internal/gcpdial"
)

var (
	proj   = flag.String("project", "golang-org", "GCP project name")
	region = flag.String("region", "us-central1", "GCP region")
	group  = flag.String("group", "play-sandbox-rigm", "regional instance group name")
)

func main() {
	flag.Parse()
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Flags() | log.Lmicroseconds)

	log.Printf("starting")
	d := gcpdial.NewRegionInstanceGroupDialer(*proj, *region, *group)

	ctx := context.Background()
	for {
		ip, err := d.PickIP(ctx)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("picked %v", ip)
		time.Sleep(time.Second)
	}
}
