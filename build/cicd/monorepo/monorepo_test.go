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

package monorepo

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"sge-monorepo/libs/go/sgetest"
)

func TestLabel(t *testing.T) {
	testCases := []struct {
		input      string
		relTo      string
		repos      map[string]Path
		want       Label
		wantPkgDir string
	}{
		{
			input:      "//foo/bar:baz",
			want:       Label{Pkg: "foo/bar", Target: "baz"},
			wantPkgDir: "foo/bar",
		},
		{
			input:      "//foo/bar:baz",
			relTo:      "foo", // Should be ignored, we're using //...
			want:       Label{Pkg: "foo/bar", Target: "baz"},
			wantPkgDir: "foo/bar",
		},
		{
			input:      "//foo/bar",
			want:       Label{Pkg: "foo/bar", Target: "bar"},
			wantPkgDir: "foo/bar",
		},
		{
			input:      "foo/bar:baz",
			want:       Label{Pkg: "foo/bar", Target: "baz"},
			wantPkgDir: "foo/bar",
		},
		{
			input:      "bar:baz",
			relTo:      "foo",
			want:       Label{Pkg: "foo/bar", Target: "baz"},
			wantPkgDir: "foo/bar",
		},
		{
			input:      ":bar",
			relTo:      "foo",
			want:       Label{Pkg: "foo", Target: "bar"},
			wantPkgDir: "foo",
		},
		{
			input:      "//foo/bar:bar",
			relTo:      "shared",
			repos:      repoMap("shared:shared"),
			want:       Label{Repo: "shared", Pkg: "foo/bar", Target: "bar"},
			wantPkgDir: "shared/foo/bar",
		},
		{
			input:      "//foo/bar:bar",
			relTo:      "shared/foo",
			repos:      repoMap("shared:shared"),
			want:       Label{Repo: "shared", Pkg: "foo/bar", Target: "bar"},
			wantPkgDir: "shared/foo/bar",
		},
		{
			input:      "bar:bar",
			relTo:      "shared/foo",
			repos:      repoMap("shared:shared"),
			want:       Label{Repo: "shared", Pkg: "foo/bar", Target: "bar"},
			wantPkgDir: "shared/foo/bar",
		},
		{
			input:      "@shared//foo:bar",
			repos:      repoMap("shared:shared"),
			want:       Label{Repo: "shared", Pkg: "foo", Target: "bar"},
			wantPkgDir: "shared/foo",
		},
		{
			input:      "@shared//foo:bar",
			repos:      repoMap(),
			want:       Label{Pkg: "foo", Target: "bar"},
			wantPkgDir: "foo",
		},
		{
			input:      "@shared//:bar",
			repos:      repoMap(),
			want:       Label{Pkg: "", Target: "bar"},
			wantPkgDir: "",
		},
	}
	for _, tc := range testCases {
		repoMap := tc.repos
		if len(repoMap) == 0 {
			repoMap = noRepoMap()
		}
		mr := New("", repoMap)
		got, err := mr.NewLabel(NewPath(tc.relTo), tc.input)
		if err != nil {
			t.Errorf("NewLabel(%q, %q)=%v, want no error, but got %v", tc.relTo, tc.input, got, err)
			continue
		}
		if got != tc.want {
			t.Errorf("NewLabel(%q, %q)=%v, want %v", tc.relTo, tc.input, got, tc.want)
			continue
		}
		gotPkgDir, err := mr.ResolveLabelPkgDir(got)
		if err != nil {
			t.Errorf("ResolveLabelDir(%v)=%v, want no error", got, err)
		}
		if string(gotPkgDir) != tc.wantPkgDir {
			t.Errorf("ResolveLabelDir(%v)=%s, want %s", got, gotPkgDir, tc.wantPkgDir)
		}
	}
}

