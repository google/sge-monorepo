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
	"os"
	"path/filepath"
	"testing"
)

func TestVendoring(t *testing.T) {
	pwd, err := os.Getwd()
	if err != nil {
		t.Errorf("Failed to get working directory %v", err)
		return
	}
	ctx, err := buildContext(filepath.Join(pwd, "testdata"), false)
	if err != nil {
		t.Errorf("Failed to get build context directory %v", err)
		return
	}
	if len(ctx.pkgCtxs) != 5 {
		t.Errorf("test data should contains %d", len(ctx.pkgCtxs))
	}
	allPkgs := map[string]bool{
		"orphan_lib_existing":   false,
		"orphan_lib_2b_deleted": false,
		"cc_lib_existing":       false,
		"cc_lib_2b_added":       false,
		"go_lib_2b_updated":     false,
	}
	for _, pkg := range ctx.pkgCtxs {
		name := ""
		if pkg.entry != nil {
			name = pkg.entry.Name
		} else {
			name = pkg.pkg.metatada.Name
		}
		allPkgs[name] = true
	}
	for key, val := range allPkgs {
		if !val {
			t.Errorf("%s not found", key)
		}
	}
	buildVendoringPlan(&ctx, true)
	if err != nil {
		t.Errorf("Failed to get build vendoring plan %v", err)
		return
	}
}
