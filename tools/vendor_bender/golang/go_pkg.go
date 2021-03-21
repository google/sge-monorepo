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

package golang

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"sge-monorepo/tools/vendor_bender/bazel"
	"sge-monorepo/tools/vendor_bender/protos/metadatapb"
)

var nonWordRe = regexp.MustCompile(`\W+`)

// modVersion looks up information about a module at a given version.
// The path must be the module path, not a package within the module.
// The version may be a canonical semantic version, a query like "latest",
// or a branch, tag, or revision name. ModVersion returns the name of
// the repository rule providing the module (if any), the true version,
// and the sum.
func ModVersion(mrRoot, modPath, query string, verbose bool) (version, sum string, err error) {
	goTool := filepath.Join(mrRoot, "third_party/toolchains/go/1.14.3/bin/go.exe")
	cmd := exec.Command(goTool, "mod", "download", "-json", "--", modPath+"@"+query)
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	var stdoutBuf, stderrBuf bytes.Buffer
	if verbose {
		fmt.Println(cmd)
		cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
		cmd.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	} else {
		cmd.Stdout = bufio.NewWriter(&stdoutBuf)
		cmd.Stderr = bufio.NewWriter(&stderrBuf)
	}
	if err := cmd.Run(); err != nil {
		return "", "", err
	}
	var result struct{ Version, Sum string }
	if err := json.Unmarshal(stdoutBuf.Bytes(), &result); err != nil {
		return "", "", nil
	}
	return result.Version, result.Sum, nil
}

// importPathToBazelRepoName converts a Go import path into a bazel repo name
// following the guidelines in http://bazel.io/docs/be/functions.html#workspace
func ImportPathToBazelRepoName(importpath string) string {
	importpath = strings.ToLower(importpath)
	components := strings.Split(importpath, "/")
	labels := strings.Split(components[0], ".")
	reversed := make([]string, 0, len(labels)+len(components)-1)
	for i := range labels {
		l := labels[len(labels)-i-1]
		reversed = append(reversed, l)
	}
	repoName := strings.Join(append(reversed, components[1:]...), ".")
	return nonWordRe.ReplaceAllString(repoName, "_")
}

func GoPkg(mrRoot, name, importPath, version, dst string, ignoreGazelle, verbose bool) (*metadatapb.Metadata, error) {
	version, sum, err := ModVersion(mrRoot, importPath, version, verbose)
	if err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("could not locate go module for importPath %s: %v", importPath, err)
	}
	fetchRepoExe := filepath.Join(mrRoot, "bin/windows/fetch_repo.exe")
	if err := os.Mkdir(dst, 0666); err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("failed to create target folder %s", dst)
	}
	cmdArgs := []string{
		fmt.Sprintf("-dest=%s", dst),
		fmt.Sprintf("-importpath=%s", importPath),
		fmt.Sprintf("-version=%s", version),
		fmt.Sprintf("-sum=%s", sum),
	}
	cmd := exec.Command(fetchRepoExe, cmdArgs...)
	if verbose {
		fmt.Println(cmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	goRoot := filepath.Join(mrRoot, "third_party/toolchains/go/1.14.3")
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOROOT=%s", goRoot))

	if err := cmd.Run(); err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("fetch_repo failed: %v", err)
	}
	if !ignoreGazelle {
		if _, err := bazel.EnsureWorkspaceFile(dst); err != nil {
			return &metadatapb.Metadata{}, err
		}
		if err := runGazelle(mrRoot, dst, importPath, verbose); err != nil {
			return &metadatapb.Metadata{}, err
		}
	}

	metadata := &metadatapb.Metadata{
		Name: name,
		ThirdParty: &metadatapb.ThirdParty{
			Source: &metadatapb.Source{
				GoPkg: &metadatapb.GoPkg{
					Importpath: importPath,
					Version:    version,
				},
			},
		},
	}

	return metadata, nil
}
