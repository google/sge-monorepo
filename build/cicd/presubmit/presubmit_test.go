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

package presubmit

import (
	"fmt"
	"testing"

	"sge-monorepo/build/cicd/cicdfile"
	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/monorepo/universe"
	"sge-monorepo/build/cicd/presubmit/protos/presubmitpb"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/p4lib/p4mock"
	"sge-monorepo/libs/go/sgetest"

	"github.com/google/go-cmp/cmp"
)

func TestMatcher(t *testing.T) {
	mr := monorepo.New(`C:\ws`, map[string]monorepo.Path{
		"shared": monorepo.NewPath("shared"),
	})
	mdDir := monorepo.NewPath("mddir")
	testCases := []struct {
		path      string
		includes  []string
		excludes  []string
		wantMatch bool
		wantErr   string
	}{
		// "..."
		// In this case, since CICD is not at root, it should not find a root level file.
		{path: "file.txt", wantMatch: false},
		{path: "mddir/file.txt", wantMatch: true},
		{path: "mddir/within/file.txt", wantMatch: true},

		// Explicit "..."
		{path: "file.txt", includes: []string{"..."}, wantMatch: false},
		{path: "g/file.txt", includes: []string{"..."}, wantMatch: false},
		{path: "mddir/file.txt", includes: []string{"..."}, wantMatch: true},
		{path: "mddir/path/file.txt", includes: []string{"..."}, wantMatch: true},

		// Verify that we don't get false positives.
		{path: "mddir/path/file.txt", includes: []string{"..."}, wantMatch: true},
		{path: "mddirlonger/path/file.txt", includes: []string{"..."}, wantMatch: false},
		{path: "mddir/path/file.txt", includes: []string{"path/..."}, wantMatch: true},
		{path: "mddir/paths/file.txt", includes: []string{"path/..."}, wantMatch: false},

		// Include subpath.
		{path: "mddir/file.txt", includes: []string{"path/..."}, wantMatch: false},
		{path: "mddir/path/file.txt", includes: []string{"path/..."}, wantMatch: true},
		{path: "mddir/path/within/file.txt", includes: []string{"path/..."}, wantMatch: true},

		{path: "mddir/file.txt", includes: []string{"other/..."}, wantMatch: false},
		{path: "mddir/path/file.txt", includes: []string{"other/..."}, wantMatch: false},
		{path: "mddir/path/within/file.txt", includes: []string{"other/..."}, wantMatch: false},

		// Multiple includes.
		{path: "mddir/file.txt", includes: []string{"path/...", "other/..."}, wantMatch: false},
		{path: "mddir/path/file.txt", includes: []string{"path/...", "other/..."}, wantMatch: true},
		{path: "mddir/path/within/file.txt", includes: []string{"path/...", "other/..."}, wantMatch: true},
		{path: "mddir/other/file.txt", includes: []string{"path/...", "other/..."}, wantMatch: true},
		{path: "mddir/other/within/file.txt", includes: []string{"path/...", "other/..."}, wantMatch: true},
		{path: "mddir/another/file.txt", includes: []string{"path/...", "other/..."}, wantMatch: false},
		{path: "mddir/another/within/file.txt", includes: []string{"path/...", "other/..."}, wantMatch: false},

		// Absolute.
		{path: "file.txt", includes: []string{"//..."}, wantMatch: true},
		{path: "mddir/file.txt", includes: []string{"//..."}, wantMatch: true},
		{path: "mddir/path/within/file.txt", includes: []string{"//..."}, wantMatch: true},
		{path: "mddir/other/within/file.txt", includes: []string{"//..."}, wantMatch: true},
		{path: "mddir/another/within/file.txt", includes: []string{"//..."}, wantMatch: true},

		{path: "file.txt", includes: []string{"//outside/..."}, wantMatch: false},
		{path: "mddir/file.txt", includes: []string{"//outside/..."}, wantMatch: false},
		{path: "mddir/within/file.txt", includes: []string{"//outside/..."}, wantMatch: false},
		{path: "outside/file.txt", includes: []string{"//outside/..."}, wantMatch: true},
		{path: "outside/within/file.txt", includes: []string{"//outside/..."}, wantMatch: true},

		// Excludes.
		{path: "mddir/file.txt", excludes: []string{"excluded/..."}, wantMatch: true},
		{path: "mddir/path/file.txt", excludes: []string{"excluded/..."}, wantMatch: true},
		{path: "mddir/path/within/file.txt", excludes: []string{"excluded/..."}, wantMatch: true},
		{path: "mddir/excluded/file.txt", excludes: []string{"excluded/..."}, wantMatch: false},
		{path: "mddir/excluded/within/file.txt", excludes: []string{"excluded/..."}, wantMatch: false},

		// Exact file.
		{path: "mddir/path/file.txt", includes: []string{"path/file.txt"}, wantMatch: true},
		{path: "mddir/file.txt", includes: []string{"path/file.txt"}, wantMatch: false},
		{path: "mddir/within/file.txt", includes: []string{"path/file.txt"}, wantMatch: false},
		{path: "mddir/path/file2.txt", includes: []string{"path/file.txt"}, wantMatch: false},
		{path: "mddir/path/file.rs", includes: []string{"path/file.txt"}, wantMatch: false},

		// Include/exclude
		{path: "mddir/path/file.txt", includes: []string{"path/file.txt"}, excludes: []string{"..."}, wantMatch: false},
		{path: "mddir/path/file.txt", includes: []string{"path/file.txt"}, excludes: []string{"path/file.txt"}, wantMatch: false},
		{path: "outside/file.txt", includes: []string{"//outside/..."}, excludes: []string{"..."}, wantMatch: true},

		// // By extension
		{path: "file.uasset", includes: []string{"....uasset"}, wantMatch: false},
		{path: "mddir/file.uasset", includes: []string{"....uasset"}, wantMatch: true},
		{path: "mddir/within/file.uasset", includes: []string{"....uasset"}, wantMatch: true},
		{path: "outside/file.uasset", includes: []string{"....uasset"}, wantMatch: false},
		{path: "file.rs", includes: []string{"....uasset"}, wantMatch: false},
		{path: "mddir/file.rs", includes: []string{"....uasset"}, wantMatch: false},
		{path: "mddir/within/file.rs", includes: []string{"....uasset"}, wantMatch: false},
		{path: "outside/file.rs", includes: []string{"....uasset"}, wantMatch: false},

		{path: "file.uasset", includes: []string{"//....uasset"}, wantMatch: true},
		{path: "mddir/file.uasset", includes: []string{"//....uasset"}, wantMatch: true},
		{path: "mddir/within/file.uasset", includes: []string{"//....uasset"}, wantMatch: true},
		{path: "outside/file.uasset", includes: []string{"//....uasset"}, wantMatch: true},
		{path: "file.rs", includes: []string{"//....uasset"}, wantMatch: false},
		{path: "mddir/file.rs", includes: []string{"//....uasset"}, wantMatch: false},
		{path: "mddir/within/file.rs", includes: []string{"//....uasset"}, wantMatch: false},
		{path: "outside/file.rs", includes: []string{"//....uasset"}, wantMatch: false},

		// // Only within a directory.
		{path: "file.rs", includes: []string{"//outside/*.rs"}, wantMatch: false},
		{path: "mddir/file.rs", includes: []string{"//outside/*.rs"}, wantMatch: false},
		{path: "outside/file.rs", includes: []string{"//outside/*.rs"}, wantMatch: true},
		{path: "outside/other_file.rs", includes: []string{"//outside/*.rs"}, wantMatch: true},
		{path: "outside/within/file.rs", includes: []string{"//outside/*.rs"}, wantMatch: false},
		{path: "file.uasset", includes: []string{"//outside/*.rs"}, wantMatch: false},
		{path: "mddir/file.uasset", includes: []string{"//outside/*.rs"}, wantMatch: false},
		{path: "outside/file.uasset", includes: []string{"//outside/*.rs"}, wantMatch: false},
		{path: "outside/other_file.uasset", includes: []string{"//outside/*.rs"}, wantMatch: false},
		{path: "outside/within/file.uasset", includes: []string{"//outside/*.rs"}, wantMatch: false},

		// Exclude filetype.
		{path: "file.rs", includes: []string{"//..."}, excludes: []string{"....uasset"}, wantMatch: true},
		{path: "mddir/file.rs", includes: []string{"//..."}, excludes: []string{"....uasset"}, wantMatch: true},
		{path: "outside/file.rs", includes: []string{"//..."}, excludes: []string{"....uasset"}, wantMatch: true},
		{path: "outside/other_file.rs", includes: []string{"//..."}, excludes: []string{"....uasset"}, wantMatch: true},
		{path: "outside/within/file.rs", includes: []string{"//..."}, excludes: []string{"....uasset"}, wantMatch: true},
		{path: "file.uasset", includes: []string{"//..."}, excludes: []string{"....uasset"}, wantMatch: true},
		{path: "mddir/file.uasset", includes: []string{"//..."}, excludes: []string{"....uasset"}, wantMatch: false},

		{path: "outside/file.uasset", includes: []string{"//..."}, excludes: []string{"//outside/....uasset"}, wantMatch: false},
		{path: "outside/other_file.uasset", includes: []string{"//..."}, excludes: []string{"//outside/....uasset"}, wantMatch: false},
		{path: "outside/within/file.uasset", includes: []string{"//..."}, excludes: []string{"//outside/....uasset"}, wantMatch: false},

		// Exclude numerical.
		{path: "file.rs", includes: []string{"//..."}, excludes: []string{"//*[0-9]*"}, wantMatch: true},
		{path: "file0.rs", includes: []string{"//..."}, excludes: []string{"//*[0-9]*"}, wantMatch: false},
		{path: "10", includes: []string{"//..."}, excludes: []string{"//*[0-9]*"}, wantMatch: false},

		// Sub repo.
		{path: "file.rs", includes: []string{"@shared//..."}, excludes: []string{"@shared//path/....uasset"}, wantMatch: false},
		{path: "shared/file.rs", includes: []string{"@shared//..."}, excludes: []string{"@shared//path/....uasset"}, wantMatch: true},
		{path: "shared/path/file.rs", includes: []string{"@shared//..."}, excludes: []string{"@shared//path/....uasset"}, wantMatch: true},
		{path: "shared/asset.uasset", includes: []string{"@shared//..."}, excludes: []string{"@shared//path/....uasset"}, wantMatch: true},
		{path: "shared/path/asset.uasset", includes: []string{"@shared//..."}, excludes: []string{"@shared//path/....uasset"}, wantMatch: false},
	}

	for _, tc := range testCases {
		ps := &presubmitpb.Presubmit{
			Include: tc.includes,
			Exclude: tc.excludes,
		}
		m, gotErr := newMatcher(mr, mdDir, ps)
		if err := sgetest.CmpErr(gotErr, tc.wantErr); err != nil {
			t.Errorf("[%v] %s", tc, err)
			continue
		}
		got, err := m.match(monorepo.NewPath(tc.path))
		if err != nil {
			t.Errorf("[%v] unexpected error: %v", tc, err)
			continue
		}
		if got != tc.wantMatch {
			t.Errorf("[%v] want: %t, got: %t", tc, tc.wantMatch, got)
		}
	}
}

