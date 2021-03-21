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

// Binary gazelle_fixer applies gazelle fix, including necessary flags and p4 operations.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/p4lib"
)

func internalMain() error {
	flag.Parse()
	if flag.NArg() < 1 {
		return errors.New("no package to fix")
	}
	p4 := p4lib.New()

	// cd to the monorepo root, gazelle wants to be run from there.
	monorepoMarker, err := p4.Where("//sge/MONOREPO")
	if err != nil {
		return fmt.Errorf("could not locate monorepo root: %v", err)
	}
	pwd := filepath.Dir(monorepoMarker)
	if err := os.Chdir(pwd); err != nil {
		return err
	}

	pkg := flag.Arg(0)
	p4HaveBuildFile, err := buildFileP4Have(p4, pkg)
	if err != nil {
		return err
	}

	// p4 edit the BUILD file if it exists.
	buildFile, exists := findBuildFile(pkg)
	if exists && p4HaveBuildFile {
		if _, err := p4.Edit([]string{buildFile}, 0); err != nil {
			return err
		}
	}
	cmd := exec.Command("bin/windows/gazelle.exe", "-r=false", pkg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	// p4 add the BUILD file to the default changelist if it didn't exist but does now.
	buildFile, exists = findBuildFile(pkg)
	if exists && !p4HaveBuildFile {
		if _, err := p4.Add([]string{buildFile}); err != nil {
			return err
		}
	}
	return nil
}

func buildFileP4Have(p4 p4lib.P4, pkg string) (bool, error) {
	buildFile := path.Join(pkg, "BUILD")
	buildBazelFile := path.Join(pkg, "BUILD.bazel")
	fs, err := p4.Have(buildFile, buildBazelFile)
	if err != nil {
		return false, err
	}
	return len(fs) > 0, nil
}

func findBuildFile(pkg string) (string, bool) {
	buildFile := path.Join(pkg, "BUILD")
	buildBazelFile := path.Join(pkg, "BUILD.bazel")
	if files.FileExists(buildFile) {
		return buildFile, true
	}
	if files.FileExists(buildBazelFile) {
		return buildBazelFile, true
	}
	return "", false
}

func main() {
	if err := internalMain(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
