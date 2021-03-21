// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// package monitoring is a convenience wrapper over the GCP Cloud monitoring API, especially in the
// sense of issuing metrics.

package monitoring

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	monitoring "cloud.google.com/go/monitoring/apiv3"
	"google.golang.org/api/iterator"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	monitoredrespb "google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ResourceType is a convenience way of defining common Monitored Resources, which roughly equate to
// metric sources. GCP has a long list of resources but normally common cases are used, which need
// to be encoded in a proto. This type permits to easily select some common use cases and have the
// library write the proto for you. If you need the whole flexibility, you can use
// |MetricBackend.SendMetric| and provide the MonitoredResource proto yourself.
// A list of resources can be found in: https://cloud.google.com/monitoring/api/resources
type ResourceType string

const (
	// Untagged resource that falls into a big bucket. In general should be avoided because of rate
	// limiting.
	Global ResourceType = "global"

	// FromGCEInstance generates a "gce_instance" MonitoredResource filled with the GCE metadata of
	// a running instance. NOTE: This needs to be running within a GCE VM.
	FromGCEInstance = "from_gce_instance"
)

// Label is a key-value associated with a metric.
type Label struct {
	Key   string
	Value string
}

// Client ------------------------------------------------------------------------------------------

// Client is a high level api wrapper over |MetricBackend|. It provides convenience functions in
// order to make Metric reporting easier. If more flexibility is needed, you will need to use
// |MetricBackend| directly.
//
// Usage:
//      client, err := monitoring.NewFromDefaultProject()
//      ...
//      err := client.SendInt64(monitoring.FromGCEInstance, "my/metric", 12)
//      ...
type Client struct {
	// Backend is the low level api used to actually send the metrics.
	backend MetricBackend
}

// New creates a |Client| with a |MetricBackend| associated with |project|.
func New(project string) (*Client, error) {
	backend, err := NewMetricBackend(project)
	if err != nil {
		return nil, err
	}
	return &Client{
		backend: backend,
	}, nil
}

// NewFromDefaultProject initializes a |Client| with a |MetricBackend| able to talk to the default
// project configured with gcloud.
// NOTE: This needs an authenticated gcloud binary in PATH.
func NewFromDefaultProject() (*Client, error) {
	out, err := exec.Command("gcloud", "config", "get-value", "core/project").Output()
	if err != nil {
		return nil, fmt.Errorf("could not obtain default project from gcloud: %v", err)
	}
	project := strings.Trim(string(out), "\r\n")
	return New(project)
}

// NewFromBackend initializes a |Client| with an already built |MetricBackend|.
func NewFromBackend(backend MetricBackend) *Client {
	return &Client{
		backend: backend,
	}
}

// SendInt64 sends ai simple int64 metric to the associated metric backend.
// |resType| is the type of resource this metric comes from. See |ResourceType|.
// |path| is the id of the metric. Expands to "custom.googleapis.com/<PATH>".
// |labels| is a slice of labels associated with the metric.
func (c *Client) SendInt64(resType ResourceType, path string, value int64, labels ...Label) error {
	resource, err := c.ObtainResource(resType)
	if err != nil {
		return fmt.Errorf("could not obtain resType %s: %v", resType, err)
	}
	point := &monitoringpb.Point{
		Interval: &monitoringpb.TimeInterval{
			EndTime: &timestamppb.Timestamp{
				Seconds: time.Now().Unix(),
			},
		},
		Value: &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_Int64Value{
				Int64Value: value,
			},
		},
	}
	return c.backend.SendMetricPoint(path, resource, point, labels...)
}

// GetMetric wraps |MetricBackend.GetMetric|.
func (c *Client) GetMetric(metricId string) (*metricpb.MetricDescriptor, bool, error) {
	return c.backend.GetMetric(metricId)
}

// GetCustomMetrics is a convenience wrapper that permits you to query for custom metrics without
// having to qualify the complete metric name.
func (c *Client) GetCustomMetric(name string) (*metricpb.MetricDescriptor, bool, error) {
	return c.backend.GetMetric(fmt.Sprintf("custom.googleapis.com/%s", name))
}

// CreateMetric wraps |MetricBackend.CreateMetric|.
func (c *Client) CreateMetric(metric *metricpb.MetricDescriptor) (*metricpb.MetricDescriptor, error) {
	return c.backend.CreateMetric(metric)
}

// ListMetricsByFilter wraps |MetricBackend.ListMetricsByFilter|.
func (c *Client) ListMetricsByFilter(filter string) ([]*metricpb.MetricDescriptor, error) {
	return c.backend.ListMetricsByFilter(filter)
}

// SendMetric is the low level call used for actually sending the metric.
// See |MetricBackend.SendMetric| for more details.
func (c *Client) SendMetricPoint(path string, resource *monitoredrespb.MonitoredResource, point *monitoringpb.Point, labels ...Label) error {
	return c.backend.SendMetricPoint(path, resource, point, labels...)
}

