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

	"github.com/google/go-cmp/cmp"
	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/libs/go/sgetest"
)

func TestValidatePathExprLines(t *testing.T) {
	testCases := []struct {
		desc    string
		expr    string
		wantErr string
	}{
		{
			desc: "simple valid case without wildcards",
			expr: "//foo bar",
		},
		{
			desc: "valid case with wildcards",
			expr: "foo/... bar/...",
		},
		{
			desc: "valid case with wildcards, not at the end",
			expr: "foo/....txt bar/...",
		},
		{
			desc:    "cannot use * wildcards",
			expr:    "foo/*.txt bar/*.txt",
			wantErr: "* wildcards",
		},
		{
			desc:    "must have same amount of wildcards",
			expr:    "foo/... bar",
			wantErr: "either one or zero wildcards",
		},
		{
			desc:    "must have same amount of wildcards (rhs)",
			expr:    "foo bar/...",
			wantErr: "either one or zero wildcards",
		},
		{
			desc:    "max one wildcard",
			expr:    "foo/.../... bar/.../...",
			wantErr: "either one or zero wildcards",
		},
		{
			desc:    "wildcards at end only",
			expr:    "foo/....txt bar/....txt",
			wantErr: "must be at the end",
		},
	}
	for _, tc := range testCases {
		gotErr := validatePathExprLine(tc.expr)
		if err := sgetest.CmpErr(gotErr, tc.wantErr); err != nil {
			t.Errorf("[%s]: %v", tc.desc, err)
		}
	}
}

func TestPathExprReplacer(t *testing.T) {
	testCases := []struct {
		desc   string
		exprs  []string
		inputs []string
		want   []string
	}{
		{
			desc: "simple explicit map",
			exprs: []string{
				"foo/foo.txt bar/bar.txt",
			},
			inputs: []string{
				"foo/foo.txt",
			},
			want: []string{
				"bar/bar.txt",
			},
		},
		{
			desc: "simple wildcard",
			exprs: []string{
				"foo/... bar/...",
			},
			inputs: []string{
				"foo/foo1.txt",
				"foo/foo2.txt",
			},
			want: []string{
				"bar/foo1.txt",
				"bar/foo2.txt",
			},
		},
		{
			desc: "filtering wildcard",
			exprs: []string{
				"foo/....txt bar/...",
			},
			inputs: []string{
				"foo/foo.txt",
				"foo/foo.wav",
			},
			want: []string{
				"bar/foo.txt",
			},
		},
		{
			desc: "filtering wildcard with different length",
			exprs: []string{
				"foo/....txt bar/baz/...",
			},
			inputs: []string{
				"foo/foo.txt",
				"foo/foo.wav",
			},
			want: []string{
				"bar/baz/foo.txt",
			},
		},
	}
	for _, ltc := range testCases {
		tc := ltc
		t.Run(tc.desc, func(t *testing.T) {
			mr := monorepo.New("", map[string]monorepo.Path{})
			per, err := makePathExprReplacer(mr, "", tc.exprs)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, input := range tc.inputs {
				pkgPath, ok, err := per.packagePathForInput(monorepo.Path(input))
				if err != nil {
					t.Fatal(err)
				}
				if ok {
					got = append(got, pkgPath)
				}
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("incorrect package paths. Diff (-want +got) %s", diff)
			}
		})
	}
}
