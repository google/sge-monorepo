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

package trie

import "testing"

func TestPathTrieGet(t *testing.T) {
	testCases := []struct {
		desc      string
		paths     []string
		lookup    string
		wantFound bool
	}{
		{
			desc:      "empty",
			paths:     nil,
			lookup:    "foo",
			wantFound: false,
		},
		{
			desc:      "failed lookup",
			paths:     []string{"a/b", "a/c"},
			lookup:    "a/b/c",
			wantFound: false,
		},
		{
			desc:      "successful lookup",
			paths:     []string{"a/b", "a/c"},
			lookup:    "a/b",
			wantFound: true,
		},
	}
	for _, tc := range testCases {
		pt := NewPathTrie()
		for _, p := range tc.paths {
			// We put ourselves as the value to help in asserting we found the right node.
			pt.Put(p, p)
		}
		got := pt.Get(tc.lookup)
		var want interface{}
		if tc.wantFound {
			want = tc.lookup
		}
		if got != want {
			t.Errorf("[%s]: got %v want %v", tc.desc, got, want)
		}
	}
}

func TestPathTrie_GetLongestPrefix(t *testing.T) {
	testCases := []struct {
		desc   string
		paths  []string
		lookup string
		want   interface{}
	}{
		{
			desc:   "empty",
			paths:  nil,
			lookup: "foo",
			want:   nil,
		},
		{
			desc:   "exact match",
			paths:  []string{"a/b", "a/c"},
			lookup: "a/b",
			want:   "a/b",
		},
		{
			desc:   "child match",
			paths:  []string{"a/b"},
			lookup: "a/b/c",
			want:   "a/b",
		},
		{
			desc:   "no match",
			paths:  []string{"a/b"},
			lookup: "a/c",
			want:   nil,
		},
		{
			desc:   "most specific match",
			paths:  []string{"a/b", "a/b/c"},
			lookup: "a/b/c/d",
			want:   "a/b/c",
		},
		{
			desc:   "longest prefix match",
			paths:  []string{"a/b", "a/b/c/d"},
			lookup: "a/b/c",
			want:   "a/b",
		},
	}
	for _, tc := range testCases {
		pt := NewPathTrie()
		for _, p := range tc.paths {
			// We put ourselves as the value to help in asserting we found the right node.
			pt.Put(p, p)
		}
		got := pt.GetLongestPrefix(tc.lookup)
		if got != tc.want {
			t.Errorf("[%s]: got %v want %v", tc.desc, got, tc.want)
		}
	}
}

func TestPathTrie_GetShortestPrefix(t *testing.T) {
	testCases := []struct {
		desc   string
		paths  []string
		lookup string
		want   interface{}
	}{
		{
			desc:   "empty",
			paths:  nil,
			lookup: "foo",
			want:   nil,
		},
		{
			desc:   "exact match",
			paths:  []string{"a/b", "a/c"},
			lookup: "a/b",
			want:   "a/b",
		},
		{
			desc:   "child match",
			paths:  []string{"a/b"},
			lookup: "a/b/c",
			want:   "a/b",
		},
		{
			desc:   "no match",
			paths:  []string{"a/b"},
			lookup: "a/c",
			want:   nil,
		},
		{
			desc:   "most specific match",
			paths:  []string{"a/b", "a/b/c"},
			lookup: "a/b/c/d",
			want:   "a/b",
		},
	}
	for _, tc := range testCases {
		pt := NewPathTrie()
		for _, p := range tc.paths {
			// We put ourselves as the value to help in asserting we found the right node.
			pt.Put(p, p)
		}
		got := pt.GetShortestPrefix(tc.lookup)
		if got != tc.want {
			t.Errorf("[%s]: got %v want %v", tc.desc, got, tc.want)
		}
	}
}
