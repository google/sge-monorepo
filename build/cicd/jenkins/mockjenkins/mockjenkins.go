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

// Package mockjenkins provides facilities for mocking the jenkins package.
package mockjenkins

import (
	"sge-monorepo/build/cicd/jenkins"

    "sge-monorepo/build/cicd/cirunner/protos/cirunnerpb"
)

var _ jenkins.Remote = (*mockRemote)(nil)

// NewRemote returns a no-op mock jenkins remote.
func NewRemote() jenkins.Remote {
	return &mockRemote{}
}

type mockRemote struct{}

func (*mockRemote) SendBuildRequest(string, ...jenkins.UnitOption) error {
	return nil
}

func (*mockRemote) SendPublishRequest(string, ...jenkins.UnitOption) error {
	return nil
}

func (*mockRemote) SendTestRequest(string, ...jenkins.UnitOption) error {
	return nil
}

func (*mockRemote) SendTaskRequest(string, ...jenkins.UnitOption) error {
	return nil
}

func (*mockRemote) SendPresubmitRequest(*cirunnerpb.RunnerInvocation_Presubmit) error {
	return nil
}
