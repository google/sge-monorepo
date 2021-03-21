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
	"net/url"
	"os/user"
	"strings"
	"time"

	"sge-monorepo/build/cicd/jenkins"
	"sge-monorepo/libs/go/p4lib"

	"sge-monorepo/build/cicd/cirunner/protos/cirunnerpb"
	"sge-monorepo/build/cicd/cirunner/runners/unit_runner/protos/unit_runnerpb"

	"github.com/golang/protobuf/proto"
)

// p4 keys may not contain slashes, skip leading // and replace the rest with ':'
// Example key: sge-unit-runner:build:cicd:sgeb:sgeb_publish
func p4Key(user, label string) string {
	if strings.HasPrefix(label, "//") {
		label = label[2:]
	}
	key := fmt.Sprintf("sgeb-remote:%s:%s", user, label)
	key = strings.ReplaceAll(key, "/", ":")
	key = strings.ReplaceAll(key, "\\", ":")
	return key
}

type remoteRequest struct {
	action   string
	label    string
	logLevel string
	change   int
	args     []string
}

// remote executes a sgeb action on a remote build machine backed by Jenkins.
func remote(req remoteRequest) error {
	creds, err := jenkins.CredentialsForProject("INSERT_PROJECT")
	if err != nil {
		return fmt.Errorf("could not get credentials: %w", err)
	}
	// Because we're running locally, we need to change the host. When running in the workstation,
	// we pipe locally.
	creds.Host = "INSERT_HOST"

	u, err := user.Current()
	if err != nil {
		return fmt.Errorf("could not get current user: %w", err)
	}
	// Clear the key before sending the request.
	p4 := p4lib.New()
	key := p4Key(u.Username, req.label)
	if err := p4.KeySet(key, "0"); err != nil {
		return fmt.Errorf("could not send P4 key %q: %w", key, err)
	}
	fmt.Printf("Running remotely. P4 Task key: %s\n", key)
	options := func(opts *jenkins.UnitOptions) {
		opts.Change = req.change
		opts.TaskKey = key
		opts.LogLevel = req.logLevel
		opts.Args = req.args
	}
	remote := jenkins.NewRemote(creds)
	switch req.action {
	case "build":
		if err := remote.SendBuildRequest(req.label, options); err != nil {
			return fmt.Errorf("could not send build request: %w", err)
		}
	case "publish":
		if err := remote.SendPublishRequest(req.label, options); err != nil {
			return fmt.Errorf("could not send publish request: %w", err)
		}
	case "test":
		if err := remote.SendTestRequest(req.label, options); err != nil {
			return fmt.Errorf("could not send test request: %w", err)
		}
	default:
		return fmt.Errorf("unsupported remote action %q", req.action)
	}
	// Wait for logs.
	fmt.Println("Waiting for the build machine to expose logs...")
	var task *unit_runnerpb.Task
	for i := 0; i < 150; i++ {
		t, ok, err := lookForTask(p4, creds, key)
		if err != nil {
			return fmt.Errorf("could not get logs: %w", err)
		}
		// If the key is still not set, we wait.
		if !ok {
			time.Sleep(2 * time.Second)
			continue
		}
		task = t
		break
	}
	if task == nil {
		return fmt.Errorf("timeout waiting on P4 key %q", key)
	}
	fmt.Printf("Logs available at: %s\n", task.ResultsUrl)
	return nil
}

func lookForTask(p4 p4lib.P4, creds *cirunnerpb.JenkinsCredentials, key string) (*unit_runnerpb.Task, bool, error) {
	// Obtain the proto from the key.
	value, err := p4.KeyGet(key)
	if err != nil {
		return nil, false, fmt.Errorf("could not get P4 key %q: %w", key, err)
	}
	// "0" means the key is unset.
	value = strings.TrimSpace(value)
	if value == "0" {
		return nil, false, nil
	}
	task := &unit_runnerpb.Task{}
	if err := proto.UnmarshalText(value, task); err != nil {
		return nil, false, fmt.Errorf("could not unmarshall proto: %w", err)
	}
	if task.ResultsUrl != "" {
		// We got an URL, we do a fixup. We need to make sure the hostname we use to get the
		// logs is the same we used to make the request. This will permit us to bypass the fact
		// that Jenkins might return an proxied URL.
		url, err := url.Parse(task.ResultsUrl)
		if err != nil {
			return nil, false, fmt.Errorf("could not parse url %q: %w", task.ResultsUrl, err)
		}
		url.Host = fmt.Sprintf("%s:%d", creds.Host, creds.Port)
		task.ResultsUrl = url.String()
	}
	return task, true, nil
}
