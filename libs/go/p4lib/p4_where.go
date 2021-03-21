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
)

// Define callback interface for working with p4 where.
type wherecb map[string]string

func (cb *wherecb) outputStat(stats map[string]string) error {
	p, ok := stats["path"]
	if !ok {
		return fmt.Errorf("missing 'path' in %v", stats)
	}
	(*cb)["path"] = p
	return nil
}
func (cb *wherecb) retry(_, _ string) {
	*cb = wherecb{}
}

func (cb *wherecb) tagProtocol() {}

func (p4 *impl) Where(p string) (string, error) {
	cb := wherecb{}
	err := p4.runCmdCb(&cb, "where", p)
	if err != nil {
		return "", err
	}
	if v, ok := cb["path"]; ok {
		return v, nil
	}
	return "", fmt.Errorf("p4 where: could not find path %q", p)
}

// Define callback interface for working with p4 where supporting multiple files.
type whereexcb struct {
	paths []string
}

func (cb *whereexcb) outputStat(stats map[string]string) error {
	p, ok := stats["path"]
	if !ok {
		return fmt.Errorf("missing 'path' in %v", stats)
	}
	cb.paths = append(cb.paths, p)
	return nil
}
func (cb *whereexcb) retry(_, _ string) {
	cb.paths = nil
}

func (cb *whereexcb) tagProtocol() {}

func (p4 *impl) WhereEx(paths []string) ([]string, error) {
	cb := whereexcb{}
	err := p4.runCmdCb(&cb, "where", paths...)
	if err != nil {
		return nil, err
	}
	return cb.paths, nil
}
