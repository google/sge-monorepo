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

// Package owners handles OWNER logic for the monorepo.
package owners

import (
	"io/ioutil"
	"path"
	"sort"
	"strings"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/libs/go/files"
)

// FindOwnersForFile returns a set with all owners in all OWNERS files reachable from the given path.
func FindOwnersForFile(mr monorepo.Monorepo, p monorepo.Path) (Set, error) {
	return findOwnersForFile(mr, p, false)
}

// FindOwnersForFile returns a set with all owners in the closest OWNERS file reachable from the given path.
func FindClosestOwnersForFile(mr monorepo.Monorepo, p monorepo.Path) (Set, error) {
	return findOwnersForFile(mr, p, true)
}

func findOwnersForFile(mr monorepo.Monorepo, p monorepo.Path, onlyClosest bool) (Set, error) {
	owners := Set{}
	d := p.Dir()
	for {
		ownersFile := path.Join(mr.ResolvePath(d), "OWNERS")
		if files.FileExists(ownersFile) {
			buf, err := ioutil.ReadFile(ownersFile)
			if err != nil {
				return nil, err
			}
			for _, line := range strings.Split(string(buf), "\n") {
				owners.Add(strings.TrimSpace(line))
			}
			if onlyClosest {
				break
			}
		}
		if d == "" {
			break
		}
		d = d.Dir()
	}
	return owners, nil
}

// Set is a set of owners.
type Set map[string]bool

// Add adds a name to the set.
func (s Set) Add(name string) {
	s[name] = true
}

// Covers returns whether this set contains an owner suitable for the other set.
// Examples:
// {joe@foo.com} against {joe@foo.com, bloe@foo.com} -> true
// {bloe@foo.com} against {joe@foo.com, bloe@foo.com} -> true
// {jake@foo.com} against {joe@foo.com, bloe@foo.com} -> false
func (s Set) Covers(other Set) bool {
	for o := range s {
		if other.Contains(o) {
			return true
		}
	}
	return false
}

// Contains returns whether the set contains the given owner name.
func (s Set) Contains(name string) bool {
	_, ok := s[name]
	return ok
}

// Sorted returns all owner names in the set in sorted order.
func (s Set) Sorted() []string {
	var owners []string
	for o := range s {
		owners = append(owners, o)
	}
	sort.Strings(owners)
	return owners
}

// HasCoverage returns whether the reviewer set covers all the owners for all files.
func HasCoverage(reviewers Set, mr monorepo.Monorepo, files []monorepo.Path) (bool, error) {
	for _, f := range files {
		owners, err := FindOwnersForFile(mr, f)
		if err != nil {
			return false, err
		}
		if !reviewers.Covers(owners) {
			return false, nil
		}
	}
	return true, nil
}
