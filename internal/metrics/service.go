// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package metrics provides a service for reporting metrics to
// Stackdriver, or locally during development.
package metrics

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"path"
	"time"

	"cloud.google.com/go/compute/metadata"
	"contrib.go.opencensus.io/exporter/prometheus"
	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/stats/view"
	"google.golang.org/appengine"
	mrpb "google.golang.org/genproto/googleapis/api/monitoredres"
)

// NewService initializes a *Service.
//
// The Service returned is configured to send metric data to
// StackDriver. When not running on GCE, it will host metrics through
// a prometheus HTTP handler.
//
// views will be passed to view.Register for export to the metric
// service.
func NewService(resource *MonitoredResource, views []*view.View) (*Service, error) {
	err := view.Register(views...)
	if err != nil {
		return nil, err
	}

	if !metadata.OnGCE() {
		view.SetReportingPeriod(5 * time.Second)
		pe, err := prometheus.NewExporter(prometheus.Options{})
		if err != nil {
			return nil, fmt.Errorf("prometheus.NewExporter: %w", err)
		}
		view.RegisterExporter(pe)
		return &Service{pExporter: pe}, nil
	}

	projID, err := metadata.ProjectID()
	if err != nil {
		return nil, err
	}
	if resource == nil {
		return nil, errors.New("resource is required, got nil")
	}
	sde, err := stackdriver.NewExporter(stackdriver.Options{
		ProjectID:         projID,
		MonitoredResource: resource,
		ReportingInterval: time.Minute, // Minimum interval for Stackdriver is 1 minute.
	})
	if err != nil {
		return nil, err
	}

	// Minimum interval for Stackdriver is 1 minute.
	view.SetReportingPeriod(time.Minute)
	// Start the metrics exporter.
	if err := sde.StartMetricsExporter(); err != nil {
		sde.Close()
		return nil, err
	}

	return &Service{sdExporter: sde}, nil
}

// Service controls metric exporters.
type Service struct {
	sdExporter *stackdriver.Exporter
	pExporter  *prometheus.Exporter
}

func (m *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if m.pExporter != nil {
		m.pExporter.ServeHTTP(w, r)
		return
	}
	http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
}

// Stop flushes metrics and stops exporting. Stop should be called
// before exiting.
func (m *Service) Stop() {
	if sde := m.sdExporter; sde != nil {
		// Flush any unsent data before exiting.
		sde.Flush()

		sde.StopMetricsExporter()
	}
}

// MonitoredResource wraps a *mrpb.MonitoredResource to implement the
// monitoredresource.MonitoredResource interface.
type MonitoredResource mrpb.MonitoredResource

func (r *MonitoredResource) MonitoredResource() (resType string, labels map[string]string) {
	return r.Type, r.Labels
}

// GCEResource populates a MonitoredResource with GCE Metadata.
//
// The returned MonitoredResource will have the type set to "generic_task".
func GCEResource(jobName string) (*MonitoredResource, error) {
	projID, err := metadata.ProjectID()
	if err != nil {
		return nil, err
	}
	zone, err := metadata.Zone()
	if err != nil {
		return nil, err
	}
	inst, err := metadata.InstanceName()
	if err != nil {
		return nil, err
	}
	group, err := instanceGroupName()
	if err != nil {
		return nil, err
	} else if group == "" {
		group = projID
	}

	return (*MonitoredResource)(&mrpb.MonitoredResource{
		Type: "generic_task", // See: https://cloud.google.com/monitoring/api/resources#tag_generic_task
		Labels: map[string]string{
			"project_id": projID,
			"location":   zone,
			"namespace":  group,
			"job":        jobName,
			"task_id":    inst,
		},
	}), nil
}

// GAEResource returns a *MonitoredResource with fields populated and
// for StackDriver.
//
// The resource will be in StackDrvier's gae_instance type.
func GAEResource(ctx context.Context) (*MonitoredResource, error) {
	// appengine.IsAppEngine is confusingly false as we're using a custom
	// container and building without the appenginevm build constraint.
	// Check metadata.OnGCE instead.
	if !metadata.OnGCE() {
		return nil, fmt.Errorf("not running on appengine")
	}
	projID, err := metadata.ProjectID()
	if err != nil {
		return nil, err
	}
	return (*MonitoredResource)(&mrpb.MonitoredResource{
		Type: "gae_instance",
		Labels: map[string]string{
			"project_id":  projID,
			"module_id":   appengine.ModuleName(ctx),
			"version_id":  appengine.VersionID(ctx),
			"instance_id": appengine.InstanceID(),
			"location":    appengine.Datacenter(ctx),
		},
	}), nil
}

// instanceGroupName fetches the instanceGroupName from the instance
// metadata.
//
// The instance group manager applies a custom "created-by" attribute
// to the instance, which is not part of the metadata package API, and
// must be queried separately.
//
// An empty string will be returned if a metadata.NotDefinedError is
// returned when fetching metadata. An error will be returned if other
// errors occur when fetching metadata.
func instanceGroupName() (string, error) {
	ig, err := metadata.InstanceAttributeValue("created-by")
	if errors.As(err, new(metadata.NotDefinedError)) {
		return "", nil
	} else if err != nil {
		return "", err
	}
	// "created-by" format: "projects/{{InstanceID}}/zones/{{Zone}}/instanceGroupManagers/{{Instance Group Name}}
	ig = path.Base(ig)
	return ig, nil
}
