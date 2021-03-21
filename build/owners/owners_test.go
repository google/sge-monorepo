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

package owners

import (
	"io/ioutil"
	"os"
	"path"
	"testing"

	"sge-monorepo/build/cicd/monorepo"
)

func TestOwners(t *testing.T) {
	testCases := []struct {
		desc            string
		ownerFiles      map[string]string
		files           []string
		lgtms           []string
		wantHasCoverage bool
	}{
		{
			desc:            "No files in CL needs no owners",
			files:           nil,
			lgtms:           nil,
			wantHasCoverage: true,
		},
		{
			desc: "No lgtm, no coverage",
			ownerFiles: map[string]string{
				"OWNERS":     "root@foo.com",
				"foo/OWNERS": "foo@foo.com",
			},
			files:           []string{"foo/foo.txt"},
			lgtms:           nil,
			wantHasCoverage: false,
		},
		{
			desc: "direct owner coverage",
			ownerFiles: map[string]string{
				"OWNERS":     "root@foo.com",
				"foo/OWNERS": "foo@foo.com",
			},
			files:           []string{"foo/foo.txt"},
			lgtms:           []string{"foo@foo.com"},
			wantHasCoverage: true,
		},
		{
			desc: "indirect owner coverage",
			ownerFiles: map[string]string{
				"OWNERS":     "root@foo.com",
				"foo/OWNERS": "foo@foo.com",
			},
			files:           []string{"foo/foo.txt"},
			lgtms:           []string{"root@foo.com"},
			wantHasCoverage: true,
		},
		{
			desc: "direct owner coverage of two files",
			ownerFiles: map[string]string{
				"OWNERS":     "root@foo.com",
				"bar/OWNERS": "bar@foo.com",
				"foo/OWNERS": "foo@foo.com",
			},
			files: []string{
				"bar/bar.txt",
				"foo/foo.txt",
			},
			lgtms: []string{
				"bar@foo.com",
				"foo@foo.com",
			},
			wantHasCoverage: true,
		},
		{
			desc: "one owner missing",
			ownerFiles: map[string]string{
				"OWNERS":     "root@foo.com",
				"bar/OWNERS": "bar@foo.com",
				"foo/OWNERS": "foo@foo.com",
			},
			files: []string{
				"bar/bar.txt",
				"foo/foo.txt",
			},
			lgtms: []string{
				"bar@foo.com",
			},
			wantHasCoverage: false,
		},
		{
			desc: "multiple files covered by root",
			ownerFiles: map[string]string{
				"OWNERS":     "root@foo.com",
				"bar/OWNERS": "bar@foo.com",
				"foo/OWNERS": "foo@foo.com",
			},
			files: []string{
				"bar/bar.txt",
				"foo/foo.txt",
			},
			lgtms: []string{
				"root@foo.com",
			},
			wantHasCoverage: true,
		},
	}
	for _, tcl := range testCases {
		tc := tcl
		t.Run(tc.desc, func(t *testing.T) {
			root, err := ioutil.TempDir("", "")
			defer os.RemoveAll(root)
			if err != nil {
				t.Fatal(err)
			}
			mr := monorepo.New(root, map[string]monorepo.Path{})
			for of, contents := range tc.ownerFiles {
				f := mr.ResolvePath(monorepo.NewPath(of))
				if err := os.MkdirAll(path.Dir(f), 0666); err != nil {
					t.Fatal(err)
				}
				if err := ioutil.WriteFile(f, []byte(contents), 0666); err != nil {
					t.Fatal(err)
				}
			}
			reviewers := Set{}
			for _, lgtm := range tc.lgtms {
				reviewers.Add(lgtm)
			}
			var paths []monorepo.Path
			for _, f := range tc.files {
				paths = append(paths, monorepo.NewPath(f))
			}
			got, err := HasCoverage(reviewers, mr, paths)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.wantHasCoverage {
				t.Errorf("want %v got %v", tc.wantHasCoverage, got)
			}
		})
	}
}
