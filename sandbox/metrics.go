// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"net/http"
	"path"
	"time"

	"cloud.google.com/go/compute/metadata"
	"contrib.go.opencensus.io/exporter/prometheus"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
	mrpb "google.golang.org/genproto/googleapis/api/monitoredres"
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
	gr, err := gceResource("go-playground-sandbox")
	if err != nil {
		return nil, err
	}

	sd, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID:         projID,
		MonitoredResource: gr,
		ReportingInterval: time.Minute, // Minimum interval for stackdriver is 1 minute.
	})
	if err != nil {
		return nil, err
	}

	// Minimum interval for stackdriver is 1 minute.
	view.SetReportingPeriod(time.Minute)
	// Start the metrics exporter.
	if err := sd.StartMetricsExporter(); err != nil {
		return nil, err
	}

	return &metricService{sdExporter: sd}, nil
}

// metricService controls metric exporters.
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

// Stop flushes metrics and stops exporting. Stop should be called before exiting.
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

// gceResource populates a monitoredResource with GCE Metadata.
//
// The returned monitoredResource will have the type set to "generic_task".
func gceResource(jobName string) (*monitoredResource, error) {
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
	igName, err := instanceGroupName()
	if err != nil {
		return nil, err
	} else if igName == "" {
		igName = projID
	}

	return (*monitoredResource)(&mrpb.MonitoredResource{
		Type: "generic_task", // See: https://cloud.google.com/monitoring/api/resources#tag_generic_task
		Labels: map[string]string{
			"project_id": projID,
			"location":   zone,
			"namespace":  igName,
			"job":        jobName,
			"task_id":    iname,
		},
	}), nil
}

// instanceGroupName fetches the instanceGroupName from the instance metadata.
//
// The instance group manager applies a custom "created-by" attribute to the instance, which is not part of the
// metadata package API, and must be queried separately.
//
// An empty string will be returned if a metadata.NotDefinedError is returned when fetching metadata.
// An error will be returned if other errors occur when fetching metadata.
func instanceGroupName() (string, error) {
	ig, err := metadata.InstanceAttributeValue("created-by")
	if nde := metadata.NotDefinedError(""); err != nil && !errors.As(err, &nde) {
		return "", err
	}
	if ig == "" {
		return "", nil
	}
	// "created-by" format: "projects/{{InstanceID}}/zones/{{Zone}}/instanceGroupManagers/{{Instance Group Name}}
	ig = path.Base(ig)
	return ig, err
}
