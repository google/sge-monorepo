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

// Package sgetest provides testing utilities.
package sgetest

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"
)

// CmpErr compares the an error against an expected substring.
// If err != nil and wantErr == "", an error is returned.
// If err == nil and wantErr != "", an error is returned.
// If err != nil but the error doesn't contain the substring in wantErr, an error is returned.
// If err == nil and wantErr == "", no error is returned.
// If err != nil and contains the wantErr substring, no error is returned.
func CmpErr(err error, wantErr string) error {
	if err == nil && wantErr != "" {
		return fmt.Errorf("want error containing %q, got no error", wantErr)
	}
	if err != nil {
		if wantErr == "" {
			return fmt.Errorf("want no error, got %v", err)
		} else if !strings.Contains(err.Error(), wantErr) {
			return fmt.Errorf("want error containing %q, got %v", wantErr, err)
		}
	}
	return nil
}

// WriteFiles creates the passed files rooted in the given root.
// All necessary directories are created.
func WriteFiles(root string, files map[string]string) error {
	for fp, content := range files {
		p := path.Join(root, fp)
		if err := os.MkdirAll(path.Dir(p), 0644); err != nil {
			return err
		}
		if err := ioutil.WriteFile(p, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}
