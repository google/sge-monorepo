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

// Binary cl_publisher will take binaries created by bazel and publish them to a specified location.
package main

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"sge-monorepo/build/cicd/sgeb/buildtool"
	"sge-monorepo/build/packagemanifest"
	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/sgeflag"

	"sge-monorepo/build/cicd/sgeb/protos/buildpb"

	"github.com/golang/glog"
)

var flags = struct {
	name        string
	outDir      string
	change      string
	submitCl    bool
	description sgeflag.StringList
}{}

func main() {
	flag.StringVar(&flags.name, "name", "", "name to add to the CL description")
	flag.StringVar(&flags.outDir, "out_dir", "", "output directory relative to monorepo root")
	flag.StringVar(&flags.change, "c", "", "change to add files to. If omitted a new CL is created.")
	flag.BoolVar(&flags.submitCl, "submit_cl", false, "submits the CL after it is created")
	flag.Var(&flags.description, "desc", "additional lines of description to add to the CL")
	flag.Parse()
	glog.Info("application start")
	glog.Infof("%v", os.Args)

	err := publish()

	if err != nil {
		glog.Errorf("%v", err)
	}
	glog.Info("application exit")
	glog.Flush()

	if err != nil {
		os.Exit(1)
	}
}

func publish() error {
	p4 := p4lib.New()
	helper := buildtool.MustLoad()

	// Search input artifacts for regular input files and package manifests.
	var inputFiles []string
	var packages []string
	for _, inputs := range helper.Invocation().Inputs {
		for _, input := range inputs.Artifacts {
			p, ok := buildtool.ResolveArtifact(input)
			if !ok {
				continue
			}
			if packagemanifest.IsPkgArtifact(input) {
				packages = append(packages, p)
			} else {
				inputFiles = append(inputFiles, p)
			}
		}
	}
	// For all inputs, let's see where under the out_dir to place them.
	pkgBuilder := packagemanifest.NewPackageBuilder()
	// Without any other structure we place files at the root of cl_publisher's out_dir.
	for _, f := range inputFiles {
		if err := pkgBuilder.AddPkgPath(path.Base(f), f); err != nil {
			return err
		}
	}
	for _, pkg := range packages {
		if err := pkgBuilder.AddPkgManifest(pkg); err != nil {
			return err
		}
	}
	// Map to depot destination path.
	outDir, err := helper.ResolveBuildPath(flags.outDir)
	if err != nil {
		return fmt.Errorf("cannot resolve out_dir %q: %v", flags.outDir, err)
	}
	inputToDestPath := map[string]string{}
	for _, pkgEntry := range pkgBuilder.Entries() {
		inputToDestPath[pkgEntry.FilePath] = path.Join(outDir, pkgEntry.PkgPath)
	}
	// Verify that we can use perforce publishing.
	for _, destPath := range inputToDestPath {
		_, err := p4.Where(destPath)
		if err != nil {
			return fmt.Errorf("trying to use p4 on a local directory not in workspace: %s", destPath)
		}
	}
	// Do not bother publishing files that haven't changed.
	for f, destPath := range inputToDestPath {
		if eq, err := filesEqual(f, destPath); err != nil {
			return err
		} else if eq {
			// Skip file.
			delete(inputToDestPath, f)
		}
	}
	if len(inputToDestPath) == 0 {
		fmt.Println("no files changed, nothing to publish")
		helper.MustWritePublishResult(&buildpb.PublishInvocationResult{})
		return nil
	}

	// Try to update in place
	newFiles := map[string]string{}
	for src, dest := range inputToDestPath {
		ok, err := updateFile(p4, src, dest)
		if err != nil {
			return err
		}
		if !ok {
			newFiles[src] = dest
		}
	}
	if len(newFiles) > 0 {
		var change int
		switch flags.change {
		case "":
			var desc []string
			desc = append(desc, fmt.Sprintf("Publish %s to %s.", flags.name, flags.outDir))
			if len(flags.description) > 0 {
				desc = append(desc, "")
				for _, d := range flags.description {
					desc = append(desc, d)
				}
			}
			pubInv := helper.Invocation().PublishInvocation
			if pubInv.BaseCl != 0 || pubInv.CiResultUrl != "" {
				desc = append(desc, "")
			}
			if pubInv.BaseCl != 0 {
				desc = append(desc, fmt.Sprintf("Built from CL/%d", pubInv.BaseCl))
			}
			if pubInv.CiResultUrl != "" {
				desc = append(desc, fmt.Sprintf("CI results: %s", pubInv.CiResultUrl))
			}
			if change, err = p4.Change(strings.Join(desc, "\n")); err != nil {
				return fmt.Errorf("could not create changelist. %v", err)
			}
		case "default":
			change = 0
		default:
			if change, err = strconv.Atoi(flags.change); err != nil {
				return fmt.Errorf("invalid change %q: %v", flags.change, err)
			}
		}
		for src, dest := range newFiles {
			if err := publishFile(p4, change, src, dest); err != nil {
				if err := cleanupChange(p4, change); err != nil {
					fmt.Println(err)
				}
				return err
			}
		}
		if flags.submitCl {
			if _, err := p4.Submit(change); err != nil {
				if err := cleanupChange(p4, change); err != nil {
					fmt.Println(err)
				}
				return err
			}
		}
	}
	var publishedFiles []*buildpb.PublishedFile
	for f := range inputToDestPath {
		fileInfo, err := os.Stat(f)
		if err != nil {
			return fmt.Errorf("could not get size of file %s: %v", f, err)
		}
		publishedFiles = append(publishedFiles, &buildpb.PublishedFile{
			Size: fileInfo.Size(),
		})
	}
	var version string
	baseCl := helper.Invocation().PublishInvocation.BaseCl
	if baseCl != 0 {
		version = fmt.Sprintf("%d", baseCl)
	}
	result := &buildpb.PublishResult{
		Name:    flags.name,
		Version: version,
		Files:   publishedFiles,
	}
	helper.MustWritePublishResult(&buildpb.PublishInvocationResult{
		PublishResults: []*buildpb.PublishResult{result},
	})
	return nil
}