// ObtainResource translates within |ResourceType| and the MonitoredResource proto. This can be
// useful when using the low level |MetricBackend|.
func (c *Client) ObtainResource(resourceType ResourceType) (*monitoredrespb.MonitoredResource, error) {
	switch resourceType {
	case Global:
		return &monitoredrespb.MonitoredResource{
			Type: "global",
		}, nil
	case FromGCEInstance:
		if !metadata.OnGCE() {
			return nil, fmt.Errorf("FromGCEInstance ResourceType needs to be executed in a vm")
		}
		instanceId, err := metadata.InstanceID()
		if err != nil {
			return nil, fmt.Errorf("could not obtain vm instance id from metadata: %v", err)
		}
		zone, err := metadata.Zone()
		if err != nil {
			return nil, fmt.Errorf("could not obtain vm zone from metadata: %v", err)
		}
		return &monitoredrespb.MonitoredResource{
			Type: "gce_instance",
			Labels: map[string]string{
				"project_id":  c.backend.Project(),
				"instance_id": instanceId,
				"zone":        zone,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsuppored ResourceType %s", resourceType)
	}
}

// MetricBackend -------------------------------------------------------------------------------------

// MetricBackend is an abstract interface to refer about metrics in your project.
// Each client is associated with a GCP project. You will need to provide the resource and data
// point protos directly. This provides much flexibility at the cost of convenience.
type MetricBackend interface {
	// Project returns the associated GCP project the api is bound to.
	Project() string

	// GetMetric queries for a particular metric within the project.
	// |metricName| is the full METRIC_ID. Examples:
	//      compute.googleapis.com/instance/disk/read_bytes_count
	//      custom.googleapis.com/my/custom/metric_value
	// The boolean indicates whether the secret exists or not, which is different from a runtime
	// error (eg. Could not connect).
	GetMetric(metricId string) (*metricpb.MetricDescriptor, bool, error)

	// CreateMetric creates a new metric in the project according to the given MetricDescriptor.
	// If successful, returns the newly created MetricDescriptor.
	CreateMetric(metric *metricpb.MetricDescriptor) (*metricpb.MetricDescriptor, error)

	// ListMetricsByFilter lists the metrics associated with the project.
	// |filter| is a GCP filter: https://cloud.google.com/monitoring/api/v3/filters. If empty, it
	// will list all the *custom* metrics.
	ListMetricsByFilter(filter string) ([]*metricpb.MetricDescriptor, error)

	// SendMetricPoint sends a Data point into the a particular metric.
	// |path| is the "name" that the metric will have. Expands to "custom.googleapis.com/<PATH>"
	// |labels| are any key-value labels that need to be associated with the metric.
	SendMetricPoint(path string, resource *monitoredrespb.MonitoredResource, point *monitoringpb.Point, labels ...Label) error
}

// NewMetricBackend creates a |MetricBackend| associated with the given GCP |project|.
func NewMetricBackend(project string) (MetricBackend, error) {
	ctx := context.Background()
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not create monitoring client: %v", err)
	}
	return &impl{
		project: project,
		ctx:     ctx,
		client:  client,
	}, nil
}

type impl struct {
	project string
	ctx     context.Context
	client  *monitoring.MetricClient
}

func (backend *impl) Project() string {
	return backend.project
}

func (backend *impl) GetMetric(metricId string) (*metricpb.MetricDescriptor, bool, error) {
	name := fmt.Sprintf("projects/%s/metricDescriptors/%s", backend.project, metricId)
	request := &monitoringpb.GetMetricDescriptorRequest{
		Name: name,
	}
	metric, err := backend.client.GetMetricDescriptor(backend.ctx, request)
	if err != nil {
		// Check for non found usecase.
		if strings.Contains(err.Error(), "NotFound") {
			return nil, false, nil
		}
		return nil, false, err
	}
	return metric, true, nil
}

func (backend *impl) CreateMetric(metric *metricpb.MetricDescriptor) (*metricpb.MetricDescriptor, error) {
	projectUrl := "projects/" + backend.project
	request := &monitoringpb.CreateMetricDescriptorRequest{
		Name:             projectUrl,
		MetricDescriptor: metric,
	}
	return backend.client.CreateMetricDescriptor(backend.ctx, request)
}

func (backend *impl) ListMetricsByFilter(filter string) ([]*metricpb.MetricDescriptor, error) {
	if filter == "" {
		filter = "metric.type = starts_with(\"custom.googleapis.com\")"
	}
	projectUrl := "projects/" + backend.project
	request := &monitoringpb.ListMetricDescriptorsRequest{
		Name:   projectUrl,
		Filter: filter,
	}
	it := backend.client.ListMetricDescriptors(backend.ctx, request)
	var metrics []*metricpb.MetricDescriptor
	for {
		metric, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func (backend *impl) SendMetricPoint(path string, resource *monitoredrespb.MonitoredResource, point *monitoringpb.Point, labels ...Label) error {
	projectUrl := "projects/" + backend.project
	metricType := "custom.googleapis.com/" + path
	request := &monitoringpb.CreateTimeSeriesRequest{
		Name: projectUrl,
		TimeSeries: []*monitoringpb.TimeSeries{
			{
				Metric: &metricpb.Metric{
					Type:   metricType,
					Labels: labelSliceToMap(labels...),
				},
				Resource: resource,
				Points:   []*monitoringpb.Point{point},
			},
		},
	}
	if err := backend.client.CreateTimeSeries(backend.ctx, request); err != nil {
		return fmt.Errorf("could not send metrict %s: %v", metricType, err)
	}
	return nil
}

func labelSliceToMap(labels ...Label) map[string]string {
	m := map[string]string{}
	for _, label := range labels {
		m[label.Key] = label.Value
	}
	return m
}
