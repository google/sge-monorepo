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
	"fmt"
	"time"

	"github.com/golang/glog"
)

type describecb []Description

const p4DateFormat = "2006/01/02 15:04:05"

func (cb *describecb) outputStat(stats map[string]string) error {
	idx := len(*cb)
	*cb = append(*cb, Description{})
	description := &(*cb)[idx]
	for key, value := range stats {
		if err := setTaggedField(description, key, value, false); err != nil {
			glog.Warningf("Couldn't set field %v: %v", key, err)
		}
	}
	if description.DateUnix != 0 {
		description.Date = time.Unix(description.DateUnix, 0).UTC().Format(p4DateFormat)
	}
	return nil
}
func (cb *describecb) tagProtocol() {}

// Describe invokes a "p4 describe" that gives details about a changelist
func (p4 *impl) Describe(cl []int) ([]Description, error) {
	cb := describecb{}
	args := make([]string, 0, len(cl))
	for _, c := range cl {
		args = append(args, fmt.Sprintf("%d", c))
	}
	err := p4.runCmdCb(&cb, "describe", args...)
	if err != nil {
		return nil, err
	}
	return cb, nil
}

// Desribe invokes a "p4 describe" that gives details about a (possibly shelved) changelist.
func (p4 *impl) DescribeShelved(cls ...int) ([]Description, error) {
	cb := describecb{}
	args := []string{"-S"}
	for _, c := range cls {
		args = append(args, fmt.Sprintf("%d", c))
	}
	err := p4.runCmdCb(&cb, "describe", args...)
	if err != nil {
		return nil, err
	}
	return cb, nil
}
