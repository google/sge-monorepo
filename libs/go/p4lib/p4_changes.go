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
	"time"

	"github.com/golang/glog"
)

type changecb []Change

func (cb *changecb) outputStat(stats map[string]string) error {
	idx := len(*cb)
	*cb = append(*cb, Change{})
	change := &(*cb)[idx]
	for key, value := range stats {
		if err := setTaggedField(change, key, value, false); err != nil {
			glog.Warningf("Couldn't set field %v: %v", key, err)
		}
	}
	if change.DateUnix != 0 {
		change.Date = time.Unix(change.DateUnix, 0).UTC().Format(p4DateFormat)
	}
	return nil
}
func (cb *changecb) tagProtocol() {}

// Changes executes a p4 changes command and returns a slice of p4 change details
func (p4 *impl) Changes(args ...string) ([]Change, error) {
	cb := changecb{}
	err := p4.runCmdCb(&cb, "changes", args...)
	if err != nil {
		return nil, err
	}
	return cb, nil
}
