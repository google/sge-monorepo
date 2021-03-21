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

// Package check contains utilities for checker tools.
// When you link against this library two flags are registered with the flag library,
// --checker-invocation and --checker-invocation-result.
// These point to protos that communicate presubmit results between sgep and the checker tool.
// Use `check.MustLoad` to obtain a helper object that assists in loading the invocation input
// and writing the checker result.
package check

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/presubmit/check/protos/checkpb"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"

	"github.com/golang/protobuf/proto"
	_ "github.com/golang/glog" // Ensure we the binary registers the glog flags
)

var _ Helper = (*helper)(nil)
var checkerInvocationFlag = flag.String("checker-invocation", "", "path to checker invocation proto")
var checkerInvocationResultFlag = flag.String("checker-invocation-result", "", "path to checker result proto")

// Helper assists the between the presubmit system and the checker tool.
type Helper interface {
	// OnlyCheck returns the single triggered check.
	// Exits if there is not exactly one triggered check.
	// You can safely use this unless your checker tool uses batch mode.
	// In such case the checks can be obtained via Invocation().TriggeredChecks.
	OnlyCheck() *checkpb.TriggeredCheck

	// Invocation returns the loaded invocation proto.
	Invocation() *checkpb.CheckerInvocation

	// AddResult adds a check result for the check with the corresponding id.
	// If your tool only supports one check, MustWriteOnlyResult is more convenient.
	AddResult(result *buildpb.Result)

	// MustWriteResult writes the results to --checker-invocation-result.
	// Exits with an error on failure.
	MustWriteResult()

	// ResolveCheckPath resolves a path into a monorepo-relative one.
	// Relative paths are resolved relative to the check referred to by checkIdx.
	// Can be used to resolve paths in checker arguments. Since these are opaque
	// strings, the presubmit system is unable to resolve them for you.
	// Input files have already been resolved by the presubmit system.
	ResolveCheckPath(checkIdx int, path string) (string, error)

	// ResolvePath resolves path into a monorepo-relative one.
	// This is useful for resolving checker tool binaries.
	// Relative paths are resolved relative to the monorepo root.
	ResolvePath(path string) (string, error)

	// RelPath makes a monorepo path from an absolute path.
	RelPath(path string) (string, error)

	// Returns a map of log labels from the invocation.
	LogLabels() map[string]string
}

type helper struct {
	monorepo   monorepo.Monorepo
	invocation *checkpb.CheckerInvocation
	result     *checkpb.CheckerInvocationResult
}

// MustLoad loads a checker invocation.
// You must call flag.Parse prior to calling this function
// Exits with an error on failure.
func MustLoad() Helper {
	if *checkerInvocationFlag == "" || *checkerInvocationResultFlag == "" {
		fmt.Println("--checker-invocation or --checker-invocation-result. Did you forget to call flag.Parse?")
		os.Exit(1)
	}
	bytes, err := ioutil.ReadFile(*checkerInvocationFlag)
	if err != nil {
		fmt.Printf("failed to load invocation %s: %v\n", *checkerInvocationFlag, err)
		os.Exit(1)
	}
	h := helper{}
	h.invocation = &checkpb.CheckerInvocation{}
	if err := proto.Unmarshal(bytes, h.invocation); err != nil {
		fmt.Printf("failed to unmarshal invocation proto %s, %v\n", *checkerInvocationFlag, err)
		os.Exit(1)
	}
	h.result = &checkpb.CheckerInvocationResult{}
	h.monorepo, _, err = monorepo.NewFromPwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return &h
}

func (h *helper) Invocation() *checkpb.CheckerInvocation {
	return h.invocation
}

func (h *helper) OnlyCheck() *checkpb.TriggeredCheck {
	if h.invocation == nil {
		fmt.Println("must call MustLoad before calling OnlyTriggeredCheck")
		os.Exit(1)
	}
	if len(h.invocation.TriggeredChecks) != 1 {
		fmt.Printf("expected 1 check, found %d checks\n", len(h.invocation.TriggeredChecks))
		os.Exit(1)
	}
	return h.invocation.TriggeredChecks[0]
}

func (h *helper) AddResult(result *buildpb.Result) {
	h.result.Results = append(h.result.Results, result)
}

func (h *helper) MustWriteResult() {
	bytes, err := proto.Marshal(h.result)
	if err != nil {
		fmt.Printf("failed to marshal result proto: %v", err)
		os.Exit(1)
	}
	if err := ioutil.WriteFile(*checkerInvocationResultFlag, bytes, 0666); err != nil {
		fmt.Printf("failed to write result proto: %v", err)
		os.Exit(1)
	}
}

func (h *helper) ResolveCheckPath(checkIdx int, p string) (string, error) {
	return h.resolvePath(monorepo.Path(h.invocation.TriggeredChecks[checkIdx].Dir), p)
}

func (h *helper) ResolvePath(p string) (string, error) {
	return h.resolvePath("", p)
}

func (h *helper) resolvePath(relTo monorepo.Path, p string) (string, error) {
	mrp, err := h.monorepo.NewPath(relTo, p)
	if err != nil {
		return "", fmt.Errorf("could not resolve path %s", p)
	}
	return string(mrp), nil
}

func (h *helper) RelPath(p string) (string, error) {
	mrp, err := (h.monorepo.RelPath(p))
	if err != nil {
		return "", err
	}
	return string(mrp), nil
}

func (h *helper) LogLabels() map[string]string {
	labels := map[string]string{}
	for _, l := range h.invocation.LogLabels {
		labels[l.Key] = l.Value
	}
	return labels
}

// LogsFromString returns checker logs with inlined contents that match the string.
func LogsFromString(tag, logs string) []*buildpb.Artifact {
	return []*buildpb.Artifact{
		{
			Tag:      tag,
			Contents: []byte(logs),
		},
	}
}