func TestAnalyzeChange(t *testing.T) {
	shared := universe.MonorepoDef{
		Root: "//shared",
	}
	foo := universe.MonorepoDef{
		Root: "//foo",
	}
	bar := universe.MonorepoDef{
		Root: "//bar",
	}
	u, err := universe.NewFromDef(universe.Def{
		shared,
		foo,
		bar,
	})
	if err != nil {
		t.Fatal(err)
	}
	p4 := p4mock.New()
	p4.OpenedFunc = func(change string) ([]p4lib.OpenedFile, error) {
		return []p4lib.OpenedFile{
			{Path: "//shared/a.txt", Status: p4lib.DiffChange},
			{Path: "//shared/b.txt", Status: p4lib.DiffChange},
			{Path: "//foo/foo.txt", Status: p4lib.DiffChange},
		}, nil
	}
	p4.WhereFunc = func(p string) (s string, err error) {
		switch p {
		case "//shared/MONOREPO":
			return "testdata/shared/MONOREPO", nil
		case "//foo/MONOREPO":
			return "testdata/foo/MONOREPO", nil
		}
		return "", fmt.Errorf("unknown path %s", p)
	}
	mp := cicdfile.NewProviderWithFileName("CICD_TEST", ".test")
	r := NewRunner(u, p4, mp).(*runner)
	got, err := r.analyzeChange()
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		monorepo  string
		wantFiles []string
	}{
		{
			monorepo:  "testdata/foo",
			wantFiles: []string{"foo.txt"},
		},
		{
			monorepo:  "testdata/shared",
			wantFiles: []string{"a.txt", "b.txt"},
		},
	}
	// Depends on testdata CICD.test files.
	// There is one CICD file in each root, with presubmits matching everything.
	for _, w := range want {
		foundMr := false
		for _, ts := range got {
			if ts.monorepo.Root == w.monorepo {
				var gotFiles []string
				for _, t := range ts.triggered {
					for _, f := range t.matchingFiles {
						gotFiles = append(gotFiles, string(f.path))
					}
				}
				if !cmp.Equal(gotFiles, w.wantFiles) {
					t.Errorf("wrong paths found for monorepo %s. got %v want %v", w.monorepo, gotFiles, w.wantFiles)
				}
				foundMr = true
			}
		}
		if !foundMr {
			t.Errorf("could not find set for monorepo %s", w.monorepo)
		}
	}
}