func updateFile(p4 p4lib.P4, srcPath, destPath string) (bool, error) {
	fstat, err := p4.Fstat(destPath)
	if err != nil {
		return false, nil
	}
	// If the file is open in any CL, workRev will be non-zero.
	if len(fstat.FileStats) == 0 || fstat.FileStats[0].WorkRev == 0 {
		return false, nil
	}
	if err = copyFile(srcPath, destPath); err != nil {
		return false, fmt.Errorf("could not copy binary: %v", err)
	}
	return true, nil
}

func publishFile(p4 p4lib.P4, change int, srcPath, destPath string) error {
	if _, err := p4.Edit([]string{destPath}, change); err != nil {
		return fmt.Errorf("p4 edit failed: %v", err)
	}
	glog.Infof("copying %s -> %s\n", srcPath, destPath)
	if err := copyFile(srcPath, destPath); err != nil {
		return fmt.Errorf("could not copy file: %v", err)
	}

	var args []string

	if change != 0 {
		args = append(args, "-c", strconv.Itoa(change))
	}

	if _, err := p4.Add([]string{destPath}, args...); err != nil {
		return fmt.Errorf("could not add %s to changelist: %v", destPath, err)
	}
	return nil
}

func cleanupChange(p4 p4lib.P4, change int) error {
	if _, err := p4.ExecCmd(fmt.Sprintf("revert -c %d //...", change)); err != nil {
		return fmt.Errorf("could not revert change %d: %v", change, err)
	}
	if _, err := p4.ExecCmd(fmt.Sprintf("change -d -f %d", change)); err != nil {
		return fmt.Errorf("could not delete change %d: %v", change, err)
	}
	return nil
}

func copyFile(srcFileName string, dstFileName string) error {
	dstDir := filepath.Dir(dstFileName)
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dstDir, os.ModePerm); err != nil {
			return err
		}
	}
	srcHandle, err := os.Open(srcFileName)
	if err != nil {
		return err
	}
	defer srcHandle.Close()
	dstHandle, err := os.Create(dstFileName)
	if err != nil {
		return err
	}
	defer dstHandle.Close()
	if _, err := io.Copy(dstHandle, srcHandle); err != nil {
		return err
	}
	return nil
}

func filesEqual(src, dest string) (bool, error) {
	if !files.FileExists(dest) {
		return false, nil
	}
	srcHash, err := hashFile(src)
	if err != nil {
		return false, err
	}
	destHash, err := hashFile(dest)
	if err != nil {
		return false, err
	}
	return bytes.Equal(srcHash, destHash), nil
}

func hashFile(p string) ([]byte, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}
