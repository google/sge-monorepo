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
	"strings"
	"testing"

	"sge-monorepo/libs/go/sgetest"

	"sge-monorepo/build/cicd/presubmit/check/checkmock"
	"sge-monorepo/build/cicd/presubmit/check/protos/checkpb"
)

func TestCheckDesc(t *testing.T) {
	testCases := []struct {
		desc             string
		clDesc           string
		wantErr          string
		wantErrLog       string
		requiredRegex    string
		prohibitedRegex  string
		suppressionRegex string
	}{
		{
			desc:    "empty case want error",
			clDesc:  "",
			wantErr: "empty CL description",
		},
		{
			desc: "has TESTED= tag",
			clDesc: `This CL is very good.
TESTED=yes very much
`,
			requiredRegex: `^TESTED=.+$`,
		},
		{
			desc:          "wants TESTED= tag",
			clDesc:        `This CL is very good`,
			requiredRegex: `^TESTED=.+$`,
			wantErrLog:    "required regex",
		},
		{
			desc:             "wants TESTED= tag but SUPPRESSED",
			clDesc:           `This CL is very good but SUPPRESSED`,
			requiredRegex:    `^TESTED=.+$`,
			suppressionRegex: `SUPPRESSED`,
		},
		{
			desc:             "wants TESTED= tag but SUPPRESSED",
			clDesc:           `This CL is very good but not suppressed`,
			requiredRegex:    `^TESTED=.+$`,
			suppressionRegex: `NOTSUPPRESSED`,
			wantErrLog:       "required regex",
		},
		{
			desc:            "no FORBIDDEN tag",
			clDesc:          `This CL is very permitted`,
			prohibitedRegex: `FORBIDDEN`,
		},
		{
			desc:            "has FORBIDDEN tag",
			clDesc:          `This CL is very FORBIDDEN`,
			prohibitedRegex: `FORBIDDEN`,
			wantErrLog:      "prohibited regex",
		},
		{
			desc:             "has FORBIDDEN tag but SUPPRESSED",
			clDesc:           `This CL is very FORBIDDEN but SUPPRESSED`,
			prohibitedRegex:  `FORBIDDEN`,
			suppressionRegex: `SUPPRESSED`,
		},
		{
			desc:             "has FORBIDDEN tag but not SUPPRESSED",
			clDesc:           `This CL is very FORBIDDEN but not suppressed`,
			prohibitedRegex:  `FORBIDDEN`,
			suppressionRegex: `NOTSUPPRESSED`,
			wantErrLog:       "prohibited regex",
		},
	}
	for _, tcl := range testCases {
		tc := tcl
		t.Run(tc.desc, func(t *testing.T) {
			helper := checkmock.NewHelper(&checkpb.CheckerInvocation{
				ClDescription: tc.clDesc,
			})
			flags.requiredRegexp = tc.requiredRegex
			flags.prohibitedRegexp = tc.prohibitedRegex
			flags.suppressionRegexp = tc.suppressionRegex
			success, err := checkdesc(helper)
			if err := sgetest.CmpErr(err, tc.wantErr); err != nil {
				t.Fatal(err)
			}
			if err != nil {
				return
			}
			if success {
				if tc.wantErrLog != "" {
					t.Fatalf("want failure with log %q, got success", tc.wantErrLog)
				}
			} else {
				results := helper.Result.Results
				if len(results) != 1 {
					t.Fatalf("want 1 result, got %d", len(results))
				}
				result := results[0]
				if len(result.Logs) != 1 {
					t.Fatalf("want 1 log, got %d", len(result.Logs))
				}
				logs := result.Logs[0]
				msg := string(logs.Contents)
				if tc.wantErrLog == "" {
					t.Fatalf("want success, got failure with logs %q", msg)
				}
				if !strings.Contains(msg, tc.wantErrLog) {
					t.Fatalf("want %q in logs, got %q", tc.wantErrLog, msg)
				}
			}
		})
	}
}
