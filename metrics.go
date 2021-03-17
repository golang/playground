// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	BuildLatencyDistribution = view.Distribution(1, 5, 10, 15, 20, 25, 50, 75, 100, 125, 150, 200, 250, 300, 400, 500, 750, 1000, 1500, 2000, 2500, 3000, 3500, 4000, 4500, 5000, 5500, 6000, 7000, 8000, 9000, 10000, 20000, 30000)
	kGoBuildSuccess          = tag.MustNewKey("go-playground/frontend/go_build_success")
	kGoRunSuccess            = tag.MustNewKey("go-playground/frontend/go_run_success")
	kGoVetSuccess            = tag.MustNewKey("go-playground/frontend/go_vet_success")
	mGoBuildLatency          = stats.Float64("go-playground/frontend/go_build_latency", "", stats.UnitMilliseconds)
	mGoRunLatency            = stats.Float64("go-playground/frontend/go_run_latency", "", stats.UnitMilliseconds)
	mGoVetLatency            = stats.Float64("go-playground/frontend/go_vet_latency", "", stats.UnitMilliseconds)

	goBuildCount = &view.View{
		Name:        "go-playground/frontend/go_build_count",
		Description: "Number of snippets built",
		Measure:     mGoBuildLatency,
		TagKeys:     []tag.Key{kGoBuildSuccess},
		Aggregation: view.Count(),
	}
	goBuildLatency = &view.View{
		Name:        "go-playground/frontend/go_build_latency",
		Description: "Latency distribution of building snippets",
		Measure:     mGoBuildLatency,
		Aggregation: BuildLatencyDistribution,
	}
	goRunCount = &view.View{
		Name:        "go-playground/frontend/go_run_count",
		Description: "Number of snippets run",
		Measure:     mGoRunLatency,
		TagKeys:     []tag.Key{kGoRunSuccess},
		Aggregation: view.Count(),
	}
	goRunLatency = &view.View{
		Name:        "go-playground/frontend/go_run_latency",
		Description: "Latency distribution of running snippets",
		Measure:     mGoRunLatency,
		Aggregation: BuildLatencyDistribution,
	}
	goVetCount = &view.View{
		Name:        "go-playground/frontend/go_vet_count",
		Description: "Number of vet runs",
		Measure:     mGoVetLatency,
		TagKeys:     []tag.Key{kGoVetSuccess},
		Aggregation: view.Count(),
	}
	goVetLatency = &view.View{
		Name:        "go-playground/sandbox/go_vet_latency",
		Description: "Latency distribution of vet runs",
		Measure:     mGoVetLatency,
		Aggregation: BuildLatencyDistribution,
	}
)

// views should contain all measurements. All *view.View added to this
// slice will be registered and exported to the metric service.
var views = []*view.View{
	goBuildCount,
	goBuildLatency,
	goRunCount,
	goRunLatency,
	goVetCount,
	goVetLatency,
}
