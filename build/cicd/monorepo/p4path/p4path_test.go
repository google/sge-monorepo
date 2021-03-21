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

package p4path

import (
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strings"
	"testing"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/libs/go/sgetest"

	"github.com/google/go-cmp/cmp"
)

func TestExpr(t *testing.T) {
	testCases := []struct {
		input   string
		relTo   string
		repos   map[string]monorepo.Path
		want    string
		wantErr string
	}{
		{
			input: "...",
			want:  "...",
		},
		{
			input:   "//[",
			wantErr: "syntax error",
		},
		{
			input: "*.rs",
			relTo: "foo",
			want:  "foo/*.rs",
		},
		{
			input: "//...",
			relTo: "foo",
			want:  "...",
		},
		{
			input: "@shared//foo/bar/[0-9]*",
			repos: repoMap(),
			want:  "foo/bar/[0-9]*",
		},
		{
			input: "@shared//foo/bar/*?golang[a-zA-Z].go",
			relTo: "baz",
			repos: repoMap(),
			want:  "foo/bar/*?golang[a-zA-Z].go",
		},
		{
			input: "@shared//foo/bar",
			repos: repoMap("shared:shared"),
			want:  "shared/foo/bar",
		},
		{
			input: "//foo/bar/...uasset",
			relTo: "shared",
			repos: repoMap("shared:shared"),
			want:  "shared/foo/bar/...uasset",
		},
	}
	for _, tc := range testCases {
		repoMap := tc.repos
		if len(repoMap) == 0 {
			repoMap = noRepoMap()
		}
		mr := monorepo.New("", repoMap)
		got, gotErr := NewExpr(mr, monorepo.NewPath(tc.relTo), tc.input)

		if err := sgetest.CmpErr(gotErr, tc.wantErr); err != nil {
			t.Errorf("NewPathExpr(%q, %q): %s", tc.relTo, tc.input, err)
			continue
		}

		if tc.want != "" && string(got) != tc.want {
			t.Errorf("NewPathExpr(%q, %q)=%q, want %q", tc.relTo, tc.input, got, tc.want)
		}
	}
}

func TestExprMatches(t *testing.T) {
	testCases := []struct {
		expr      string // Gets built into a monorepo.Path
		path      string // Gets built into a monorepo.Path
		wantMatch bool
		wantErr   string
	}{
		{expr: "...", path: "file.rs", wantMatch: true},
		{expr: "...", path: "path/file.txt", wantMatch: true},
		{expr: "...", path: "path/within/F1l3.uasset", wantMatch: true},
		{expr: "path/...", path: "file.rs", wantMatch: false},
		{expr: "path/...", path: "path/file.txt", wantMatch: true},
		{expr: "path/...", path: "path/within/F1l3.uasset", wantMatch: true},
		{expr: "path/within/...", path: "file.rs", wantMatch: false},
		{expr: "path/within/...", path: "path/file.txt", wantMatch: false},
		{expr: "path/within/...", path: "path/within/F1l3.uasset", wantMatch: true},

		// Only within path.
		{expr: "path/*", path: "file.rs", wantMatch: false},
		{expr: "path/*", path: "path/file.rs", wantMatch: true},
		{expr: "path/*", path: "metadata/file.go", wantMatch: false},
		{expr: "path/*.go", path: "file.go", wantMatch: false},
		{expr: "path/*.go", path: "path/file.rs", wantMatch: false},
		{expr: "path/*.go", path: "path/file.go", wantMatch: true},
		{expr: "path/*.go", path: "metadata/file.go", wantMatch: false},

		// Base glob only base directory.
		{expr: "*", path: "file.txt", wantMatch: true},
		{expr: "*", path: "file.go", wantMatch: true},
		{expr: "*", path: "path/file.go", wantMatch: false},

		// Globs match per directory.
		{expr: "path/*/file.go", path: "file.go", wantMatch: false},
		{expr: "path/*/file.go", path: "path/file.rs", wantMatch: false},
		{expr: "path/*/file.go", path: "path/file.go", wantMatch: false},
		{expr: "path/*/file.go", path: "path/other/file.go", wantMatch: true},
		{expr: "path/*/file.go", path: "path/other/file.rs", wantMatch: false},
		{expr: "other/*/file.go", path: "path/other/file.go", wantMatch: false},
		{expr: "path/*/file.go", path: "metadata/file.go", wantMatch: false},
		{expr: "path/*/file.go", path: "path/bar/baz/file.go", wantMatch: false},

		// Multiple globs.
		{expr: "path/*/*/file.go", path: "path/baz/file.go", wantMatch: false},
		{expr: "path/*/*/file.go", path: "path/bar/baz/file.go", wantMatch: true},
		{expr: "path/*/*/file.go", path: "path/foo/bar/baz/file.go", wantMatch: false},

		// Glob beyond directory boundary.
		{expr: "path/**/file.go", path: "path/file.go", wantMatch: false},
		{expr: "path/**/file.go", path: "path/baz/file.go", wantMatch: true},
		{expr: "path/**/file.go", path: "path/bar/baz/file.go", wantMatch: true},

		// Wrong matchers
		{expr: "path/[*", path: "path/[[", wantErr: "syntax error"},

		// ... exprs.
		{expr: "path/....rs", path: "file.rs", wantMatch: false},
		{expr: "path/....rs", path: "path/file.rs", wantMatch: true},
		{expr: "path/....rs", path: "path/file.go", wantMatch: false},
		{expr: "path/....rs", path: "path/within/file.rs", wantMatch: true},
	}

	for _, tc := range testCases {
		expr := Expr(tc.expr)
		path := monorepo.NewPath(tc.path)
		gotMatch, gotErr := expr.Matches(path)
		if err := sgetest.CmpErr(gotErr, tc.wantErr); err != nil {
			t.Errorf("case %v: %q", tc, err)
			continue
		}

		if gotMatch != tc.wantMatch {
			t.Errorf("case %v, want: %t, got: %t", tc, tc.wantMatch, gotMatch)
		}
	}
}

