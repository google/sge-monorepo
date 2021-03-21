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
	"io/ioutil"
	"os"
	"path"
	"sort"
	"testing"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/sgeb/protos/sgebpb"
	"sge-monorepo/libs/go/sgetest"

	"github.com/golang/protobuf/proto"
	"github.com/google/go-cmp/cmp"
)

func TestLoadBuildUnits(t *testing.T) {
	testCases := []struct {
		desc    string
		path    string
		content string
		loadBu  string
		wantBu  *sgebpb.BuildUnit
		loadTu  string
		wantTu  *sgebpb.TestUnit
	}{
		{
			desc: "simple load",
			path: "foo/BUILDUNIT",
			content: `
build_unit {
  name: 'foo',
  target: 'foo',
}
test_unit {
  name: 'foo_test',
  target: 'foo_test',
}
`,
			loadBu: "//foo:foo",
			wantBu: &sgebpb.BuildUnit{
				Name:   "foo",
				Target: "foo",
			},
			loadTu: "//foo:foo_test",
			wantTu: &sgebpb.TestUnit{
				Name:   "foo_test",
				Target: []string{"foo_test"},
			},
		},
		{
			desc:   "missing BUILDUNIT file",
			loadBu: "//foo:foo",
			wantBu: nil,
		},
	}
	for _, tc := range testCases {
		wsDir, err := ioutil.TempDir("", "ws")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(wsDir)
		if err = ioutil.WriteFile(path.Join(wsDir, "MONOREPO"), []byte{}, 0644); err != nil {
			t.Fatal(err)
		}
		if err = ioutil.WriteFile(path.Join(wsDir, "WORKSPACE"), []byte{}, 0644); err != nil {
			t.Fatal(err)
		}
		if tc.path != "" {
			if err = os.MkdirAll(path.Join(wsDir, path.Dir(tc.path)), 0644); err != nil {
				t.Fatal(err)
			}
			if err = ioutil.WriteFile(path.Join(wsDir, tc.path), []byte(tc.content), 0644); err != nil {
				t.Fatal(err)
			}
		}
		mr, err := monorepo.NewFromDir(wsDir)
		if err != nil {
			t.Fatalf("could not load monorepo from %s: %v", wsDir, err)
		}
		bc, err := NewContext(mr)
		if err != nil {
			t.Fatal(err)
		}
		defer bc.Cleanup()
		c := bc.(*context)
		if tc.loadBu != "" {
			l, err := mr.NewLabel("", tc.loadBu)
			if err != nil {
				t.Fatal(l)
			}
			var got *sgebpb.BuildUnit
			d, err := c.Monorepo.ResolveLabelPkgDir(l)
			if err != nil {
				t.Fatal(err)
			}
			bus, err := c.LoadBuildUnits(d)
			if err == nil {
				got, _ = c.findBuildUnit(bus, l)
			}
			if !proto.Equal(got, tc.wantBu) {
				t.Errorf("[%s] LoadBuildUnit(%s)=%v, want %v", tc.desc, tc.loadBu, got, tc.wantBu)
			}
		}
		if tc.loadTu != "" {
			l, err := mr.NewLabel("", tc.loadTu)
			if err != nil {
				t.Fatal(l)
			}
			var got *sgebpb.TestUnit
			d, err := c.Monorepo.ResolveLabelPkgDir(l)
			if err != nil {
				t.Fatal(err)
			}
			bus, err := c.LoadBuildUnits(d)
			if err == nil {
				got, _ = c.findTestUnit(bus, l)
			}
			if !proto.Equal(got, tc.wantTu) {
				t.Errorf("[%s] LoadBuildUnit(%s)=%v, want %v", tc.desc, tc.loadTu, got, tc.wantBu)
			}
		}
	}
}

