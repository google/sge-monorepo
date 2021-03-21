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

package main

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/presubmit"
	"sge-monorepo/libs/go/cloud/monitoring"
	"sge-monorepo/libs/go/log"

	"sge-monorepo/build/cicd/presubmit/protos/presubmitpb"

	labelpb "google.golang.org/genproto/googleapis/api/label"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
)

// CheckResult represents a simple check result.
type CheckResult struct {
	Check  presubmit.Check
	Begin  time.Time
	End    time.Time
	Result *presubmitpb.CheckResult
}

func (c *CheckResult) Diff() time.Duration {
	return c.End.Sub(c.Begin)
}

// PresubmitListerner is a custom listener presubmit_runner users for tracking results.
type PresubmitListener struct {
	metrics            *monitoring.Client
	presubmitId        string
	presubmitStartTime time.Time
	presubmitEndTime   time.Time
	checkStartTime     time.Time
	results            []CheckResult
	wg                 sync.WaitGroup
}

// NewPresubmitListener returns a custom listener for presubmit_runner.
func NewPresubmitListener(metrics *monitoring.Client) *PresubmitListener {
	listener := &PresubmitListener{}
	// If we have a monitoring client, we attempt to create the metric that will hold the presubmit
	// results. If there is an error, we do not append the metric client to the listener, which
	// will make it not submit any metric data points.
	if metrics != nil {
		if err := EnsureMetricsExist(metrics); err != nil {
			log.Warningf("could not ensure metrics exist: %v", err)
		} else {
			listener.metrics = metrics
		}
	}
	return listener
}

func EnsureMetricsExist(metrics *monitoring.Client) error {
	// We query to see if the presubmit metric already exists.
	metricName := "presubmit/check_duration"
	_, ok, err := metrics.GetCustomMetric(metricName)
	if err != nil {
		return fmt.Errorf("could not get metric %q: %v", metricName, err)
	} else if !ok {
		// If we didn't find the metric, we create it.
		metric := &metricpb.MetricDescriptor{
			Type: "custom.googleapis.com/presubmit/check_duration",
			Labels: []*labelpb.LabelDescriptor{
				{
					Key:         "name",
					ValueType:   labelpb.LabelDescriptor_STRING,
					Description: "Name of the presubmit check",
				},
			},
			MetricKind:  metricpb.MetricDescriptor_GAUGE,
			ValueType:   metricpb.MetricDescriptor_INT64,
			Unit:        "ms",
			Description: "Milliseconds taken to execute a given presubmit check",
		}
		if _, err := metrics.CreateMetric(metric); err != nil {
			return fmt.Errorf("could not create check_duration metric: %v", err)
		}
	}
	return nil
}

func (p *PresubmitListener) OnPresubmitStart(mr monorepo.Monorepo, presubmitId string, checks []presubmit.Check) {
	p.presubmitId = presubmitId
	p.presubmitStartTime = time.Now()
}

func (p *PresubmitListener) OnCheckStart(check presubmit.Check) {
	p.checkStartTime = time.Now()
}

func (p *PresubmitListener) OnCheckResult(mdPath monorepo.Path, check presubmit.Check, result *presubmitpb.CheckResult) {
	r := CheckResult{
		Check:  check,
		Begin:  p.checkStartTime,
		End:    time.Time{},
		Result: result,
	}
	p.results = append(p.results, r)
	// Send the metric asynchronously.
	p.wg.Add(1)
	go func(result CheckResult) {
		defer p.wg.Done()
		if p.metrics == nil {
			return
		}
		path := "presubmit/check_duration"
		ms := result.End.Sub(result.Begin).Milliseconds()
		label := monitoring.Label{
			Key:   "name",
			Value: result.Check.Id(),
		}
		if err := p.metrics.SendInt64(monitoring.FromGCEInstance, path, ms, label); err != nil {
			log.Warningf("Could not send metrics for %s: %v", result.Check.Name(), err)
		}
	}(r)
}

func (p *PresubmitListener) OnPresubmitEnd(success bool) {
	p.presubmitEndTime = time.Now()
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		// Send the overall timing metrics.
		if p.metrics == nil {
			return
		}
		// TODO: Do we want a separate metric for overall timing? Until we define that, we don't
        //       send overall timing.
		// ms := p.end.Sub(p.begin).Milliseconds()
		// if err := p.metrics.SendInt64(monitoring.FromGCEInstance, "presubmit/overall", ms); err != nil {
		// 	glog.Warningf("Could not send overall timing metrics: %v", err)
		// }
	}()
}

// PrintTimings prints all the timing information.
func (p *PresubmitListener) PrintTimings() {
	overallDiff := p.presubmitEndTime.Sub(p.presubmitStartTime)
	log.Infof("Overall timing: %v\n", overallDiff)
	// We sort decreasing.
	sort.Slice(p.results, func(i, j int) bool {
		return p.results[i].Diff() > p.results[j].Diff()
	})
	for _, c := range p.results {
		diff := c.End.Sub(c.Begin)
		ratio := float64(diff.Microseconds()) / float64(overallDiff.Microseconds())
		log.Infof("-  %s: %v (%.2f%%)\n", c.Check.Id(), c.End.Sub(c.Begin), ratio*100)
	}
}

// WaitForMetrics waits that all the async metrics are done.
func (p *PresubmitListener) WaitForMetrics() {
	p.wg.Wait()
}

func idToPath(id string) string {
	if strings.HasPrefix(id, "//") {
		id = id[2:]
	}
	id = strings.ReplaceAll(id, ":", "/")
	return id
}
