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

// Binary dev_runner is a simple runner to executed manually in the CI dev environment in order to
// test out code.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
	"time"

	_ "sge-monorepo/build/cicd/cirunner/runnertool"
	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/sgeb/build"
	"sge-monorepo/libs/go/cloud/compute"
	"sge-monorepo/libs/go/cloud/monitoring"
	"sge-monorepo/libs/go/cloud/secretmanager"
	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/log/cloudlog"
	"sge-monorepo/libs/go/p4lib"

	"sge-monorepo/build/cicd/sgeb/protos/buildpb"

	"cloud.google.com/go/spanner"
	labelpb "google.golang.org/genproto/googleapis/api/label"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
)

const (
	project = "INSERT_PROJECT"
	zone    = "INSERT_ZONE"
)

func devRunner() error {
	fmt.Println("Executing dev_runner")
	return doLogs()
}

func doLogs() error {
	cloudLogger, err := cloudlog.New("dev_runner", cloudlog.WithLabels(map[string]string{
		"runner": "today",
		"when":   time.Now().String(),
	}))
	if err != nil {
		return fmt.Errorf("could not get cloud logger: %w", err)
	}
	log.AddSink(log.NewGlog(), cloudLogger)
	defer log.Shutdown()

	log.Infof("Test Info")
	log.Warningf("Test Warning")
	log.Errorf("Test Error")
	return nil
}

var dirs = []string{
	"E:\\WS\\prod\\sge",
	"C:\\p4\\sge",
}

func printBuildResult(logs io.Writer, l monorepo.Label, result *buildpb.BuildResult) {
	if result.OverallResult.Success {
		artifacts := result.BuildResult.GetArtifactSet().GetArtifacts()
		if len(artifacts) > 0 {
			fmt.Printf("%s successfully built\n", l)
			for _, output := range artifacts {
				uri := output.Uri
				prefix := "file:///"
				if strings.HasPrefix(uri, prefix) {
					uri = uri[len(prefix):]
				}
				// Make it easier to copy-paste result into a Windows command prompt.
				if runtime.GOOS == "windows" {
					uri = strings.ReplaceAll(uri, `/`, `\`)
				}
				fmt.Printf("  %s\n", uri)
			}
		} else {
			fmt.Printf("%s successfully built (no outputs produced)\n", l)
		}
		return
	}
	build.PrintFailedBuildResult(logs, result)
}

func runSpanner() error {
	ctx := context.Background()
	db := "INSERT_PROJECT"
	client, err := spanner.NewClient(ctx, db)
	if err != nil {
		return fmt.Errorf("could not create spanner client: %v", err)
	}
	defer client.Close()

	row, err := client.Single().ReadRow(ctx, "table", spanner.Key{int64(22)}, []string{"number", "text"})
	if err != nil {
		return fmt.Errorf("could not read row: %v", err)
	}
	number := int64(0)
	text := ""
	if err := row.Columns(&number, &text); err != nil {
		return fmt.Errorf("could not extract row: %v", err)
	}
	fmt.Printf("NUMBER: %d, TEXT: %s\n", int(number), text)
	return nil
}

func env() error {
	for _, e := range os.Environ() {
		fmt.Println(e)
	}
	fmt.Printf("TERM: %q\n", os.Getenv("TERM"))
	fmt.Printf("FOO: %q\n", os.Getenv("FOO"))
	return nil
}

func runCompute() error {
	client, err := compute.NewFromDefaultProject()
	if err != nil {
		return fmt.Errorf("could not create compute: %v", err)
	}
	instances, err := client.InstancesList("us-central1-a", "us-central1-c")
	if err != nil {
		return fmt.Errorf("could not list all instances: %v", err)
	}
	for _, instance := range instances {
		fmt.Println("- ", instance.Name)
	}
	return nil
}

func p4clients() error {
	p4 := p4lib.New()

	client, err := p4.Client("")
	if err != nil {
		return fmt.Errorf("could not get default p4 client: %v", err)
	}
	fmt.Println(client.String())
	clients, err := p4.Clients()
	if err != nil {
		return fmt.Errorf("could not list clients: %v", err)
	}
	fmt.Println("Listing clients")
	for _, c := range clients {
		fmt.Println("- ", c)
	}
	return nil
}

func metrics() error {
	project := "INSERT_PROJECT"
	client, err := monitoring.New(project)
	if err != nil {
		return fmt.Errorf("could not create monitoring client for project %s: %v", project, err)
	}
	metrics, err := client.ListMetricsByFilter("")
	if err != nil {
		return fmt.Errorf("could not list metrics: %v", err)
	}
	for _, metric := range metrics {
		fmt.Println("METRIC:", metric.Type)
	}

	{
		metric := "presubmit/asfdasdasd"
		_, ok, err := client.GetCustomMetric(metric)
		if err != nil {
			return err
		} else if !ok {
			return fmt.Errorf("could not find metric %q", metric)
		}
	}

	{
		metric := &metricpb.MetricDescriptor{
			Type: "custom.googleapis.com/test/AAAAA",
			Labels: []*labelpb.LabelDescriptor{
				{
					Key:         "name",
					ValueType:   labelpb.LabelDescriptor_STRING,
					Description: "Test metric",
				},
			},
			MetricKind:  metricpb.MetricDescriptor_GAUGE,
			ValueType:   metricpb.MetricDescriptor_INT64,
			Unit:        "ms",
			Description: "Milliseconds taken to execute a given presubmit check",
		}
		result, err := client.CreateMetric(metric)
		if err != nil {
			return fmt.Errorf("could not create test metric: %v", err)
		}
		fmt.Println(result)
	}
	return nil
}

func secrets() error {
	secrets, err := secretmanager.NewFromDefaultProject()
	if err != nil {
		return fmt.Errorf("could not create secretmanager: %v", err)
	}
	fmt.Printf("Associated secrets for project %q\n", secrets.Project())
	secretName := "cirunner_environment"
	secret, ok, err := secrets.AccessLatest(secretName)
	if err != nil {
		return fmt.Errorf("could not obtain cirunner_email secret: %v", err)
	} else if !ok {
		return fmt.Errorf("secret %q not found", secretName)
	}
	fmt.Println("SECRET:", secret)
	return nil
}

func main() {
	flag.Parse()
	if err := devRunner(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
