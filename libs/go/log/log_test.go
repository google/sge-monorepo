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

package log

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDefaultFormat(t *testing.T) {
	tests := []struct {
		args     []interface{}
		wantFmt  string
		wantArgs []interface{}
	}{
		{
			args:     []interface{}{1},
			wantFmt:  "%v",
			wantArgs: []interface{}{1},
		},
		{
			args:     []interface{}{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			wantFmt:  "%v %v %v %v %v %v %v %v %v %v",
			wantArgs: []interface{}{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
		{
			args:     []interface{}{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11},
			wantFmt:  "%v %v %v %v %v %v %v %v %v %v",
			wantArgs: []interface{}{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		},
	}

	for _, test := range tests {
		fmt, args := defaultFmtInternal(test.args...)
		if fmt != test.wantFmt {
			t.Errorf("unexpected format string, want '%s', got '%s'", test.wantFmt, fmt)
		}
		if diff := cmp.Diff(test.wantArgs, args); diff != "" {
			t.Errorf("unexpected args, want %v, got %v", test.wantArgs, args)
		}
	}
}
