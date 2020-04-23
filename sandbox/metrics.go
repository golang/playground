// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"net/http"
	"time"

	"cloud.google.com/go/compute/metadata"
	"contrib.go.opencensus.io/exporter/prometheus"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	mrpb "google.golang.org/genproto/googleapis/api/monitoredres"
)

// Customizations of ochttp views. Views are updated as follows:
//  * The views are prefixed with go-playground-sandbox.
//  * ochttp.KeyServerRoute is added as a tag to label metrics per-route.
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

// newMetricService initializes a *metricService.
//
// The metricService returned is configured to send metric data to StackDriver.
// When the sandbox is not running on GCE, it will host metrics through a prometheus HTTP handler.
func newMetricService() (*metricService, error) {
	err := view.Register(
		ServerRequestCountView,
		ServerRequestBytesView,
		ServerResponseBytesView,
		ServerLatencyView,
		ServerRequestCountByMethod,
		ServerResponseCountByStatusCode)
	if err != nil {
		return nil, err
	}

	if !metadata.OnGCE() {
		view.SetReportingPeriod(5 * time.Second)
		pe, err := prometheus.NewExporter(prometheus.Options{})
		if err != nil {
			return nil, fmt.Errorf("newMetricsService(): prometheus.NewExporter: %w", err)
		}
		view.RegisterExporter(pe)
		return &metricService{pExporter: pe}, nil
	}

	projID, err := metadata.ProjectID()
	if err != nil {
		return nil, err
	}
	zone, err := metadata.Zone()
	if err != nil {
		return nil, err
	}
	iname, err := metadata.InstanceName()
	if err != nil {
		return nil, err
	}

	sd, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID: projID,
		MonitoredResource: (*monitoredResource)(&mrpb.MonitoredResource{
			Type: "generic_task",
			Labels: map[string]string{
				"instance_id": iname,
				"job":         "go-playground-sandbox",
				"project_id":  projID,
				"zone":        zone,
			},
		}),
		ReportingInterval: time.Minute, // Minimum interval for stackdriver is 1 minute.
	})
	if err != nil {
		return nil, err
	}

	// Minimum interval for stackdriver is 1 minute.
	view.SetReportingPeriod(time.Minute)
	view.RegisterExporter(sd)
	// Start the metrics exporter.
	if err := sd.StartMetricsExporter(); err != nil {
		return nil, err
	}

	return &metricService{sdExporter: sd}, nil
}

type metricService struct {
	sdExporter *stackdriver.Exporter
	pExporter  *prometheus.Exporter
}

func (m *metricService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.pExporter != nil {
		m.pExporter.ServeHTTP(w, r)
		return
	}
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

func (m *metricService) Stop() {
	if sde := m.sdExporter; sde != nil {
		// Flush any unsent data before exiting.
		sde.Flush()

		sde.StopMetricsExporter()
	}
}

// monitoredResource wraps a *mrpb.MonitoredResource to implement the
// monitoredresource.MonitoredResource interface.
type monitoredResource mrpb.MonitoredResource

func (r *monitoredResource) MonitoredResource() (resType string, labels map[string]string) {
	return r.Type, r.Labels
}
