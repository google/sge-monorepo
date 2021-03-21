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

package packagemanifest

import (
	"fmt"
	"io/ioutil"
	"sort"

	"sge-monorepo/build/cicd/sgeb/buildtool"

	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
	"sge-monorepo/build/packagemanifest/protos/packagemanifestpb"

	"github.com/golang/protobuf/proto"
)

const packageManifestTag = "packagemanifest"

// NewBuilder returns a package manifest builder.
func NewBuilder() *Builder {
	return &Builder{map[string]string{}}
}

// Builder assists in constructing package manifests.
type Builder struct {
	entries map[string]string
}

// Add adds an entry to the package manifest.
// Returns an error on duplicate entries.
func (b *Builder) Add(pkgPath, filePath string) error {
	if _, ok := b.entries[pkgPath]; ok {
		return fmt.Errorf("duplicate package path: %q", pkgPath)
	}
	b.entries[pkgPath] = filePath
	return nil
}

// WritePkgManifestArtifact writes and returns artifact suitable for consumption by a dependency.
func (b *Builder) WritePkgManifestArtifact(p string) (*buildpb.Artifact, error) {
	msg := b.Build()
	if err := ioutil.WriteFile(p, []byte(proto.MarshalTextString(msg)), 066); err != nil {
		return nil, fmt.Errorf("could not write package manifest : %v", err)
	}
	return &buildpb.Artifact{
		Tag: packageManifestTag,
		Uri: buildtool.LocalFileUri(p),
	}, nil
}

// Build returns a package manifest proto message.
func (b *Builder) Build() proto.Message {
	pkg := &packagemanifestpb.PackageManifest{}
	for pkgPath, srcPath := range b.entries {
		pkg.Entries = append(pkg.Entries, &packagemanifestpb.Entry{
			FilePath:    srcPath,
			PackagePath: pkgPath,
		})
	}
	sort.Slice(pkg.Entries, func(i, j int) bool {
		return pkg.Entries[i].PackagePath < pkg.Entries[j].PackagePath
	})
	return pkg
}

// NewPackageBuilder returns a builder that assists in building packages.
func NewPackageBuilder() *PackageBuilder {
	return &PackageBuilder{map[string]string{}}
}

// PackageBuilder assists in building packages.
type PackageBuilder struct {
	pkg map[string]string
}

// AddPkgPath adds an entry to the package.
// Returns an error on duplicate entries.
func (pb *PackageBuilder) AddPkgPath(pkgPath, filePath string) error {
	if _, ok := pb.pkg[pkgPath]; ok {
		return fmt.Errorf("duplicate package path: %q", pkgPath)
	}
	pb.pkg[pkgPath] = filePath
	return nil
}

// Returns whether the artifact is a package manifest.
func IsPkgArtifact(artifact *buildpb.Artifact) bool {
	return artifact.Tag == packageManifestTag
}

// AddPkgManifests loads a package manifest from disk and adds all its entries.
// Returns an error on duplicate entries.
func (pb *PackageBuilder) AddPkgManifest(p string) error {
	contents, err := ioutil.ReadFile(p)
	if err != nil {
		return fmt.Errorf("could not read package manifest %s: %v", p, err)
	}
	pkgManifest := &packagemanifestpb.PackageManifest{}
	if err := proto.UnmarshalText(string(contents), pkgManifest); err != nil {
		return fmt.Errorf("could not unmarshal package manifest %s: %v", p, err)
	}
	for _, e := range pkgManifest.Entries {
		if err := pb.AddPkgPath(e.PackagePath, e.FilePath); err != nil {
			return err
		}
	}
	return nil
}

// PkgEntry is a single entry in a package.
type PkgEntry struct {
	PkgPath  string
	FilePath string
}

// Entries returns a sorted list of package entries.
func (pb *PackageBuilder) Entries() []PkgEntry {
	var res []PkgEntry
	for pkgPath, filePath := range pb.pkg {
		res = append(res, PkgEntry{pkgPath, filePath})
	}
	sort.Slice(res, func(i, j int) bool {
		return res[i].PkgPath < res[j].PkgPath
	})
	return res
}
