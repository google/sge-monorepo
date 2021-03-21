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

// Package workspace contains utility functions for vendor-bendor commands.
package bazel

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"sge-monorepo/libs/go/p4lib"
)

type LabelReplacement struct {
	File         string
	Replacements []string
}

// Used when modifying the global workspace file
const workspaceFile = "WORKSPACE"
const wokspaceRepoDeclFmt = `
local_repository(
    name = "%s",
    path = "%s",
)
`

// Used when modifying a language repo file
const localRepoFile = "local_repositories.bzl"
const langRepoDeclFmt = `
    native.local_repository(
        name = "%s",
        path = "%s",
    )
`

// Gets the target file where a local_repository entry is going to be added.
func RepoDeclFile(root, repoPath, repoName string) (string, error) {
	declFile := filepath.Join(root, workspaceFile)
	declBytes, err := ioutil.ReadFile(declFile)
	if err != nil {
		return "", fmt.Errorf("could not read repository declaration file %s: %v", declFile, err)
	}
	if strings.Contains(string(declBytes), "\""+repoName+"\"") {
		fmt.Println("Workspace already contains a repo having the same name: ", repoName)
		return "", fmt.Errorf("workspace already contains a repo having the same name: %s", repoName)
	}
	// If we have a language defined (except for go and bzl), we need to use the local_repositories.bzl for it

	localRepoDeclFile := filepath.Clean(filepath.Join(repoPath, "..", localRepoFile))
	declBytes, err = ioutil.ReadFile(localRepoDeclFile)
	if err == nil {
		if strings.Contains(string(declBytes), "\""+repoName+"\"") {
			return "", fmt.Errorf("local repository file already contains a repo having the same name: %s", repoName)
		}
		declFile = localRepoDeclFile
	}
	return declFile, nil
}

const repoDeclDescription = "Vendor bender repository declarations changes"

func ModifyRepoDecl(declFile, repoPath, repoName, importPath string) error {
	declFmt := wokspaceRepoDeclFmt
	if !strings.HasSuffix(declFile, workspaceFile) {
		declFmt = langRepoDeclFmt
	}
	declBytes, err := ioutil.ReadFile(declFile)
	if err != nil {
		return fmt.Errorf("could not read repository declaration file %s: %v", declFile, err)
	}
	content := string(declBytes)
	if importPath != "" {
		content += fmt.Sprintf("\n# gazelle:repository go_repository name=%s importpath=%s", repoName, importPath) + fmt.Sprintf(declFmt, repoName, repoPath)
	} else {
		content += fmt.Sprintf(declFmt, repoName, repoPath)
	}
	p4 := p4lib.New()
	client, err := p4.Client("")
	if err != nil {
		return fmt.Errorf("failed to get current client")
	}
	pcls, err := p4.Changes("-c", client.Client, "-s", "pending", "-L")
	cl := 0
	for _, pcl := range pcls {
		if strings.Contains(pcl.Description, repoDeclDescription) {
			cl = pcl.Cl
			break
		}
	}
	if cl == 0 {
		cl, err = p4.Change(repoDeclDescription)
		if err != nil {
			return fmt.Errorf("failed to create a new cl for repository declarations")
		}
	}
	_, err = p4.Edit([]string{declFile}, cl)
	if err != nil {
		return fmt.Errorf("failed to edit repository declaration file %s: %v", declFile, err)
	}
	if err = ioutil.WriteFile(declFile, []byte(content), 0666); err != nil {
		return fmt.Errorf("could not write repository declaration file %s: %v", declFile, err)
	}
	return nil
}

// EnsureWorkspaceFile adds a WORKSPACE file to the directory if one doesn't exist.
// Returns whether the file was created.
func EnsureWorkspaceFile(dir string) (bool, error) {
	wsFilePath := filepath.Join(dir, "WORKSPACE")
	if _, err := os.Stat(wsFilePath); err == nil {
		return false, nil
	}
	wsFile, err := os.Create(wsFilePath)
	if err != nil {
		return false, err
	}
	if err = wsFile.Close(); err != nil {
		return false, err
	}
	return true, nil
}

var labelRe = regexp.MustCompile(`@([A-Za-z0-9\-._]+)(?:\/\/)((?:[A-Za-z0-9/\-._])*(?::[A-Za-z0-9!%_@^#$&'()*\-+,;<=>?[\]{|}~/.]+)?)`)

func collectBazelFiles(dir string, files *[]string) error {
	children, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, child := range children {
		if child.IsDir() {
			if err = collectBazelFiles(path.Join(dir, child.Name()), files); err != nil {
				return err
			}
		} else {
			if child.Name() == "BUILD" || child.Name() == "BUILD.bazel" || path.Ext(child.Name()) == ".bzl" {
				*files = append(*files, path.Join(dir, child.Name()))
			}
		}
	}
	return nil
}

func CollectLabelReplacements(pkgWsPaths map[string]string, path, repoPath string) ([]LabelReplacement, error) {
	var localLabels [][]string
	if len(repoPath) != 0 {
		children, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, err
		}
		localLabels = append(localLabels, []string{"//:", "//" + repoPath + ":"})
		for _, child := range children {
			if child.IsDir() {
				localLabels = append(localLabels, []string{"//" + child.Name(), "//" + repoPath + "/" + child.Name()})
			}
		}
	}
	var files []string
	collectBazelFiles(path, &files)
	var replacements []LabelReplacement
	for _, file := range files {
		in, err := ioutil.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read file %s: %v", file, err)
		}
		content := string(in)
		var fileReplacements []string
		for _, matches := range labelRe.FindAllStringSubmatch(content, len(content)) {
			if len(matches) != 3 {
				return nil, fmt.Errorf("internal error, expected 3 matches, found %d", len(matches))
			}
			if val, ok := pkgWsPaths[matches[1]]; ok {
				sep := ""
				if len(matches[2]) != 0 && matches[2][0] != ':' {
					sep = "/"
				}
				fileReplacements = append(fileReplacements, []string{
					matches[0],
					fmt.Sprintf("%s%s%s", val, sep, matches[2]),
				}...)
			}
		}
		for _, localLabel := range localLabels {
			if strings.Contains(content, localLabel[0]) {
				fileReplacements = append(fileReplacements, []string{
					localLabel[0],
					localLabel[1],
				}...)
			}
		}
		if len(fileReplacements) != 0 {
			replacements = append(replacements, LabelReplacement{file, fileReplacements})
		}
	}
	return replacements, nil
}

func ApplyLabelReplacements(replacements []LabelReplacement, wsFile string) error {
	cl := 0
	p4 := p4lib.New()
	if wsFile != "" {
		newCl, err := p4.Change(fmt.Sprintf("Removing %s", wsFile))
		if err != nil {
			return fmt.Errorf("failed to create a new cl for labels replacement")
		}
		cl = newCl
	}
	for _, fileReplacements := range replacements {
		buf, err := ioutil.ReadFile(fileReplacements.File)
		if err != nil {
			return fmt.Errorf("failed to read file %s: %v", fileReplacements.File, err)
		}
		r := strings.NewReplacer(fileReplacements.Replacements...)
		if cl != 0 {
			p4.Edit([]string{fileReplacements.File}, cl)
			if err != nil {
				return fmt.Errorf("failed to edit file for label replacements %s: %v", fileReplacements.File, err)
			}
		}
		ioutil.WriteFile(fileReplacements.File, []byte(r.Replace(string(buf))), 0666)
	}
	if cl != 0 {
		p4.Delete([]string{wsFile}, cl)
	}
	return nil
}
