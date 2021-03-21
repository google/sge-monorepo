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

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"sge-monorepo/tools/vendor_bender/git"
	"sge-monorepo/tools/vendor_bender/golang"
	"sge-monorepo/tools/vendor_bender/protos/manifestpb"
)

func detectDivergences(path string) {
	filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		if info.IsDir() {
			return nil
		}
		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		str := string(bytes)
		if utf8.ValidString(str) && strings.Contains(str, "SGE_START") {
			fmt.Printf("Warning: divergence detected in file: %s\n", path)
		}
		return nil
	})
}

func validateManifestEntry(mrRoot string, entry *manifestpb.ManifestEntry, verbose bool) error {
	if entry.GetGoSrc() != nil && entry.Name != "bazel_gazelle" {
		repoName := golang.ImportPathToBazelRepoName(entry.GetGoSrc().ImportPath)
		if entry.Name != repoName {
			return fmt.Errorf("the name of the repo should be changed to %s instead of %s", repoName, entry.Name)
		}
		if entry.GetGoSrc().Version == "latest" {
			version, _, err := golang.ModVersion(mrRoot, entry.GetGoSrc().ImportPath, entry.GetGoSrc().Version, verbose)
			if err != nil {
				return fmt.Errorf("failed to get corresponding version to latest for %s: %v", entry.GetGoSrc().ImportPath, err)
			}
			return fmt.Errorf("the version for %s should be changed to %s to proceed", entry.GetGoSrc().ImportPath, version)
		}
	} else if entry.GetGitSrc() != nil && entry.GetGitSrc().Commit == "" {
		gitSrc := entry.GetGitSrc()
		sha, err := git.CommitFromRef(mrRoot, gitSrc.GetUrl(), gitSrc.Ref, verbose)
		if err != nil {
			return fmt.Errorf("a commit field is mandatory, couldn't deduct the sha1 for the commit %v", err)
		} else {
			return fmt.Errorf("a commit field is mandatory, please add a commit field with the following hash to %s, %s", entry.Name, sha)
		}
	}
	return nil
}

func needsUpdate(pkg *pkg, entry *manifestpb.ManifestEntry) bool {
	if pkg.metatada.ThirdParty == nil || pkg.metatada.ThirdParty.Source == nil {
		return true
	}
	if entry.SetAsLocalRepository && !pkg.metatada.ThirdParty.IsLocalRepository {
		return true
	}
	mdSource := pkg.metatada.ThirdParty.Source
	if mdSource.GitPkg != nil && entry.GetGitSrc() != nil {
		if pkg.metatada.ThirdParty.Source.GitPkg.Sha == entry.GetGitSrc().Commit {
			return false
		}
	} else if mdSource.GoPkg != nil && entry.GetGoSrc() != nil {
		if mdSource.GoPkg.Version == entry.GetGoSrc().Version &&
			mdSource.GoPkg.Importpath == entry.GetGoSrc().ImportPath {
			return false
		}
	} else if mdSource.ZipPkg != nil && entry.GetZipSrc() != nil {
		if mdSource.ZipPkg.Version == entry.GetZipSrc().Version {
			return false
		}
	} else if mdSource.RustPkg != nil && entry.GetRustSrc() != nil {
		if mdSource.RustPkg.Version == mdSource.GetRustPkg().Version {
			return false
		}
	}
	return true
}
