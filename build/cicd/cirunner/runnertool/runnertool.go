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

// Package runnertool provides helper functions for cirunner internal runners.
// It provides a helper class that assists with loading/writing the protos that cirunner uses to
// communicate with the runners.
//
// The normal usage if to get the invocation and verify the existense of the message the internal
// runner cares for:
//
//      helper := runnertool.MustLoad()
//      customRunnerInvocation := helper.Invocation().CustomRunnerInvocation
//      if customRunnerInvocation == nil {
//          return fmt.Errorf("no custom runner invocation found")
//      }
//      ...
//
// See //sge/build/cicd/presubmit_runner/presubmit_runner.go for an example.
package runnertool

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"sge-monorepo/libs/go/email"

	"sge-monorepo/build/cicd/cirunner/protos/cirunnerpb"
	"sge-monorepo/build/cicd/sgeb/protos/sgebpb"

	_ "github.com/golang/glog" // Register flags
	"github.com/golang/protobuf/proto"
)

var _ Helper = (*helper)(nil)
var runnerInvocationFlag = flag.String("runner-invocation", "", "path to the runner invocation proto")

// Helper is a simple interface to simplify the creation of ci internal runners.
type Helper interface {
	// Invocation returns the loaded invocation proto.
	Invocation() *cirunnerpb.RunnerInvocation
}

type helper struct {
	invocation *cirunnerpb.RunnerInvocation
}

// MustLoad loads the runner tool invocation.
// You must call flag.Parse prior to calling this function.
// Exits with an error message on failure.
func MustLoad() Helper {
	if *runnerInvocationFlag == "" {
		fmt.Println("Missing --runner-invocation flag. did you forget to call flag.Parse?")
		os.Exit(1)
	}
	bytes, err := ioutil.ReadFile(*runnerInvocationFlag)
	if err != nil {
		fmt.Printf("Failed to load invocation %s: %v\n", *runnerInvocationFlag, err)
		os.Exit(1)
	}
	h := helper{}
	h.invocation = &cirunnerpb.RunnerInvocation{}
	if err := proto.UnmarshalText(string(bytes), h.invocation); err != nil {
		fmt.Printf("Failed to unmarshal invocation proto %s: %v\n", *runnerInvocationFlag, err)
		os.Exit(1)
	}
	return &h
}

// HasInvocation returns whether the binary was launched with an invocation.
func HasInvocation() bool {
	return *runnerInvocationFlag != ""
}

// Invocation returns the loaded invocation.
func (h *helper) Invocation() *cirunnerpb.RunnerInvocation {
	return h.invocation
}

// Notifier is a helper struct to manage success/failure emails consistently.
type Notifier struct {
	client email.Client
	config *sgebpb.NotificationConfig
	emails []string
	policy sgebpb.NotificationPolicy
}

// NewNotifier returns a notification helper used by runners to send emails.
func NewNotifier(client email.Client, defaultPolicy sgebpb.NotificationPolicy, config *sgebpb.NotificationConfig) Notifier {
	var emails []string
	if config != nil {
		emails = append(emails, config.Email...)
	}
	policy := defaultPolicy
	if config != nil && config.Policy != sgebpb.NotificationPolicy_DEFAULT {
		policy = config.Policy
	}
	return Notifier{
		client: client,
		config: config,
		emails: emails,
		policy: policy,
	}
}

func (n Notifier) OnSuccess(e *email.Email, wasHealthy bool) error {
	switch n.policy {
	case sgebpb.NotificationPolicy_NOTIFY_NEVER, sgebpb.NotificationPolicy_NOTIFY_ON_FAILURE:
		return nil
	case sgebpb.NotificationPolicy_NOTIFY_ON_FAILURE_AND_RECOVERY:
		if wasHealthy {
			return nil
		}
	}
	return n.notify(e)
}

func (n Notifier) OnFailure(e *email.Email) error {
	if n.policy == sgebpb.NotificationPolicy_NOTIFY_NEVER {
		return nil
	}
	return n.notify(e)
}

func (n Notifier) notify(e *email.Email) error {
	if len(n.emails) == 0 {
		return nil
	}
	e.To = n.emails
	return n.client.Send(e)
}
