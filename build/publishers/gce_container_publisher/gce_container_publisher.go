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

// Binary gce_container_publisher is meant to update a container running on a VM.

package main

import(
    "flag"
    "fmt"
    "os"
    "os/exec"

	"sge-monorepo/libs/go/log"
	"sge-monorepo/build/cicd/sgeb/buildtool"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
)

var flags = struct {
    project string
    instance string
    container string
}{}

func updateContainer() error {
    args := []string{
        "gcloud",
        "compute",
        "instances",
        "update-container", flags.instance,
        "--container-image", flags.container,
        "--project", flags.project,
    }
    log.Infof("Running: %s", args)
    cmd := exec.Command(args[0], args[1:]...)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("%s", out)
    }
    return nil
}

// This wrapper exists because os.Exit does not execute defer calls.
func internalMain() error {
    flag.StringVar(&flags.project, "project", "", "GCP project the instance belongs to")
    flag.StringVar(&flags.instance, "instance", "", "Name of the instance to update")
    flag.StringVar(&flags.container, "container", "", "Path of the container to deploy")
    flag.Parse()
    helper := buildtool.MustLoad()
    if err := validateFlags(); err != nil {
        return fmt.Errorf("could not validate flags: %w", err)
    }

    log.AddSink(log.NewGlog())
    defer log.Shutdown()

    if err := updateContainer(); err != nil {
        return fmt.Errorf("could not update container: %w", err)
    }
    result := &buildpb.PublishResult{
        Name: fmt.Sprintf("VM: %s - Container: %s", flags.instance, flags.container),
        Files: []*buildpb.PublishedFile{{Size: 0}},
    }
    helper.MustWritePublishResult(&buildpb.PublishInvocationResult{
        PublishResults: []*buildpb.PublishResult{result},
    })
    return nil
}

func validateFlags() error {
    if flags.project == "" {
        flag.PrintDefaults()
        return fmt.Errorf("flag %q cannot be empty", "project")
    }
    if flags.instance == "" {
        flag.PrintDefaults()
        return fmt.Errorf("flag %q cannot be empty", "instance")
    }
    if flags.container == "" {
        flag.PrintDefaults()
        return fmt.Errorf("flag %q cannot be empty", "container")
    }
    return nil
}

func main() {
    if err := internalMain(); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
