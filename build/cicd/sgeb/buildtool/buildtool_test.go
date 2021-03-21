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

package buildtool

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"sge-monorepo/build/cicd/sgeb/build"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"

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

	dir, err := ioutil.TempDir("", "buildtool")
	if err != nil {
		t.Fatalf("could not create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)
	input := path.Join(dir, "input")
	output := path.Join(dir, "output")
	invocation := &buildpb.ToolInvocation{}
	invocationBytes, err := proto.Marshal(invocation)
	if err != nil {
		t.Fatalf("could not marshal invocation: %v", err)
	}
	if err := ioutil.WriteFile(input, invocationBytes, 0666); err != nil {
		t.Fatalf("could not write invocation: %v", err)
	}

	toolInvocationFlag = &input
	toolInvocationResultFlag = &output
	h := MustLoad()
	if !proto.Equal(h.Invocation(), invocation) {
		t.Fatal("did not load invocation correctly")
	}
	h.MustWriteBuildResult(&buildpb.BuildInvocationResult{
		Result: &buildpb.Result{
			Success: false,
			Logs:    build.LogsFromString("", "builder failed"),
		},
	})

	resultBytes, err := ioutil.ReadFile(output)
	if err != nil {
		t.Fatalf("could not read result: %v", err)
	}
	result := &buildpb.BuildInvocationResult{}
	if err := proto.Unmarshal(resultBytes, result); err != nil {
		t.Fatalf("could not unmarshal result: %v", err)
	}
	got := string(result.Result.Logs[0].Contents)
	if got != "builder failed" {
		t.Fatalf("incorrect results got %s want %s", got, "builder failed")
	}
}
