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
)

func TestCollectMappings(t *testing.T) {
	mappings := []mapping{}
	err := collectMappings(".", "file.txt", "file", &mappings)
	if err != nil {
		t.Errorf("failed on collecting mappings: %s", err)
		return
	}
	testMappings := []struct {
		m     mapping
		found bool
	}{
		{mapping{"testdata/file.txt", "testdata/file"}, false},
		{mapping{"testdata/folder/file.txt", "testdata/folder/file"}, false},
		{mapping{"testdata/other_folder/subfolder/file/file.txt", "testdata/other_folder/subfolder/file/file"}, false},
	}

	if len(testMappings) != len(mappings) {
		t.Errorf("got %v expected %v", mappings, testMappings)
		return
	}

	for _, mapping := range mappings {
		for idx, testMapping := range testMappings {
			if mapping.fromPath == testMapping.m.fromPath && mapping.toPath == testMapping.m.toPath {
				if testMapping.found {
					t.Errorf("mapping %q already found", mapping)
					break
				}
				testMappings[idx].found = true
				break
			}
		}
	}

	for _, testMapping := range testMappings {
		if testMapping.found == false {
			t.Errorf("failed to find: %q", testMapping.m)
		}
	}
}
