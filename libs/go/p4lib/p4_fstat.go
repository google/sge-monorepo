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

package p4lib

import (
	"strings"

	"github.com/golang/glog"
)

// Make FstatResult a StatHandler.
func (fs *FstatResult) outputStat(stats map[string]string) error {
	if desc, ok := stats["desc"]; ok {
		// Assign the description to the FstatResult.
		fs.Desc = strings.TrimSpace(desc)
	} else {
		file := &FileStat{}
		for key, value := range stats {
			if err := setTaggedField(file, key, value, false); err != nil {
				glog.Warningf("Couldn't set field %v: %v", key, err)
			}
		}
		fs.FileStats = append(fs.FileStats, *file)
	}
	return nil
}

// Fstat invokes a "p4 fstat" which collects details about the specified file(s)
func (p4 *impl) Fstat(args ...string) (*FstatResult, error) {
	fs := &FstatResult{}
	err := p4.runCmdCb(fs, "fstat", args...)
	if err != nil {
		return nil, err
	}
	return fs, nil
}
