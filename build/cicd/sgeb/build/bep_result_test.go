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
	"path"
	"sort"
	"testing"

	"sge-monorepo/build/cicd/bep"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"

	bepb "bazel.io/src/main/java/com/google/devtools/build/lib/buildeventstream/proto"
	"bazel.io/src/main/protobuf"
	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/encoding/protowire"
)

func TestGetBuildResults(t *testing.T) {
	testCases := []struct {
		desc   string
		events []proto.Message
		want   *buildpb.BuildInvocationResult
	}{
		{
			desc: "Success case",
			events: []proto.Message{
				namedSetOfFilesEvent("set-a", []string{"a1.txt", "a2.txt"}, []string{"set-b", "set-c"}),
				namedSetOfFilesEvent("set-b", []string{"b.txt", "bc.txt"}, []string{"set-d"}),
				namedSetOfFilesEvent("set-c", []string{"c.txt", "bc.txt"}, []string{"set-d"}),
				namedSetOfFilesEvent("set-d", []string{"d.txt"}, nil),
				namedSetOfFilesEvent("set-e", []string{"notreached.txt"}, nil),
				targetCompleteEvent("//foo:foo", "set-a"),
			},
			want: &buildpb.BuildInvocationResult{
				Result: &buildpb.Result{
					Name:    "//foo:foo",
					Success: true,
				},
				ArtifactSet: &buildpb.ArtifactSet{
					Artifacts: []*buildpb.Artifact{
						{StablePath: "a1.txt", Uri: "file:///a1.txt"},
						{StablePath: "a2.txt", Uri: "file:///a2.txt"},
						{StablePath: "b.txt", Uri: "file:///b.txt"},
						{StablePath: "bc.txt", Uri: "file:///bc.txt"},
						{StablePath: "c.txt", Uri: "file:///c.txt"},
						{StablePath: "d.txt", Uri: "file:///d.txt"},
					},
				},
			},
		},
		{
			desc: "Indirect failure case",
			events: []proto.Message{
				targetFailedEvent("//foo:foo", &bepb.BuildEventId{
					Id: actionCompletedId("//bar:bar", "bar.txt"),
				}),
				actionExecutedEvent(
					actionCompletedId("//bar:bar", "bar.txt"),
					&bepb.ActionExecuted{
						Success: false,
						Stderr:  makeFile("stderr", "bar/stderr.out"),
					}),
			},
			want: &buildpb.BuildInvocationResult{
				Result: &buildpb.Result{
					Name:    "//foo:foo",
					Success: false,
					Logs: []*buildpb.Artifact{
						convertFile(makeFile("stderr", "bar/stderr.out")),
					},
				},
			},
		},
		{
			desc: "Indirect failure case with failure detail",
			events: []proto.Message{
				targetFailedEvent("//foo:foo", &bepb.BuildEventId{
					Id: actionCompletedId("//bar:bar", "bar.txt"),
				}),
				actionExecutedEvent(
					actionCompletedId("//bar:bar", "bar.txt"),
					&bepb.ActionExecuted{
						Success: false,
						FailureDetail: &protobuf.FailureDetail{
							Message: "some details",
						},
					}),
			},
			want: &buildpb.BuildInvocationResult{
				Result: &buildpb.Result{
					Name:    "//foo:foo",
					Success: false,
					Logs: []*buildpb.Artifact{
						{
							Tag:     "failure_details",
							Contents: []byte("//bar:bar failed to build: some details"),
						},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		buf, err := protoStream(tc.events)
		if err != nil {
			t.Fatal(err)
		}
		s, err := bep.Parse(buf)
		if err != nil {
			t.Fatal(err)
		}
		got, err := buildInvocationResult(s, "//foo:foo")
		if err != nil {
			t.Fatal(err)
		}
		if !proto.Equal(got, tc.want) {
            t.Errorf("[%s] BuildInvocationResult\n got: %v\nwant: %v", tc.desc, got, tc.want)
		}
	}
}

func TestGetTestResults(t *testing.T) {
	testCases := []struct {
		desc   string
		events []proto.Message
		want   *buildpb.TestInvocationResult
	}{
		{
			desc: "Success case",
			events: []proto.Message{
				targetConfiguredEvent("//foo:foo_test", "go_test rule"),
				targetConfiguredEvent("//bar:bar", "go_test rule"),
				targetCompleteEvent("//foo:foo_test"),
				targetCompleteEvent("//foo:bar_test"),
				testResultEvent("//foo:foo_test", &bepb.TestResult{
					Status: bepb.TestStatus_PASSED,
				}),
				testResultEvent("//bar:bar_test", &bepb.TestResult{
					Status: bepb.TestStatus_PASSED,
				}),
			},
			want: &buildpb.TestInvocationResult{
				Results: []*buildpb.Result{
					{
						Name:    "//bar:bar_test",
						Success: true,
					},
					{
						Name:    "//foo:foo_test",
						Success: true,
					},
				},
			},
		},
		{
			desc: "Failure case",
			events: []proto.Message{
				targetConfiguredEvent("//foo:foo_test", "go_test rule"),
				targetCompleteEvent("//foo:foo_test"),
				testResultEvent("//foo:foo_test", &bepb.TestResult{
					Status: bepb.TestStatus_FAILED,
					TestActionOutput: []*bepb.File{
						makeFile("test.log", "foo/test.out"),
						makeFile("test.xml", "foo/test.xml"), // We do not want this file
					},
				}),
			},
			want: &buildpb.TestInvocationResult{
				Results: []*buildpb.Result{
					{
						Name:    "//foo:foo_test",
						Success: false,
						Logs: []*buildpb.Artifact{
							convertFile(makeFile("test.log", "foo/test.out")),
						},
					},
				},
			},
		},
		{
			desc: "Indirect failure case - action",
			events: []proto.Message{
				targetConfiguredEvent("//foo:foo_test", "go_test rule"),
				targetConfiguredEvent("//bar:bar", "go_library"),
				targetFailedEvent("//foo:foo_test", &bepb.BuildEventId{
					Id: actionCompletedId("//bar:bar", "bar.txt"),
				}),
				actionExecutedEvent(
					actionCompletedId("//bar:bar", "bar.txt"),
					&bepb.ActionExecuted{
						Success: false,
						Stderr:  makeFile("stderr", "bar/stderr.out"),
					}),
			},
			want: &buildpb.TestInvocationResult{
				Results: []*buildpb.Result{
					{
						Name:    "//foo:foo_test",
						Success: false,
						Logs: []*buildpb.Artifact{
							convertFile(makeFile("stderr", "bar/stderr.out")),
						},
					},
				},
			},
		},
		{
			desc: "Indirect failure case - BUILD error",
			events: []proto.Message{
				targetConfiguredEvent("//foo:foo_test", "go_test rule"),
				targetFailedEvent("//foo:foo_test", &bepb.BuildEventId{
					Id: configuredLabelId("//bar:bar"),
				}),
				&bepb.BuildEvent{
					Id: &bepb.BuildEventId{
						Id: configuredLabelId("//bar:bar"),
					},
					Payload: abortedEvent("bar failed"),
				},
			},
			want: &buildpb.TestInvocationResult{
				Results: []*buildpb.Result{
					{
						Name:    "//foo:foo_test",
						Success: false,
						Logs: []*buildpb.Artifact{
							{Tag: "aborted", Contents: []byte("bar failed")},
						},
					},
				},
			},
		},
		{
			desc: "Invalid pattern event",
			events: []proto.Message{
				&bepb.BuildEvent{
					Id: &bepb.BuildEventId{
						Id: patternEventId("//foo/..."),
					},
					Payload: abortedEvent("invalid pattern"),
				},
			},
			want: &buildpb.TestInvocationResult{
				Results: []*buildpb.Result{
					{
						Name:    "//foo/...",
						Success: false,
						Logs: []*buildpb.Artifact{
							{Tag: "aborted", Contents: []byte("invalid pattern")},
						},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		buf, err := protoStream(tc.events)
		if err != nil {
			t.Fatal(err)
		}
		s, err := bep.Parse(buf)
		if err != nil {
			t.Fatal(err)
		}
		got, err := testInvocationResult(s)
		if err != nil {
			t.Fatal(err)
		}
		sort.Slice(got.Results, func(i, j int) bool {
			return got.Results[i].Name < got.Results[j].Name
		})
		if !proto.Equal(got, tc.want) {
			t.Errorf("TestInvocationResult got %v want %v", got, tc.want)
		}
	}
}

func protoStream(messages []proto.Message) ([]byte, error) {
	var buf []byte
	for _, m := range messages {
		b, err := proto.Marshal(m)
		if err != nil {
			return nil, err
		}
		buf = protowire.AppendBytes(buf, b)
	}
	return buf, nil
}

func targetConfiguredEvent(label string, kind string) *bepb.BuildEvent {
	return &bepb.BuildEvent{
		Id: &bepb.BuildEventId{
			Id: &bepb.BuildEventId_TargetConfigured{
				TargetConfigured: &bepb.BuildEventId_TargetConfiguredId{
					Label: label,
				},
			},
		},
		Payload: &bepb.BuildEvent_Configured{
			Configured: &bepb.TargetConfigured{
				TargetKind: kind,
			},
		},
	}
}

func targetCompleteEvent(label string, depsets ...string) *bepb.BuildEvent {
	return &bepb.BuildEvent{
		Id: &bepb.BuildEventId{
			Id: &bepb.BuildEventId_TargetCompleted{
				TargetCompleted: &bepb.BuildEventId_TargetCompletedId{
					Label: label,
				},
			},
		},
		Payload: &bepb.BuildEvent_Completed{
			Completed: &bepb.TargetComplete{
				Success: true,
				OutputGroup: []*bepb.OutputGroup{
					{
						Name:     "default",
						FileSets: depsetIds(depsets),
					},
				},
			},
		},
	}
}

func targetFailedEvent(label string, cause *bepb.BuildEventId) *bepb.BuildEvent {
	return &bepb.BuildEvent{
		Id: &bepb.BuildEventId{
			Id: &bepb.BuildEventId_TargetCompleted{
				TargetCompleted: &bepb.BuildEventId_TargetCompletedId{
					Label: label,
				},
			},
		},
		Children: []*bepb.BuildEventId{cause},
		Payload: &bepb.BuildEvent_Completed{
			Completed: &bepb.TargetComplete{},
		},
	}
}

func actionCompletedId(label, primaryOutput string) *bepb.BuildEventId_ActionCompleted {
	return &bepb.BuildEventId_ActionCompleted{
		ActionCompleted: &bepb.BuildEventId_ActionCompletedId{
			PrimaryOutput: primaryOutput,
			Label:         label,
			Configuration: nil,
		},
	}
}

func actionExecutedEvent(id *bepb.BuildEventId_ActionCompleted, event *bepb.ActionExecuted) *bepb.BuildEvent {
	return &bepb.BuildEvent{
		Id: &bepb.BuildEventId{
			Id: id,
		},
		Payload: &bepb.BuildEvent_Action{
			Action: event,
		},
	}
}

func testResultEvent(label string, result *bepb.TestResult) *bepb.BuildEvent {
	return &bepb.BuildEvent{
		Id: &bepb.BuildEventId{
			Id: &bepb.BuildEventId_TestResult{
				TestResult: &bepb.BuildEventId_TestResultId{
					Label: label,
				},
			},
		},
		Payload: &bepb.BuildEvent_TestResult{
			TestResult: result,
		},
	}
}

func configuredLabelId(label string) *bepb.BuildEventId_ConfiguredLabel {
	return &bepb.BuildEventId_ConfiguredLabel{
		ConfiguredLabel: &bepb.BuildEventId_ConfiguredLabelId{
			Label: label,
		},
	}
}

func patternEventId(pattern string) *bepb.BuildEventId_Pattern {
	return &bepb.BuildEventId_Pattern{
		Pattern: &bepb.BuildEventId_PatternExpandedId{
			Pattern: []string{pattern},
		},
	}
}

func abortedEvent(description string) *bepb.BuildEvent_Aborted {
	return &bepb.BuildEvent_Aborted{
		Aborted: &bepb.Aborted{
			Description: description,
		},
	}
}

func namedSetOfFilesEvent(id string, fnames []string, deps []string) *bepb.BuildEvent {
	return &bepb.BuildEvent{
		Id: &bepb.BuildEventId{
			Id: &bepb.BuildEventId_NamedSet{
				NamedSet: &bepb.BuildEventId_NamedSetOfFilesId{
					Id: id,
				},
			},
		},
		Payload: &bepb.BuildEvent_NamedSetOfFiles{
			NamedSetOfFiles: &bepb.NamedSetOfFiles{
				Files:    makeFiles(fnames),
				FileSets: depsetIds(deps),
			},
		},
	}
}

func makeFiles(names []string) []*bepb.File {
	var fs []*bepb.File
	for _, name := range names {
		fs = append(fs, makeFile(path.Base(name), name))
	}
	return fs
}

func convertFile(file *bepb.File) *buildpb.Artifact {
    return &buildpb.Artifact{
        StablePath: file.Name,
        Uri: file.GetUri(),
    }
}

func makeFile(name, path string) *bepb.File {
	return &bepb.File{
		Name: name,
		File: &bepb.File_Uri{
			Uri: "file:///" + path,
		},
	}
}

func depsetIds(ids []string) []*bepb.BuildEventId_NamedSetOfFilesId {
	var depsetIds []*bepb.BuildEventId_NamedSetOfFilesId
	for _, id := range ids {
		depsetIds = append(depsetIds, &bepb.BuildEventId_NamedSetOfFilesId{
			Id: id,
		})
	}
	return depsetIds
}
