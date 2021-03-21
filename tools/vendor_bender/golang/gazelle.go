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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/tools/vendor_bender/bazel"
)

func GazellePkg(root, pkgDir, importPath string, clean, verbose bool) error {
	p4 := p4lib.New()
	if newFile, err := bazel.EnsureWorkspaceFile(pkgDir); err != nil {
		return err
	} else if newFile {
		if _, err := p4.Add([]string{filepath.Join(pkgDir, "WORKSPACE")}); err != nil {
			return err
		}
	}
	// p4 edit and potentially delete any existing BUILD files.
	buildFiles := map[string]bool{}
	if err := filepath.Walk(pkgDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !isValidBuildFile(p) {
			return nil
		}
		buildFiles[p] = true
		if _, err := p4.Edit([]string{p}, 0); err != nil {
			return err
		}
		if clean {
			if err := os.Remove(p); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return err
	}

	if err := runGazelle(root, pkgDir, importPath, verbose); err != nil {
		return err
	}

	// Add new BUILD files
	if err := filepath.Walk(pkgDir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !isValidBuildFile(p) {
			return nil
		}
		// No need to p4 add preexisting file.
		if _, ok := buildFiles[p]; ok {
			return nil
		}
		if _, err := p4.Add([]string{p}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return err
	}

	// Delete removed files.
	for bf := range buildFiles {
		if files.FileExists(bf) {
			continue
		}
		// Turn the p4 edit into a p4 delete
		if _, err := p4.Revert([]string{bf}); err != nil {
			return err
		}
		if _, err := p4.Delete([]string{bf}, 0); err != nil {
			return err
		}
	}

	return nil
}

func isValidBuildFile(p string) bool {
	if !files.FileExists(p) {
		return false
	}
	// Skip file paths with p4 characters.
	if strings.Contains(p, "@") {
		return false
	}
	name := filepath.Base(p)
	return name == "BUILD" || name == "BUILD.bazel"
}

func runGazelle(mrRoot, pkg, importPath string, verbose bool) error {
	gazelleExe := filepath.Join(mrRoot, "bin/windows/gazelle.exe")
	args := []string{
		"update",
		fmt.Sprintf("-go_prefix=%s", importPath),
		"-index=false",
		"-go_naming_convention=import_alias",
		"-go_naming_convention_external=import",
		"-lang=go,proto",
	}
	com := exec.Command(gazelleExe, args...)
	com.Dir = pkg
	var stdoutBuf, stderrBuf bytes.Buffer
	if verbose {
		fmt.Println(com)
		com.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
		com.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	} else {
		com.Stdout = bufio.NewWriter(&stdoutBuf)
		com.Stderr = bufio.NewWriter(&stderrBuf)
	}
	if err := com.Run(); err != nil {
		return fmt.Errorf("gazelle update failed: %v", err)
	}
	return nil
}
