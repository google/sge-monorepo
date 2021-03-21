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

package review

import (
	"testing"

	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/p4lib/p4mock"
	"sge-monorepo/tools/ebert/ebert"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestGetFilePairs(t *testing.T) {
	tests := []struct {
		name           string
		baseCl, currCl int
		currPending    bool
		descriptions   map[int]p4lib.Description
		want           map[string]*FilePair
	}{
		{
			name:        "pending-edit",
			baseCl:      0,
			currCl:      2,
			currPending: true,
			descriptions: map[int]p4lib.Description{
				2: p4lib.Description{
					Files: []p4lib.FileAction{
						p4lib.FileAction{
							DepotPath: "//a/b",
							Revision:  2,
							Action:    "edit",
							Type:      "text",
						},
					},
				},
			},
			want: map[string]*FilePair{
				"//a/b": &FilePair{
					From:     fileRev{name: "//a/b", rev: 2},
					To:       fileRev{name: "//a/b", cl: 2},
					Action:   "edit",
					FileType: "text",
				},
			},
		},
		{
			name:        "submitted-edit",
			baseCl:      0,
			currCl:      2,
			currPending: false,
			descriptions: map[int]p4lib.Description{
				2: p4lib.Description{
					Files: []p4lib.FileAction{
						p4lib.FileAction{
							DepotPath: "//a/b",
							Revision:  2,
							Action:    "edit",
							Type:      "text",
						},
					},
				},
			},
			want: map[string]*FilePair{
				"//a/b": &FilePair{
					From:     fileRev{name: "//a/b", rev: 1},
					To:       fileRev{name: "//a/b", cl: 2},
					Action:   "edit",
					FileType: "text",
				},
			},
		},
		{
			name:        "add-delete",
			baseCl:      1,
			currCl:      2,
			currPending: true,
			descriptions: map[int]p4lib.Description{
				1: p4lib.Description{
					Files: []p4lib.FileAction{
						p4lib.FileAction{
							DepotPath: "//a/b",
							Revision:  2,
							Action:    "add",
							Type:      "text",
						},
					},
				},
				2: p4lib.Description{
					Files: []p4lib.FileAction{},
				},
			},
			want: map[string]*FilePair{
				"//a/b": &FilePair{
					From:     fileRev{name: "//a/b", cl: 1},
					Action:   "delete",
					FileType: "text",
				},
			},
		},
		{
			name:        "delete-add",
			baseCl:      1,
			currCl:      2,
			currPending: true,
			descriptions: map[int]p4lib.Description{
				1: p4lib.Description{
					Files: []p4lib.FileAction{
						p4lib.FileAction{
							DepotPath: "//a/b",
							Revision:  2,
							Action:    "delete",
							Type:      "text",
						},
					},
				},
				2: p4lib.Description{
					Files: []p4lib.FileAction{},
				},
			},
			want: map[string]*FilePair{
				"//a/b": &FilePair{
					From:     fileRev{name: "//a/b", rev: 2},
					To:       fileRev{name: "//a/b", cl: 2},
					Action:   "add",
					FileType: "text",
				},
			},
		},
		{
			name:        "multiple-edits",
			baseCl:      1,
			currCl:      2,
			currPending: true,
			descriptions: map[int]p4lib.Description{
				1: p4lib.Description{
					Files: []p4lib.FileAction{
						p4lib.FileAction{
							DepotPath: "//a/b",
							Revision:  2,
							Action:    "edit",
							Type:      "text",
							Digest:    "b1",
						},
					},
				},
				2: p4lib.Description{
					Files: []p4lib.FileAction{
						p4lib.FileAction{
							DepotPath: "//a/b",
							Revision:  2,
							Action:    "edit",
							Type:      "text",
							Digest:    "b2",
						},
					},
				},
			},
			want: map[string]*FilePair{
				"//a/b": &FilePair{
					From:     fileRev{name: "//a/b", cl: 1},
					To:       fileRev{name: "//a/b", cl: 2},
					Action:   "edit",
					FileType: "text",
				},
			},
		},
	}

	for _, test := range tests {
		p4 := p4mock.New()
		p4.DescribeShelvedFunc = func(cls ...int) ([]p4lib.Description, error) {
			descs := make([]p4lib.Description, 0, len(cls))
			for _, cl := range cls {
				descs = append(descs, test.descriptions[cl])
			}
			return descs, nil
		}
		ctx := &ebert.Context{P4: p4}

		pairs, err := getFilePairs(ctx, test.baseCl, test.currCl, test.currPending, true)
		if err != nil {
			t.Errorf("%s getFilePairs: %v", test.name, err)
		}
		if diff := cmp.Diff(test.want, pairs, cmpopts.IgnoreUnexported(FilePair{}), cmp.AllowUnexported(fileRev{})); diff != "" {
			t.Errorf("%s getFilePair diff (-want +got):\n%s", test.name, diff)
		}
	}
}

func TestExtractBugsFromDescription(t *testing.T) {
	type want struct {
		bugs  []int
		fixes []int
		desc  string
	}
	tests := []struct {
		desc string
		want want
	}{
		{
			desc: "No bugs in this\ntwo line description.",
			want: want{
				bugs:  nil,
				fixes: nil,
			},
		}, {
			desc: "Bug signifier doesn't start line\n BUG=123",
			want: want{
				bugs:  nil,
				fixes: nil,
			},
		}, {
			desc: "Invalid bug id\nBUG=foo",
			want: want{
				bugs:  nil,
				fixes: nil,
			},
		}, {
			desc: "Have BUG but not FIX\nBUG= b/123, https://b/456, 789",
			want: want{
				bugs:  []int{123, 456, 789},
				fixes: nil,
			},
		}, {
			desc: "Have FIX but not BUG\nFIX= b/123, https://b/456, 789",
			want: want{
				bugs:  nil,
				fixes: []int{123, 456, 789},
			},
		}, {
			desc: "Have both BUG and FIX\nBUG= b/123\nFIX= b/456",
			want: want{
				bugs:  []int{123},
				fixes: []int{456},
			},
		},
	}

	for _, test := range tests {
		bugs, fixes := bugsFromDescription(test.desc)
		if diff := cmp.Diff(test.want.bugs, bugs); diff != "" {
			t.Errorf("incorrect bugs for '%s': got %v, want %v", test.desc, bugs, test.want.bugs)
		}
		if diff := cmp.Diff(test.want.fixes, fixes); diff != "" {
			t.Errorf("incorrect fixes for '%s': got %v, want %v", test.desc, fixes, test.want.fixes)
		}
	}
}
