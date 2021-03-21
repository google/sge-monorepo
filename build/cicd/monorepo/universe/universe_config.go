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

package universe

import (
	"fmt"
	"path/filepath"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/libs/go/p4lib"
)

// Def is a collection of monorepo definitions.
type Def []MonorepoDef

// MonorepoDef is the configuration of a single monorepo.
type MonorepoDef struct {
	// Name of the monorepo
	Name string

	// Root is an absolute p4 path (eg. "//some-depot").
	Root string

	// Excludes are any paths that are not be included in this repo.
	// These are excludes are relative to the monorepo root.
	//
	// Examples:
	// If the root is //1p/game, having an exclude of game/Content... will actually be excluding
	// the //1p/game/game/Content/... section of the Perforce repository. The point of these excludes
	// is that they're independent of where the Monorepo is being placed.
	Excludes []string

	// A list of presubmit checker tool configurations.
    // An usable example can be found in build/checks/tools/textpb.
	ToolConfigs []string
}

// Resolve generates a Monorepo by querying where the definition is located.
func (def MonorepoDef) Resolve(p4 p4lib.P4) (monorepo.Monorepo, error) {
	markerLocation, err := p4.Where(fmt.Sprintf("%s/%s", def.Root, monorepo.Marker))
	if err != nil {
		return monorepo.Monorepo{}, err
	}
	return monorepo.NewFromDir(filepath.Dir(markerLocation))
}

// globalDef is the hard-coded definition of the universe.
var globalDef = Def{}
