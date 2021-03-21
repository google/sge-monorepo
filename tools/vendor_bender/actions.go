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
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/tools/vendor_bender/bazel"
	"sge-monorepo/tools/vendor_bender/git"
	"sge-monorepo/tools/vendor_bender/golang"
	"sge-monorepo/tools/vendor_bender/protos/licensepb"
	"sge-monorepo/tools/vendor_bender/protos/manifestpb"
	"sge-monorepo/tools/vendor_bender/protos/metadatapb"
	"sge-monorepo/tools/vendor_bender/rust"
	"sge-monorepo/tools/vendor_bender/zip"

	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/encoding/prototext"
)

// detectLicense returns the appropriate METADATA existing or to be
func getMdFilePath(path string) string {
	mdFilePath := filepath.Join(path, "METADATA")
	if stat, err := os.Stat(mdFilePath); err == nil {
		if stat.Mode().IsDir() {
			return filepath.Join(path, "METADATA.textpb")
		}
	}
	return mdFilePath
}

// byNumOfKeyPhrasesDesc implements sort.Interface based on the number of key phrases in a descending order
type byNumOfKeyPhrasesDesc []*licensepb.License

func (a byNumOfKeyPhrasesDesc) Len() int { return len(a) }
func (a byNumOfKeyPhrasesDesc) Less(i, j int) bool {
	return len(a[i].GetKeyPhrases()) > len(a[j].GetKeyPhrases())
}
func (a byNumOfKeyPhrasesDesc) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

// detectLicense returns the license info, return nil if LICENSE doesn't exist or is not in database.
func detectLicense(licenseDatabasePath, path string) (*metadatapb.License, error) {
	pkgLicenseFile := filepath.Join(path, "LICENSE")
	// try to match LICENSE file with different ext, eg. "LICENSE", "LICENSE.txt", "LICENSE.md" and etc
	matches, err := filepath.Glob(pkgLicenseFile + "*")
	if err != nil || matches == nil {
		return nil, fmt.Errorf("failed to find LICENSE file")
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("more than 1 LICENSE files were found")
	}
	pkgLicense, err := ioutil.ReadFile(matches[0])
	if err != nil {
		return nil, fmt.Errorf("failed to read LICENSE file %s: %v", matches[0], err)
	}
	// load license database
	licenseDatabaseIn, err := ioutil.ReadFile(licenseDatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read LicenseDatabase file %s: %v", licenseDatabasePath, err)
	}
	licenseDatabase := &licensepb.LicenseDatabase{}
	if err := proto.UnmarshalText(string(licenseDatabaseIn), licenseDatabase); err != nil {
		return nil, fmt.Errorf("failed to unmarshal LicenseDatabase file %s: %v", licenseDatabasePath, err)
	}
	licenses := licenseDatabase.GetLicenses()
	sort.Sort(byNumOfKeyPhrasesDesc(licenses))
	// find the match license by key phrase
	newline := regexp.MustCompile("\r?\n")
	pkgLicenseWithoutNewLine := newline.ReplaceAllString(string(pkgLicense), " ")
	pkgLicenseInfo := &metadatapb.License{}
	var findLicense bool
	for _, license := range licenses {
		findLicense = true
		keyPhrases := license.GetKeyPhrases()
		for _, keyPhrase := range keyPhrases {
			if !strings.Contains(pkgLicenseWithoutNewLine, keyPhrase) {
				findLicense = false
				break
			}
		}
		if findLicense {
			pkgLicenseInfo.Type = license.GetType()
			pkgLicenseInfo.Name = license.GetName()
			break
		}
	}
	if !findLicense {
		return nil, fmt.Errorf("failed to match the LICENSE %s in LicenseDatabase", matches[0])
	}
	return pkgLicenseInfo, nil
}

// UpdateMetadataFile updates METADATA file for the vendored package.
func updateMetadataFile(mrRoot, path string, localRepo bool, metadata *metadatapb.Metadata) error {
	mdFilePath := getMdFilePath(path)
	// detect license info
	licenseDatabasePath := filepath.Join(mrRoot, "tools/vendor_bender/LicenseDatabase")
	license, err := detectLicense(licenseDatabasePath, path)
	if err != nil {
		fmt.Printf("WARNING: LICENSE detection failed: %v\n", err)
	}
	// Update vendoring date
	now := time.Now()
	date := &metadatapb.Date{
		Year:  int32(now.Year()),
		Month: int32(now.Month()),
		Day:   int32(now.Day()),
	}
	metadata.ThirdParty.License = license
	metadata.ThirdParty.IsLocalRepository = localRepo
	metadata.ThirdParty.LastUpgradeDate = date
	// Write the info into METADATA
	mo := prototext.MarshalOptions{Multiline: true}
	buf, err := mo.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal the METADATA file: %v", err)
	}
	if err := ioutil.WriteFile(mdFilePath, buf, 0666); err != nil {
		return fmt.Errorf("failed to update METADATA file %s: %v", mdFilePath, err)
	}
	return nil
}

