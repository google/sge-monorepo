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

package check

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
	"sge-monorepo/build/cicd/presubmit/check/protos/checkpb"

	"github.com/golang/protobuf/proto"
)

func TestHelper(t *testing.T) {
	if err := ioutil.WriteFile("MONOREPO", []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove("MONOREPO")
	if err := ioutil.WriteFile("WORKSPACE", []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove("WORKSPACE")
	dir, err := ioutil.TempDir("", "checktest")
	if err != nil {
		t.Fatalf("could not create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)
	input := path.Join(dir, "input")
	output := path.Join(dir, "output")
	invocation := &checkpb.CheckerInvocation{
		TriggeredChecks: []*checkpb.TriggeredCheck{
			{
				Dir: "directory",
			},
		},
		ClNumber:      123,
		ClDescription: "fixed things",
	}
	invocationBytes, err := proto.Marshal(invocation)
	if err != nil {
		t.Fatalf("could not marshal invocation: %v", err)
	}
	if err := ioutil.WriteFile(input, invocationBytes, 0666); err != nil {
		t.Fatalf("could not write invocation: %v", err)
	}

	checkerInvocationFlag = &input
	checkerInvocationResultFlag = &output
	h := MustLoad()
	if !proto.Equal(h.Invocation(), invocation) {
		t.Fatal("did not load invocation correctly")
	}
	if !proto.Equal(h.OnlyCheck(), invocation.TriggeredChecks[0]) {
		t.Fatal("OnlyCheck() broken")
	}
	h.AddResult(&buildpb.Result{
		Success: false,
		Logs:    LogsFromString("", "checker failed"),
	})
	h.MustWriteResult()

	resultBytes, err := ioutil.ReadFile(output)
	if err != nil {
		t.Fatalf("could not read result: %v", err)
	}
	result := &checkpb.CheckerInvocationResult{}
	if err := proto.Unmarshal(resultBytes, result); err != nil {
		t.Fatalf("could not unmarshal result: %v", err)
	}
	got := string(result.Results[0].Logs[0].Contents)
	if got != "checker failed" {
		t.Fatalf("incorrect results got %s want %s", got, "checker failed")
	}
}