func TestExprSet(t *testing.T) {
	testCases := []struct {
		input     []string
		relTo     string
		path      string
		wantMatch bool
	}{
		{
			input:     nil,
			path:      "foo.txt",
			wantMatch: false,
		},
		{
			input:     []string{"//..."},
			path:      "foo.txt",
			wantMatch: true,
		},
		{
			input: []string{
				"//...",
				"-//...",
			},
			path:      "foo.txt",
			wantMatch: false,
		},
		{
			input: []string{
				"//...",
				"-//...",
				"//...",
			},
			path:      "foo.txt",
			wantMatch: true,
		},
		{
			input: []string{
				"//...",
				"-//foo/...",
				"//foo/bar/...",
			},
			path:      "baz",
			wantMatch: true,
		},
		{
			input: []string{
				"//...",
				"-//foo/...",
				"//foo/bar/...",
			},
			path:      "foo/baz",
			wantMatch: false,
		},
		{
			input: []string{
				"//...",
				"-//foo/...",
				"//foo/bar/...",
			},
			path:      "foo/bar/baz",
			wantMatch: true,
		},
	}
	for _, tc := range testCases {
		mr := monorepo.New("", noRepoMap())
		set, err := NewExprSet(mr, monorepo.NewPath(tc.relTo), tc.input)
		if err != nil {
			t.Error(err)
			continue
		}
		gotMatch, err := set.Matches(monorepo.NewPath(tc.path))
		if err != nil {
			t.Error(err)
			continue
		}
		if tc.wantMatch != gotMatch {
			t.Errorf("Matches(%v, %q)=%v, want %v", tc.input, tc.path, gotMatch, tc.wantMatch)
		}
	}
}

func TestFindFiles(t *testing.T) {
	testCases := []struct {
		desc  string
		expr  string
		files []string
		want  []string
	}{
		{
			desc: "single file",
			expr: "//foo/bar.txt",
			files: []string{
				"foo/bar.txt",
			},
			want: []string{
				"foo/bar.txt",
			},
		},
		{
			desc: "* wildcard",
			expr: "//foo/*.txt",
			files: []string{
				"foo/1.txt",
				"foo/2.txt",
				"foo/1.wav",
				"foo/bar/1.txt",
			},
			want: []string{
				"foo/1.txt",
				"foo/2.txt",
			},
		},
		{
			desc: "... wildcard",
			expr: "//foo/...",
			files: []string{
				"foo/1.txt",
				"foo/2.txt",
				"foo/bar/3.txt",
				"bar/donotfindme.txt",
			},
			want: []string{
				"foo/1.txt",
				"foo/2.txt",
				"foo/bar/3.txt",
			},
		},
	}
	for _, ltc := range testCases {
		tc := ltc
		t.Run(tc.desc, func(t *testing.T) {
			root, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			mr := monorepo.New(root, map[string]monorepo.Path{})
			expr, err := NewExpr(mr, "", tc.expr)
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(root)
			for _, p := range tc.files {
				fp := mr.ResolvePath(monorepo.Path(p))
				if err := os.MkdirAll(path.Dir(fp), 066); err != nil {
					t.Fatal(err)
				}
				if err := ioutil.WriteFile(fp, nil, 066); err != nil {
					t.Fatal(err)
				}
			}
			found, err := expr.FindFiles(mr)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, f := range found {
				got = append(got, string(f))
			}
			sort.Strings(got)
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("unexpected files. diff (-want +got) %s", diff)
			}
		})
	}
}

func repoMap(repos ...string) map[string]monorepo.Path {
	res := map[string]monorepo.Path{}
	for _, mapping := range repos {
		idx := strings.Index(mapping, ":")
		if idx == -1 {
			continue
		}
		res[mapping[:idx]] = monorepo.NewPath(mapping[idx+1:])
	}
	return res
}

func noRepoMap() map[string]monorepo.Path {
	return map[string]monorepo.Path{"shared": ""}
}