// createCl creates a CL containing changes to a given path.
func createCl(root, pkgName, pkgPath string) error {
	p4 := p4lib.New()
	cl, err := p4.Change(fmt.Sprintf("vendor %s into %s", pkgName, pkgPath))
	if err != nil {
		return fmt.Errorf("p4 changelist creation failed: %v", err)
	}
	if _, err := p4.Reconcile([]string{filepath.Join(pkgPath, "...")}, cl); err != nil {
		return fmt.Errorf("p4 add failed: %v", err)
	}
	if _, err := p4.Revert([]string{filepath.Join(pkgPath, "...")}, "-a", "-c", strconv.Itoa(cl)); err != nil {
		return fmt.Errorf("p4 revert unchanged failed: %v", err)
	}
	return nil
}

// Used when detecting a divergence file
const divergenceTag = "SGE_START"

// diffDivergences let the users manually diff the diverged files in the cl
func diffDivergences() error {
	p4 := p4lib.New()
	openedFiles, err := p4.Opened("")
	if err != nil {
		return err
	}
	first := true
	for _, openedFile := range openedFiles {
		if openedFile.Type == p4lib.FileTypeText {
			fileContent, err := p4.Print(openedFile.Path)
			if err != nil {
				return err
			}
			if strings.Contains(fileContent, divergenceTag) {
				if first {
					fmt.Println("The following opened file(s) has divergences in the existing version:")
				}
				fmt.Println(openedFile.Path)
				p4.DiffFile(openedFile.Path)
			}
		}
	}
	return nil
}

// syncPackage downlaods a package depending on its shource and handles
func syncPackage(ctx *context, pkgPath, oldPkgPath, repoDeclPath string, entry *manifestpb.ManifestEntry) error {
	tempDir, err := ioutil.TempDir("", "vendor_bender_*")
	if err != nil {
		return fmt.Errorf("couldn't create temp dir %v", err)
	}
	fmt.Println("created dir", tempDir)
	defer os.RemoveAll(tempDir)

	tempDir = path.Join(tempDir, entry.Name)
	var metadata *metadatapb.Metadata
	if entry.GetGitSrc() != nil {
		metadata, err = git.GitPkg(ctx.mrRoot, entry.Name, entry.GetGitSrc().Url, entry.GetGitSrc().Commit, tempDir, ctx.verbose)
	} else if entry.GetGoSrc() != nil {
		metadata, err = golang.GoPkg(ctx.mrRoot, entry.Name, entry.GetGoSrc().ImportPath, entry.GetGoSrc().Version, tempDir, entry.GetGoSrc().IgnoreGazelle, ctx.verbose)
	} else if entry.GetZipSrc() != nil {
		metadata, err = zip.ZipPkg(entry.Name, entry.GetGitSrc().Url, entry.GetGitSrc().Commit, tempDir)
	} else if entry.GetRustSrc() != nil {
		metadata, err = rust.RustPkg(entry.Name, entry.GetRustSrc().VersionSpec, tempDir, ctx.verbose)
	}
	if err != nil {
		return fmt.Errorf("failed to get package %s: %v", entry.Name, err)
	}
	if err := updateMetadataFile(ctx.mrRoot, tempDir, entry.SetAsLocalRepository, metadata); err != nil {
		return err
	}
	if files.DirExists(oldPkgPath) {
		for _, file := range entry.ForeignFile {
			if err := files.Copy(path.Join(oldPkgPath, file), path.Join(tempDir, file)); err != nil {
				return fmt.Errorf("failed to copy over foreign files from existing package in \"%s\"", pkgPath)
			}
		}
		if err := os.RemoveAll(oldPkgPath); err != nil {
			return fmt.Errorf("failed to delete existing package at \"%s\"", pkgPath)
		}
	}
	if !entry.SetAsLocalRepository {
		replacements, err := bazel.CollectLabelReplacements(ctx.pkgWsPaths, tempDir, ctx.pkgWsPaths[entry.Name])
		if err != nil {
			return fmt.Errorf("failed to perform ")
		}
		bazel.ApplyLabelReplacements(replacements, "")
	} else {
		if _, err := bazel.EnsureWorkspaceFile(tempDir); err != nil {
			return err
		}
	}
	if err := files.CopyDir(tempDir, pkgPath); err != nil {
		return fmt.Errorf("failed to copy temporary package sync location to its final destinaltion (\"%s\" -> \"%s\", reason: %v)", tempDir, pkgPath, err)
	}
	if err := createCl(ctx.mrRoot, entry.Name, pkgPath); err != nil {
		return fmt.Errorf("failed to create Changelist for the change. %q %q", tempDir, pkgPath)
	}
	if repoDeclPath != "" {
		importPath := ""
		if entry.GetGoSrc() != nil {
			importPath = entry.GetGoSrc().ImportPath
		}
		relRepoPath, err := filepath.Rel(ctx.mrRoot, pkgPath)
		if err != nil {
			return fmt.Errorf("failed to resolve the relative path from the root to the package %q -> %q: %v", ctx.mrRoot, pkgPath, err)
		}
		err = bazel.ModifyRepoDecl(repoDeclPath, relRepoPath, entry.Name, importPath)
		if err != nil {
			return err
		}
	}

	return nil
}

