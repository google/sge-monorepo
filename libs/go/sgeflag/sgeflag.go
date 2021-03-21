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

// Package sgeflag augments the flag package.
package sgeflag

import (
	"strings"
)

// StringList is an option for a list of strings.
// It supports repeatedly setting a value, eg. -foo=a -foo=b.
type StringList []string

func (sl *StringList) String() string {
	return strings.Join(*sl, ", ")
}

func (sl *StringList) Set(value string) error {
	*sl = append(*sl, value)
	return nil
}
