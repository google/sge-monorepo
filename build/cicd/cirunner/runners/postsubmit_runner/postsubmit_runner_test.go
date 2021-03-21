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
	"testing"
	"time"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/jenkins/mockjenkins"
	"sge-monorepo/libs/go/clock/mockclock"
	"sge-monorepo/libs/go/email/mockemail"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/p4lib/p4mock"

	"sge-monorepo/build/cicd/cirunner/runners/postsubmit_runner/protos/postsubmitpb"
	"sge-monorepo/build/cicd/cirunner/runners/unit_runner/protos/unit_runnerpb"
	"sge-monorepo/build/cicd/sgeb/protos/sgebpb"

	"github.com/golang/protobuf/proto"
)

// mockKeys provides mock p4 key storage
type mockKeys struct {
	keyVals map[string]string
}

func (mk *mockKeys) GetKey(key string) (string, error) {
	if val, ok := mk.keyVals[key]; ok {
		return val, nil
	}
	return "0", nil
}

func (mk *mockKeys) SetKey(key, val string) error {
	mk.keyVals[key] = val
	return nil
}

func (mk *mockKeys) setProto(key string, m proto.Message) {
	val := proto.MarshalTextString(m)
	_ = mk.SetKey(key, val)
}

func newP4Mock() (p4lib.P4, *mockKeys) {
	mk := &mockKeys{map[string]string{}}
	mock := p4mock.New()
	mock.KeyGetFunc = mk.GetKey
	mock.KeySetFunc = mk.SetKey
	return mock, mk
}

