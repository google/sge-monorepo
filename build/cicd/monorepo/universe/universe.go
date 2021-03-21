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
	"strings"

	"sge-monorepo/libs/go/p4lib"
)

// Universe is the collection of monorepos.
type Universe struct {
	Udef Def
}

// New constructs a universe from the global def
func New() (Universe, error) {
	return NewFromDef(globalDef)
}

// NewFromDef constructs a universe from a supplied def
func NewFromDef(def Def) (Universe, error) {
	if err := validateUniverseDef(def); err != nil {
		return Universe{}, err
	}
	return Universe{def}, nil
}

// GetMonorepo retrieves a given monorepo from name. Returns nil if not found.
func (u *Universe) GetMonorepo(name string) *MonorepoDef {
	for _, mr := range u.Udef {
		if mr.Name == name {
			return &mr
		}
	}
	return nil
}

// UpdateClientView updates the given client to have the viewspec defined by the given
// universe. If |clientName| is empty, it means the default P4CLIENT.
func (u *Universe) UpdateClientView(p4 p4lib.P4, clientName string) error {
	client, err := u.createP4View(p4, clientName)
	if err != nil {
		return err
	}
	// P4lib will return stdout/stderr of the app, which has the actual error message in there.
	out, err := p4.ClientSet(client)
	if err != nil {
		return fmt.Errorf("%s: %s", err, out)
	}
	return nil
}

// If |clientName| is empty, it means the default P4CLIENT.
func (u *Universe) createP4View(p4 p4lib.P4, clientName string) (*p4lib.Client, error) {
	// Synthesize a client spec from the default current one.
	client, err := p4.Client(clientName)
	if err != nil {
		return nil, err
	}

	// ViewEntries map from perforce path to client path. In here we always map the same
	// directory structure as in the Perforce server.
	// We don't use filepath.Join here because we control the separator.
	//
	// Example:
	//      //foo/some-project/... //CLIENT_NAME/foo/some-project/...
	//      -//foo/some-project/ue4/... //CLIENT_NAME/foo/some-project/ue4/...
	var viewEntries []p4lib.ViewEntry
	for _, mr := range u.Udef {
		// We add the root. Roots are always absolute paths to a dir (eg. //foo/some-project).
		root := fmt.Sprintf("%s/...", mr.Root)
		viewEntries = append(viewEntries, p4lib.ViewEntry{
			Source:      root,
			Destination: fmt.Sprintf("//%s/%s", client.Client, root[2:]),
		})

		// Add any exclusions. They are prepended with a "-".
		// All excludes are monorepo-root relative.
		for _, exclude := range mr.Excludes {
			// We add the root to it.
			absExclude := fmt.Sprintf("%s/%s", mr.Root, exclude)
			viewEntries = append(viewEntries, p4lib.ViewEntry{
				Source:      fmt.Sprintf("-%s", absExclude),
				Destination: fmt.Sprintf("//%s/%s", client.Client, absExclude[2:]),
			})
		}
	}

	// Replace the new View into the client and commit it.
	client.View = viewEntries
	return client, nil
}

func validateUniverseDef(udef Def) error {
	// Monorepos should be disjoint. In this context, we check that no root is a child
	// of another root or they are not the same.
	for _, mr := range udef {
		if len(mr.Root) < 3 || mr.Root[0:2] != "//" {
			return fmt.Errorf("monorepo root should start with //. name: %s, root: %s",
				mr.Name, mr.Root)
		}
		// Verify that excludes are relative.
		for _, exclude := range mr.Excludes {
			if !isRelativePath(exclude) {
				return fmt.Errorf("monorepo exclude path should not be absolute. name: %s, exclude: %s",
					mr.Name, exclude)
			}
		}
		for _, other := range udef {
			if mr.Root == other.Root && mr.Name != other.Name {
				return fmt.Errorf("monorepos have same root: %s (%s) and %s (%s)",
					other.Name, other.Root, mr.Name, mr.Root)
			}
			if isChildPath(mr.Root, other.Root) {
				return fmt.Errorf("monorepo is contained by another monorepo: %s (%s) contains %s (%s)",
					other.Name, other.Root, mr.Name, mr.Root)
			}
		}
	}
	return nil
}

func isRelativePath(path string) bool {
	if path == "" {
		return true
	}
	if strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return false
	}
	return true
}

// isChildOf returns true if |child| is indeed a directory contained by |parent|.
func isChildPath(parent string, child string) bool {
	// The parent should at least be the common part plus a '/' and something else
	if len(child) < len(parent)+2 {
		return false
	}
	// We compare the |parent| part which should be the same.
	if child[:len(parent)] != parent {
		return false
	}
	// If |child| is indeed underneath, now it would have to have a separator to
	// represent the sub-directory. Depending on the type of path (local-system one or repo
	// relative, the separator could be different).t
	return child[len(parent)] == '/' || child[len(parent)] == '\\'
}
