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

package cicdfile

import (
	"os"
	"testing"

	"sge-monorepo/build/cicd/cicdfile/protos/cicdfilepb"
	"sge-monorepo/build/cicd/monorepo"
)

func TestSearchForCicdFiles(t *testing.T) {
	testCases := []struct {
		inputPaths []string
		want       []File
	}{
		{
			inputPaths: nil,
			want:       nil,
		},
		{
			inputPaths: []string{
				"testdata/A/AC/file.txt",
			},
			want: nil,
		},
		{
			inputPaths: []string{
				"testdata/A/AC/file.txt",
				"testdata/B/file.txt",
				"testdata/B/BA/file.txt",
				"testdata/B/BB/file.txt",
				"testdata/B/BB/BBA/file.txt",
				"testdata/B/BB/BBB/file.txt",
			},
			want: []File{
				createCicdFile("testdata/B/BB/BBA/CICD.textpb"),
				createCicdFile("testdata/B/BB/BBB/CICD"),
				createCicdFile("testdata/B/CICD"),
			},
		},
	}
	runfiles, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	mr := monorepo.Monorepo{Root: runfiles}
	for _, tc := range testCases {
		var paths []monorepo.Path
		for _, p := range tc.inputPaths {
			paths = append(paths, monorepo.NewPath(p))
		}
		mp := NewProviderWithFileName("CICD_TEST", ".test")
		got, err := mp.FindCicdFiles(mr, paths)
		if err != nil {
			t.Error(err)
			continue
		}
		if len(got) != len(tc.want) {
			t.Errorf("got: %q, want: %q", got, tc.want)
			continue
		}
		for i := 0; i < len(got); i++ {
			if got[i].Path != tc.want[i].Path {
			}
		}
	}
}

func createCicdFile(p string) File {
	return File{
		Path:  monorepo.NewPath(p),
		Proto: &cicdfilepb.CicdFile{},
	}
}
