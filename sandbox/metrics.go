// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

var (
	kContainerCreateSuccess = tag.MustNewKey("go-playground/sandbox/container_create_success")
	mContainers             = stats.Int64("go-playground/sandbox/container_count", "number of sandbox containers", stats.UnitDimensionless)
	mUnwantedContainers     = stats.Int64("go-playground/sandbox/unwanted_container_count", "number of sandbox containers that are unexpectedly running", stats.UnitDimensionless)
	mMaxContainers          = stats.Int64("go-playground/sandbox/max_container_count", "target number of sandbox containers", stats.UnitDimensionless)
	mContainerCreateLatency = stats.Float64("go-playground/sandbox/container_create_latency", "", stats.UnitMilliseconds)

	containerCount = &view.View{
		Name:        "go-playground/sandbox/container_count",
		Description: "Number of running sandbox containers",
		TagKeys:     nil,
		Measure:     mContainers,
		Aggregation: view.LastValue(),
	}
	unwantedContainerCount = &view.View{
		Name:        "go-playground/sandbox/unwanted_container_count",
		Description: "Number of running sandbox containers that are not being tracked by the sandbox",
		TagKeys:     nil,
		Measure:     mUnwantedContainers,
		Aggregation: view.LastValue(),
	}
	maxContainerCount = &view.View{
		Name:        "go-playground/sandbox/max_container_count",
		Description: "Maximum number of containers to create",
		TagKeys:     nil,
		Measure:     mMaxContainers,
		Aggregation: view.LastValue(),
	}
	containerCreateCount = &view.View{
		Name:        "go-playground/sandbox/container_create_count",
		Description: "Number of containers created",
		Measure:     mContainerCreateLatency,
		TagKeys:     []tag.Key{kContainerCreateSuccess},
		Aggregation: view.Count(),
	}
	containerCreationLatency = &view.View{
		Name:        "go-playground/sandbox/container_create_latency",
		Description: "Latency distribution of container creation",
		Measure:     mContainerCreateLatency,
		Aggregation: ochttp.DefaultLatencyDistribution,
	}
)

// Customizations of ochttp views. Views are updated as follows:
//   - The views are prefixed with go-playground-sandbox.
//   - ochttp.KeyServerRoute is added as a tag to label metrics per-route.
var (
	ServerRequestCountView = &view.View{
		Name:        "go-playground-sandbox/http/server/request_count",
		Description: "Count of HTTP requests started",
		Measure:     ochttp.ServerRequestCount,
		TagKeys:     []tag.Key{ochttp.KeyServerRoute},
		Aggregation: view.Count(),
	}
	ServerRequestBytesView = &view.View{
		Name:        "go-playground-sandbox/http/server/request_bytes",
		Description: "Size distribution of HTTP request body",
		Measure:     ochttp.ServerRequestBytes,
		TagKeys:     []tag.Key{ochttp.KeyServerRoute},
		Aggregation: ochttp.DefaultSizeDistribution,
	}
	ServerResponseBytesView = &view.View{
		Name:        "go-playground-sandbox/http/server/response_bytes",
		Description: "Size distribution of HTTP response body",
		Measure:     ochttp.ServerResponseBytes,
		TagKeys:     []tag.Key{ochttp.KeyServerRoute},
		Aggregation: ochttp.DefaultSizeDistribution,
	}
	ServerLatencyView = &view.View{
		Name:        "go-playground-sandbox/http/server/latency",
		Description: "Latency distribution of HTTP requests",
		Measure:     ochttp.ServerLatency,
		TagKeys:     []tag.Key{ochttp.KeyServerRoute},
		Aggregation: ochttp.DefaultLatencyDistribution,
	}
	ServerRequestCountByMethod = &view.View{
		Name:        "go-playground-sandbox/http/server/request_count_by_method",
		Description: "Server request count by HTTP method",
		TagKeys:     []tag.Key{ochttp.Method, ochttp.KeyServerRoute},
		Measure:     ochttp.ServerRequestCount,
		Aggregation: view.Count(),
	}
	ServerResponseCountByStatusCode = &view.View{
		Name:        "go-playground-sandbox/http/server/response_count_by_status_code",
		Description: "Server response count by status code",
		TagKeys:     []tag.Key{ochttp.StatusCode, ochttp.KeyServerRoute},
		Measure:     ochttp.ServerLatency,
		Aggregation: view.Count(),
	}
)

// views should contain all measurements. All *view.View added to this
// slice will be registered and exported to the metric service.
var views = []*view.View{
	containerCount,
	unwantedContainerCount,
	maxContainerCount,
	containerCreateCount,
	containerCreationLatency,
	ServerRequestCountView,
	ServerRequestBytesView,
	ServerResponseBytesView,
	ServerLatencyView,
	ServerRequestCountByMethod,
	ServerResponseCountByStatusCode,
}