func TestValidateBuildUnits(t *testing.T) {
	testCases := []struct {
		desc    string
		input   *sgebpb.BuildUnits
		wantErr string
	}{
		{
			desc:  "empty build units",
			input: &sgebpb.BuildUnits{},
		},
		{
			desc: "simple valid build units",
			input: &sgebpb.BuildUnits{
				BuildUnit: []*sgebpb.BuildUnit{
					{
						Name:   "foo",
						Target: "//foo:foo",
					},
					{
						Name:   "bar",
						Target: "//bar:bar",
					},
				},
			},
		},
		{
			desc: "duplicate build units",
			input: &sgebpb.BuildUnits{
				BuildUnit: []*sgebpb.BuildUnit{
					{
						Name:   "foo",
						Target: "//foo:foo",
					},
					{
						Name:   "foo",
						Target: "//foo:foo",
					},
				},
			},
			wantErr: "same name",
		},
		{
			desc: "duplicate build/test units",
			input: &sgebpb.BuildUnits{
				BuildUnit: []*sgebpb.BuildUnit{
					{
						Name:   "foo",
						Target: "//foo:foo",
					},
				},
				TestUnit: []*sgebpb.TestUnit{
					{
						Name:   "foo",
						Target: []string{"//foo:foo"},
					},
				},
			},
			wantErr: "same name",
		},
		{
			desc: "must have target or bin",
			input: &sgebpb.BuildUnits{
				BuildUnit: []*sgebpb.BuildUnit{
					{
						Name: "foo",
					},
				},
			},
			wantErr: "target or bin",
		},
		{
			desc: "must not have target and bin",
			input: &sgebpb.BuildUnits{
				BuildUnit: []*sgebpb.BuildUnit{
					{
						Name:   "foo",
						Target: "foo",
						Bin:    "foo",
					},
				},
			},
			wantErr: "target and bin",
		},
		{
			desc: "must not have env vars",
			input: &sgebpb.BuildUnits{
				BuildUnit: []*sgebpb.BuildUnit{
					{
						Name:    "foo",
						Target:  "foo",
						EnvVars: []*sgebpb.EnvVar{{}},
					},
				},
			},
			wantErr: "env vars",
		},
		{
			desc: "must not have env vars",
			input: &sgebpb.BuildUnits{
				BuildUnit: []*sgebpb.BuildUnit{
					{
						Name:   "foo",
						Target: "foo",
						Deps:   []string{"dep"},
					},
				},
			},
			wantErr: "deps",
		},
		{
			desc: "can have just trigger_paths",
			input: &sgebpb.BuildUnits{
				PublishUnit: []*sgebpb.PublishUnit{
					{
						Name: "foo",
                        Bin: "//some/builder",
                        BuildUnit: []string{"//foo/build"},
						PostSubmit: &sgebpb.PostSubmit{
							TriggerPaths: &sgebpb.PostSubmitTriggerPathSet{},
						},
					},
				},
			},
		},
		{
			desc: "cannot have trigger_paths and frequency",
			input: &sgebpb.BuildUnits{
				PublishUnit: []*sgebpb.PublishUnit{
					{
						Name: "foo",
                        Bin: "//some/builder",
                        BuildUnit: []string{"//foo/build"},
						PostSubmit: &sgebpb.PostSubmit{
							TriggerPaths: &sgebpb.PostSubmitTriggerPathSet{},
							Frequency:    &sgebpb.PostSubmitFrequency{},
						},
					},
				},
			},
			wantErr: "trigger_paths and frequency",
		},
	}
	for _, tc := range testCases {
		err := validateBuildUnits(tc.input)
		if err := sgetest.CmpErr(err, tc.wantErr); err != nil {
			t.Errorf("[%s] %v", tc.desc, err)
			continue
		}
	}
}

func TestExpandTestSuite(t *testing.T) {
	files := map[string]string{
		"MONOREPO":  "",
		"WORKSPACE": "",
		"BUILDUNIT": `
test_suite {
  name: "foo_all_suite"
  test_unit: "foo:all_suite"
}

test_suite {
  name: "just_foo_suite"
  test_unit: "foo:foo"
}
`,
		"foo/BUILDUNIT": `
test_suite {
  name: "all_suite"
  test_unit: "..."
}

test_unit {
  name: "foo"
  bin: "nop"
}
`,
		"foo/bar/BUILDUNIT": `
test_unit {
  name: "bar"
  bin: "nop"
}
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
	testCases := []struct {
		tu   string
		want []string
	}{
		{
			tu: "//foo/bar:bar",
			want: []string{
				"//foo/bar:bar",
			},
		},
		{
			tu: "//foo:all_suite",
			want: []string{
				"//foo/bar:bar",
				"//foo:foo",
			},
		},
		{
			tu: "//:foo_all_suite", // Points to //foo:all_suite
			want: []string{
				"//foo/bar:bar",
				"//foo:foo",
			},
		},
		{
			tu: "//:just_foo_suite", // Points to //foo:foo
			want: []string{
				"//foo:foo",
			},
		},
	}
	for _, tc := range testCases {
		mr, err := monorepo.NewFromDir(wsDir)
		if err != nil {
			t.Fatalf("could not load monorepo from %s: %v", wsDir, err)
		}
		bc, err := NewContext(mr)
		if err != nil {
			t.Fatal(err)
		}
		defer bc.Cleanup()
		gotLabels, err := bc.ExpandTargetExpression(monorepo.TargetExpression(tc.tu))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, l := range gotLabels {
			got = append(got, l.String())
		}
		sort.Strings(got)
		if !cmp.Equal(got, tc.want) {
			t.Errorf("ExpandTestSuite(%s)=%v, want %v", tc.tu, got, tc.want)
		}
	}
}