func TestPostSubmit(t *testing.T) {
	type testState struct {
		taskKey      string
		keys         *mockKeys
		clock        *mockclock.MockClock
		changedFiles []string
	}
	type step struct {
		desc           string
		action         func(*testState)
		wantStatus     postsubmitpb.PostSubmitStatus
		wantEmailCount int
		wantTaskCount  int
	}
	testCases := []struct {
		desc       string
		postSubmit *sgebpb.PostSubmit
		steps      []step
	}{
		{
			desc:       "no conditions",
			postSubmit: &sgebpb.PostSubmit{},
			steps: []step{
				{
					desc:          "there are no conditions, so no publishing should happen",
					wantStatus:    postsubmitpb.PostSubmitStatus_SUCCESS,
					wantTaskCount: 0,
				},
			},
		},
		{
			desc: "success case",
			postSubmit: &sgebpb.PostSubmit{
				Notify: &sgebpb.NotificationConfig{
					Email: []string{"foo@foo.com"},
				},
				TriggerAlwaysForTesting: true,
			},
			steps: []step{
				{
					desc:          "first step should kick off a task and set status to pending",
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
				{
					desc:          "task has not started yet (in queue), so no change to status",
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
				{
					desc: "task is now running, status should remain in pending",
					action: func(ts *testState) {
						taskState := &unit_runnerpb.Task{
							Status: unit_runnerpb.TaskStatus_RUNNING,
						}
						ts.keys.setProto(ts.taskKey, taskState)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
				{
					desc:          "task is still running, so no change to status",
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
				{
					desc: "task finished successfully, status should be success",
					action: func(ts *testState) {
						taskState := &unit_runnerpb.Task{
							Status: unit_runnerpb.TaskStatus_SUCCESS,
						}
						ts.keys.setProto(ts.taskKey, taskState)
					},
					wantStatus:     postsubmitpb.PostSubmitStatus_SUCCESS,
					wantTaskCount:  1,
					wantEmailCount: 1, // Should have sent success email
				},
			},
		},
		{
			desc: "failure case",
			postSubmit: &sgebpb.PostSubmit{
				Notify: &sgebpb.NotificationConfig{
					Email: []string{"foo@foo.com"},
				},
				TriggerAlwaysForTesting: true,
			},
			steps: []step{
				{
					desc:          "first step should kick off a task and set status to pending",
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
				{
					desc: "task failed, status should go to failed",
					action: func(ts *testState) {
						taskState := &unit_runnerpb.Task{
							Status: unit_runnerpb.TaskStatus_FAILED,
						}
						ts.keys.setProto(ts.taskKey, taskState)
					},
					wantStatus:     postsubmitpb.PostSubmitStatus_FAILED,
					wantTaskCount:  1,
					wantEmailCount: 1, // Should have sent failure email
				},
				{
					desc:           "we have no retry policy so additional steps do nothing",
					wantStatus:     postsubmitpb.PostSubmitStatus_FAILED,
					wantTaskCount:  1,
					wantEmailCount: 1, // Ensure no repeat failure emails
				},
			},
		},
		{
			desc: "custom timeout",
			postSubmit: &sgebpb.PostSubmit{
				TriggerAlwaysForTesting: true,
				TimeoutMinutes:          timeoutTaskSecondsDefault/60 + 10,
			},
			steps: []step{
				{
					desc:          "first step should kick off a task and set status to pending",
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
				{
					desc: "task is now running, no other change expected",
					action: func(ts *testState) {
						taskState := &unit_runnerpb.Task{
							Status: unit_runnerpb.TaskStatus_RUNNING,
						}
						ts.keys.setProto(ts.taskKey, taskState)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
				{
					desc: "advance until default task timeout, nothing should happen (10 mins remaining)",
					action: func(ts *testState) {
						ts.clock.Advance(timeoutTaskSecondsDefault)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
				{
					desc: "advance the extra 10 minutes, task should time out",
					action: func(ts *testState) {
						ts.clock.Advance(10 * 60)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_FAILED,
					wantTaskCount: 1,
				},
			},
		},
		{
			desc: "retry logic",
			postSubmit: &sgebpb.PostSubmit{
				TriggerAlwaysForTesting: true,
			},
			steps: []step{
				{
					desc:          "first step should kick off a task and set status to pending",
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
				{
					desc:          "not advancing time should do nothing",
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
				{
					desc: "advance until queue timeout, should immediately kick another task",
					action: func(ts *testState) {
						ts.clock.Advance(timeoutTaskQueueSeconds)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 2, // Another task was issued
				},
				{
					desc: "task is now running, no other change expected",
					action: func(ts *testState) {
						taskState := &unit_runnerpb.Task{
							Status: unit_runnerpb.TaskStatus_RUNNING,
						}
						ts.keys.setProto(ts.taskKey, taskState)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 2,
				},
				{
					desc: "advance until task timeout, should fail task",
					action: func(ts *testState) {
						ts.clock.Advance(timeoutTaskSecondsDefault)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_FAILED,
					wantTaskCount: 2,
				},
				{
					desc:          "doing nothing should keep the task in the failed state",
					wantStatus:    postsubmitpb.PostSubmitStatus_FAILED,
					wantTaskCount: 2,
				},
				{
					desc: "wait until it is time to retry, should kick off another task",
					action: func(ts *testState) {
						ts.clock.Advance(retrySeconds)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 3, // Another task was issued
				},
			},
		},
		{
			desc: "daily publish",
			postSubmit: &sgebpb.PostSubmit{
				Frequency: &sgebpb.PostSubmitFrequency{
					DailyAtUtc: "00:00",
				},
			},
			steps: []step{
				{
					desc: "set clock to noon, should not publish",
					action: func(ts *testState) {
						ts.clock.SetTime(time.Date(2000, time.January, 1, 12, 0, 0, 0, time.UTC))
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_SUCCESS,
					wantTaskCount: 0,
				},
				{
					desc: "set clock to midnight, should publish",
					action: func(ts *testState) {
						ts.clock.Advance(12 * 60 * 60)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
				{
					desc: "finish task",
					action: func(ts *testState) {
						taskState := &unit_runnerpb.Task{
							Status: unit_runnerpb.TaskStatus_SUCCESS,
						}
						ts.keys.setProto(ts.taskKey, taskState)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_SUCCESS,
					wantTaskCount: 1,
				},
				{
					desc:          "same time, already published, should not publish again",
					wantStatus:    postsubmitpb.PostSubmitStatus_SUCCESS,
					wantTaskCount: 1,
				},
				{
					desc: "set clock to next midnight, should publish",
					action: func(ts *testState) {
						ts.clock.Advance(24 * 60 * 60)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 2,
				},
			},
		},
		{
			desc: "triggered by changed files",
			postSubmit: &sgebpb.PostSubmit{
				TriggerPaths: &sgebpb.PostSubmitTriggerPathSet{
					Path: []string{
						"//triggerme/...",
					},
				},
			},
			steps: []step{
				{
					desc:          "no files changed, should not trigger",
					wantStatus:    postsubmitpb.PostSubmitStatus_SUCCESS,
					wantTaskCount: 0,
				},
				{
					desc: "wrong files changed, should not trigger",
					action: func(ts *testState) {
						ts.changedFiles = []string{
							"donottriggerme/foo.txt",
						}
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_SUCCESS,
					wantTaskCount: 0,
				},
				{
					desc: "right files changed, should trigger",
					action: func(ts *testState) {
						ts.changedFiles = []string{
							"triggerme/foo.txt",
						}
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
			},
		},
		{
			desc: "path triggers and daily builds",
			postSubmit: &sgebpb.PostSubmit{
				TriggerPaths: &sgebpb.PostSubmitTriggerPathSet{
					Path: []string{
						"//triggerme/...",
					},
				},
				Frequency: &sgebpb.PostSubmitFrequency{
					DailyAtUtc: "00:00",
				},
			},
			steps: []step{
				{
					desc: "no files changed, 12 o'clock, should not trigger",
					action: func(ts *testState) {
						ts.clock.SetTime(time.Date(2000, time.January, 1, 12, 0, 0, 0, time.UTC))
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_SUCCESS,
					wantTaskCount: 0,
				},
				{
					desc: "advance to midnight, should trigger.",
					action: func(ts *testState) {
						ts.clock.Advance(12 * 60 * 60)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 1,
				},
				{
					desc: "finish task",
					action: func(ts *testState) {
						taskState := &unit_runnerpb.Task{
							Status: unit_runnerpb.TaskStatus_SUCCESS,
						}
						ts.keys.setProto(ts.taskKey, taskState)
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_SUCCESS,
					wantTaskCount: 1,
				},
				{
					desc: "advance to 12 o'clock and trigger paths. Should trigger.",
					action: func(ts *testState) {
						ts.clock.Advance(12 * 60 * 60)
						ts.changedFiles = []string{
							"triggerme/foo.txt",
						}
					},
					wantStatus:    postsubmitpb.PostSubmitStatus_PENDING,
					wantTaskCount: 2,
				},
			},
		},
	}
	for _, tc := range testCases {
		// Setup test harness
		p4, keys := newP4Mock()
		mockEmail := mockemail.NewClient()
		clock := mockclock.New()
		r := runner{
			p4:            p4,
			jenkinsRemote: mockjenkins.NewRemote(),
			emailClient:   mockEmail,
			clock:         clock,
		}
		p := postSubmitUnit{
			label:      "//foo:foo",
			postSubmit: tc.postSubmit,
			kind:       publishKind,
		}
		key := r.stateKey(p.label)
		ts := &testState{
			keys:  keys,
			clock: clock,
		}
		steps := tc.steps
		mr := monorepo.New("", map[string]monorepo.Path{})

		// Run steps
		t.Run(tc.desc, func(t *testing.T) {
			var taskCount int
			for _, s := range steps {
				if s.action != nil {
					s.action(ts)
				}
				var changedFiles []monorepo.Path
				for _, cf := range ts.changedFiles {
					p, err := mr.NewPath("", cf)
					if err != nil {
						t.Fatal(err)
					}
					changedFiles = append(changedFiles, p)
				}
				err := r.processPostSubmitUnit(mr, p, changedFiles)
				if err != nil {
					t.Fatal(err)
				}
				state, err := readState(p4, key)
				if err != nil {
					t.Fatal(err)
				}
				var taskKey string
				if state.Task != nil {
					taskKey = state.Task.Key
				}
				if ts.taskKey != taskKey {
					ts.taskKey = taskKey
					if taskKey != "" {
						taskCount = taskCount + 1
					}
				}
				if s.wantStatus != state.Status {
					t.Fatalf("step %q: want state %v, got %v", s.desc, s.wantStatus, state.Status)
				}
				if s.wantTaskCount != taskCount {
					t.Fatalf("step %q: want %d tasks issued, got %d", s.desc, s.wantTaskCount, taskCount)
				}
				if mockEmail.SendCount() != s.wantEmailCount {
					t.Fatalf("step %q: expected %d email sent, got %d", s.desc, s.wantEmailCount, mockEmail.SendCount())
				}
			}
		})
	}
}
