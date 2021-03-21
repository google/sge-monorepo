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
	"errors"
	"testing"

	"sge-monorepo/build/cicd/presubmit/check/checkmock"
	"sge-monorepo/build/cicd/presubmit/check/protos/checkpb"
	"sge-monorepo/libs/go/sgetest"
)

func TestGoFmt(t *testing.T) {
	testCases := []struct {
		desc    string
		input   string
		wantErr string
	}{
		{
			desc:    "correctly formatted",
			input:   "testdata/good.go.testinput",
			wantErr: "",
		},
		{
			desc:    "incorrectly formatted",
			input:   "testdata/bad.go.testinput",
			wantErr: "unexpected diff",
		},
		{
			desc:    "bad syntax",
			input:   "testdata/badsyntax.go.testinput",
			wantErr: "expected declaration",
		},
	}
	for _, tc := range testCases {
		helper := checkmock.NewHelper(&checkpb.CheckerInvocation{
			TriggeredChecks: []*checkpb.TriggeredCheck{
				{
					Files: []*checkpb.File{
						{
							Path: tc.input,
						},
					},
				},
			},
		})
        // TODO: This will not work back in a bazel environment!
		_, err := checkfmt(helper, "gofmt", "testdata/bin/windows/gofmt.exe", nil)
		if err != nil {
			t.Fatalf("gofmt failed: %v", err)
		}
		var errFromLogs error
		if !helper.Result.Results[0].Success {
			errFromLogs = errors.New(string(helper.Result.Results[0].Logs[0].Contents))
		}
		if err := sgetest.CmpErr(errFromLogs, tc.wantErr); err != nil {
			t.Errorf("[%s]: %v", tc.desc, err)
		}
	}
}
