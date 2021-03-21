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

package build

import (
	"fmt"
	"sort"
	"strings"

	"sge-monorepo/build/cicd/bep"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"

	bepb "bazel.io/src/main/java/com/google/devtools/build/lib/buildeventstream/proto"
)

// buildInvocationResult parses the BEP stream and returns a BuildInvocationResult.
func buildInvocationResult(s *bep.Stream, label string) (*buildpb.BuildInvocationResult, error) {
	result := &buildpb.Result{
		Name: label,
	}
	var artifactSet *buildpb.ArtifactSet
	for _, be := range s.Events {
		switch id := be.Id.Id.(type) {
		case *bepb.BuildEventId_TargetCompleted:
			if id.TargetCompleted.Label != label {
				continue
			}
			tce, ok := be.Payload.(*bepb.BuildEvent_Completed)
			success := ok && tce.Completed.Success
			if success {
				tc := tce.Completed
				result.Success = true
				artifacts := map[string]*buildpb.Artifact{}
				for _, outputGroup := range tc.OutputGroup {
					// We only care about the default output group.
					// We do not issue build commands with output groups other than the default one.
					if outputGroup.Name == "default" {
						artifacts = map[string]*buildpb.Artifact{}
						for n, f := range s.Depsets.Files(outputGroup.FileSets) {
							artifacts[n] = fileToArtifact(f)
						}
					}
				}
				var sortedArtifacts []*buildpb.Artifact
				for _, f := range artifacts {
					sortedArtifacts = append(sortedArtifacts, f)
				}
				sort.Slice(sortedArtifacts, func(i, j int) bool {
					return sortedArtifacts[i].StablePath < sortedArtifacts[j].StablePath
				})
				artifactSet = &buildpb.ArtifactSet{
					Artifacts: sortedArtifacts,
				}
			} else {
				logs, err := bepFailureCause(s, be)
				if err != nil {
					return nil, err
				}
				result.Logs = logs
			}
		case *bepb.BuildEventId_Pattern:
			if r := maybePatternFailedToLoadResult(id, be); r != nil {
				result = r
			}
		}
	}
	return &buildpb.BuildInvocationResult{
		Result:      result,
		ArtifactSet: artifactSet,
	}, nil
}

// testInvocationResult parses the BEP stream and returns a TestInvocationResult.
func testInvocationResult(s *bep.Stream) (*buildpb.TestInvocationResult, error) {
	result := &buildpb.TestInvocationResult{}
	// Locate all tests
	tests := map[string]bool{}
	for _, be := range s.Events {
		tcid, ok := be.Id.Id.(*bepb.BuildEventId_TargetConfigured)
		if !ok {
			continue
		}
		tce, ok := be.Payload.(*bepb.BuildEvent_Configured)
		if !ok {
			continue
		}
		// This is the best way I can think of to locate only test rules.
		// Example: "go_test rule"
		if strings.HasSuffix(tce.Configured.TargetKind, "_test rule") {
			tests[tcid.TargetConfigured.Label] = true
		}
	}
	for _, be := range s.Events {
		switch id := be.Id.Id.(type) {
		case *bepb.BuildEventId_TestResult:
			tre, ok := be.Payload.(*bepb.BuildEvent_TestResult)
			if !ok {
				// Aborted.
				continue
			}
			success := tre.TestResult.Status == bepb.TestStatus_PASSED
			r := &buildpb.Result{
				Name:    id.TestResult.Label,
				Success: success,
			}
			if !success {
				var logs []*buildpb.Artifact
				for _, log := range tre.TestResult.TestActionOutput {
					// We only want the test log, not the test XML output.
					if log.Name == "test.log" {
						logs = append(logs, fileToArtifact(log))
					}
				}
				r.Logs = logs
			}
			result.Results = append(result.Results, r)
		case *bepb.BuildEventId_TargetCompleted:
			// If this isn't a test, but rather some dependent target,
			// we do not want it in the output.
			if _, ok := tests[id.TargetCompleted.Label]; !ok {
				continue
			}
			tce, ok := be.Payload.(*bepb.BuildEvent_Completed)
			if ok {
				if tce.Completed.Success {
					continue
				}
			}
			logs, err := bepFailureCause(s, be)
			if err != nil {
				return nil, err
			}
			r := &buildpb.Result{
				Name:    id.TargetCompleted.Label,
				Success: false,
				Logs:    logs,
			}
			result.Results = append(result.Results, r)
		case *bepb.BuildEventId_Pattern:
			if r := maybePatternFailedToLoadResult(id, be); r != nil {
				result.Results = append(result.Results, r)
			}
		}
	}
	return result, nil
}

// bepFailureCause attempts to find the underlying cause for a given failing BEP event.
// It searches the children of the BEP event for an action completed event
// and returns its logs.
func bepFailureCause(s *bep.Stream, be *bepb.BuildEvent) ([]*buildpb.Artifact, error) {
	var ret []*buildpb.Artifact
	for _, childId := range be.Children {
		switch id := childId.Id.(type) {
		case *bepb.BuildEventId_ActionCompleted:
			key, err := bep.EventKey(childId)
			if err != nil {
				return nil, err
			}
			childEvent, ok := s.Events[key]
			if !ok {
				continue
			}
			ace, ok := childEvent.Payload.(*bepb.BuildEvent_Action)
			if !ok {
				continue
			}
			if ace.Action.Stdout != nil {
				ret = append(ret, fileToArtifact(ace.Action.Stdout))
			}
			if ace.Action.Stderr != nil {
				ret = append(ret, fileToArtifact(ace.Action.Stderr))
			}
			if ace.Action.FailureDetail != nil {
				msg := fmt.Sprintf("%s failed to build: %s", id.ActionCompleted.Label, ace.Action.FailureDetail.Message)
				ret = append(ret, &buildpb.Artifact{
					Tag:      "failure_details",
					Contents: []byte(msg),
				})
			}
		case *bepb.BuildEventId_ConfiguredLabel:
			key, err := bep.EventKey(childId)
			if err != nil {
				return nil, err
			}
			childEvent, ok := s.Events[key]
			if !ok {
				continue
			}
			abe, ok := childEvent.Payload.(*bepb.BuildEvent_Aborted)
			if !ok {
				continue
			}
			if abe.Aborted.Description != "" {
				ret = append(ret, abortedLogs(abe))
			}
		}
	}
	return ret, nil
}

func abortedLogs(abe *bepb.BuildEvent_Aborted) *buildpb.Artifact {
	return &buildpb.Artifact{
		Tag:      "aborted",
		Contents: []byte(abe.Aborted.Description),
	}
}

func maybePatternFailedToLoadResult(id *bepb.BuildEventId_Pattern, be *bepb.BuildEvent) *buildpb.Result {
	abe, ok := be.Payload.(*bepb.BuildEvent_Aborted)
	if !ok {
		return nil
	}
	if abe.Aborted.Description == "" {
		return nil
	}
	return &buildpb.Result{
		Name:    strings.Join(id.Pattern.Pattern, " "),
		Success: false,
		Logs:    []*buildpb.Artifact{abortedLogs(abe)},
	}
}

func fileToArtifact(f *bepb.File) *buildpb.Artifact {
	if uri, ok := f.File.(*bepb.File_Uri); ok {
		return &buildpb.Artifact{
			StablePath: f.Name,
			Uri:        uri.Uri,
		}
	} else if contents, ok := f.File.(*bepb.File_Contents); ok {
		return &buildpb.Artifact{
			StablePath: f.Name,
			Contents:   contents.Contents,
		}
	}
	return nil
}