type pkgAdd struct {
	entry        *manifestpb.ManifestEntry
	path         string
	repoDeclPath string
}

func (pkgAdd *pkgAdd) print(ctx *context) {
	fmt.Printf("- Adding package %s in %s", pkgAdd.entry.Name, pkgAdd.path)
}

func (pkgAdd *pkgAdd) run(ctx *context) error {
	return syncPackage(ctx, pkgAdd.path, pkgAdd.path, pkgAdd.repoDeclPath, pkgAdd.entry)
}

type pkgDelete struct {
	path string
}

func (pkgDelete *pkgDelete) print(ctx *context) {
	fmt.Printf("- Deleting package at \"%s\"\n", pkgDelete.path)
}

func (pkgDelete *pkgDelete) run(ctx *context) error {
	p4 := p4lib.New()
	cl, err := p4.Change(fmt.Sprintf("deleteing package at %s", pkgDelete.path))
	if err != nil {
		return fmt.Errorf("p4 changelist creation failed: %v", err)
	}
	if _, err := p4.Delete([]string{path.Join(pkgDelete.path, "...")}, cl); err != nil {
		return fmt.Errorf("failed to p4 delete package at \"%s\"", pkgDelete.path)
	}
	if err := os.RemoveAll(pkgDelete.path); err != nil {
		return fmt.Errorf("failed to delete package at \"%s\"", pkgDelete.path)
	}
	return nil
}

type pkgUpdate struct {
	entry   *manifestpb.ManifestEntry
	path    string
	oldPath string
}

func (pkgUpdate *pkgUpdate) print(ctx *context) {
	fmt.Printf("- Updating package at \"%s\"\n", pkgUpdate.path)
}

func (pkgUpdate *pkgUpdate) run(ctx *context) error {
	return syncPackage(ctx, pkgUpdate.path, pkgUpdate.oldPath, "", pkgUpdate.entry)
}

type labelUpdate struct {
	replacements []bazel.LabelReplacement
	wsFile       string
}

func (labelUpdate *labelUpdate) print(ctx *context) {
	for _, replacement := range labelUpdate.replacements {
		fmt.Printf("- Label replacements on file \"%s\"\n", replacement.File)
	}
}

func (labelUpdate *labelUpdate) run(ctx *context) error {
	return bazel.ApplyLabelReplacements(labelUpdate.replacements, labelUpdate.wsFile)
}

type bazelRegen struct {
	entry *manifestpb.ManifestEntry
	path  string
	clean bool
}

func (bazelRegen *bazelRegen) print(ctx *context) {
	fmt.Printf("- Generating bazel files for \"%s\"\n", bazelRegen.entry.Name)
}

func (bazelRegen *bazelRegen) run(ctx *context) error {
	if bazelRegen.entry.GetGoSrc() != nil {
		if err := golang.GazellePkg(ctx.mrRoot, bazelRegen.path, bazelRegen.entry.GetGoSrc().ImportPath, bazelRegen.clean, ctx.verbose); err != nil {
			return fmt.Errorf("failed to generate bazel files for %s: %v", bazelRegen.path, err)
		}
	}
	return nil
}