func TestTargetExpression(t *testing.T) {
	testCases := []struct {
		input string
		relTo string
		repos map[string]Path
		want  string
	}{
		{
			input: "//foo/bar:baz",
			want:  "//foo/bar:baz",
		},
		{
			input: "//foo/bar:baz",
			relTo: "foo", // Should be ignored, we're using //...
			want:  "//foo/bar:baz",
		},
		{
			input: "//foo/bar:baz",
			relTo: "bar", // Should be ignored, we're using //...
			want:  "//foo/bar:baz",
		},
		{
			input: "//foo/bar",
			want:  "//foo/bar:bar",
		},
		{
			input: "foo/bar:baz",
			want:  "//foo/bar:baz",
		},
		{
			input: "bar:baz",
			relTo: "foo",
			want:  "//foo/bar:baz",
		},
		{
			input: "//foo:all",
			want:  "//foo:all",
		},
		{
			input: ":all",
			relTo: "foo",
			want:  "//foo:all",
		},
		{
			input: "//foo/...",
			want:  "//foo/...",
		},
		{
			input: "bar/...",
			relTo: "foo",
			want:  "//foo/bar/...",
		},
		{
			input: "...",
			relTo: "foo",
			want:  "//foo/...",
		},
		{
			input: "//foo/bar:bar",
			relTo: "shared",
			repos: repoMap("shared:shared"),
			want:  "@shared//foo/bar:bar",
		},
		{
			input: "//foo/bar:bar",
			relTo: "shared/foo",
			repos: repoMap("shared:shared"),
			want:  "@shared//foo/bar:bar",
		},
		{
			input: "bar:bar",
			relTo: "shared/foo",
			repos: repoMap("shared:shared"),
			want:  "@shared//foo/bar:bar",
		},
		{
			input: "@shared//foo:bar",
			repos: repoMap("shared:shared"),
			want:  "@shared//foo:bar",
		},
		{
			input: "@shared//foo:bar",
			repos: repoMap(),
			want:  "//foo:bar",
		},
	}
	for _, tc := range testCases {
		repoMap := tc.repos
		if len(repoMap) == 0 {
			repoMap = noRepoMap()
		}
		mr := New("", repoMap)
		got, err := mr.NewTargetExpression(NewPath(tc.relTo), tc.input)
		if err != nil {
			t.Errorf("NewTargetExpression(%q, %q)=%v, want no error", tc.relTo, tc.input, err)
		}
		if string(got) != tc.want {
			t.Errorf("NewTargetExpression(%q, %q)=%v, want %v", tc.relTo, tc.input, got, tc.want)
		}
	}
}

func TestPath(t *testing.T) {
	testCases := []struct {
		input string
		relTo string
		repos map[string]Path
		want  string
	}{
		{
			input: "foo",
			want:  "foo",
		},
		{
			input: "bar",
			relTo: "foo",
			want:  "foo/bar",
		},
		{
			input: "//bar",
			relTo: "foo",
			want:  "bar",
		},
		{
			input: "@shared//foo/bar",
			repos: repoMap(),
			want:  "foo/bar",
		},
		{
			input: "@shared//foo/bar",
			relTo: "baz",
			repos: repoMap(),
			want:  "foo/bar",
		},
		{
			input: "@shared//foo/bar",
			repos: repoMap("shared:shared"),
			want:  "shared/foo/bar",
		},
		{
			input: "bar/baz",
			relTo: "shared/foo",
			repos: repoMap("shared:shared"),
			want:  "shared/foo/bar/baz",
		},
		{
			input: "//foo/bar",
			relTo: "shared",
			repos: repoMap("shared:shared"),
			want:  "shared/foo/bar",
		},
	}
	for _, tc := range testCases {
		repoMap := tc.repos
		if len(repoMap) == 0 {
			repoMap = noRepoMap()
		}
		mr := New("", repoMap)
		got, err := mr.NewPath(NewPath(tc.relTo), tc.input)
		if err != nil {
			t.Errorf("NewPath(%q, %q)=%v, want no error", tc.relTo, tc.input, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("NewPath(%q, %q)=%q, want %q", tc.relTo, tc.input, got, tc.want)
		}
	}
}

func repoMap(repos ...string) map[string]Path {
	res := map[string]Path{}
	for _, mapping := range repos {
		idx := strings.Index(mapping, ":")
		if idx == -1 {
			continue
		}
		res[mapping[:idx]] = NewPath(mapping[idx+1:])
	}
	return res
}

func noRepoMap() map[string]Path {
	return map[string]Path{"shared": ""}
}

func TestLoadWorkspace(t *testing.T) {
	files := map[string]string{
		"WORKSPACE": `
local_repository(
    name = "foo",
    path = "repos/foo",
)

local_repository(
    name = "bar",
    path = "repos/bar",
)

# sgeb:load more_repos.bzl
`,
		"more_repos.bzl": `
local_repository(
    name = "baz",
    path = "repos/baz",
)
`,
	}
	wsDir, err := ioutil.TempDir("", "ws")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(wsDir)
	if err := sgetest.WriteFiles(wsDir, files); err != nil {
		t.Fatal(err)
	}
	got, err := parseWorkspace(wsDir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Path{
		"foo": Path("repos/foo"),
		"bar": Path("repos/bar"),
		"baz": Path("repos/baz"),
	}
	if !cmp.Equal(got, want) {
		t.Errorf("incorrect repos found. got %v want %v", got, want)
	}
}
