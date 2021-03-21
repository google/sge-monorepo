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
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"google.golang.org/genproto/googleapis/datastore/v1"
)

type PlatformName string

const (
	Windows PlatformName = "Win64"
)

// a package is a set of files specific to a product
type PackageEntry struct {
	State   string
	Version string
}

// FileEntry : one file in one package
type FileEntry struct {
	RelativePath string         `datastore:",noindex"`
	Hash         string         `datastore:",noindex"`
	Key          *datastore.Key `datastore:"__key__"`
}

// PackageConfig : structure of json config file

type PackageConfig struct {
	ProductName  string
	Placeholders map[string]string
	Folders      []string
}

type intResult struct {
	Value int
	Err   error
}

type stringResult struct {
	Value string
	Err   error
}

func help() {
	const helpString = `
Usage:
    conjob.exe <perforce-depot-path>
`
	fmt.Println(helpString)
}

func main() {
	log.Println("con-job starting")

	if len(os.Args) <= 1 {
		help()
		os.Exit(1)
	}

	for {
		if err := builder(os.Args[1]); err != nil {
			log.Println(err)
		}
	}

	log.Println("con-job exiting")
}

func timeFunction(fnName string, t time.Time) {
	log.Println(fnName, "took", time.Now().Sub(t))
}

func perforceCommand(args ...string) (string, error) {
	var strings []string
	strings = append(strings, "-C", "utf8-bom")
	strings = append(strings, args...)
	com := exec.Command("p4", strings...)
	stdOutErr, err := com.CombinedOutput()
	return string(stdOutErr), err
}

func getChangesInfo(path string) (int, error) {
	var stdOutErr string
	var err error

	if stdOutErr, err = perforceCommand("changes", "-m1", path); err != nil {
		return 0, err
	}

	lines := strings.Split(stdOutErr, "\n")
	for _, s := range lines {
		if strings.HasPrefix(s, "Change") {
			words := strings.Split(s, " ")
			return strconv.Atoi(words[1])
		}
	}
	return 0, fmt.Errorf("couldn't list p4 changes")
}

func getWhere(path string) (string, error) {
	var stdOutErr string
	var err error

	if stdOutErr, err = perforceCommand("where", path); err != nil {
		return stdOutErr, err
	}

	lines := strings.Split(string(stdOutErr), "\n")
	if len(lines) >= 1 {
		words := strings.Split(lines[0], " ")
		if len(words) >= 3 {
			return strings.TrimSpace(words[2]), nil
		}
	}

	return "", fmt.Errorf("couldn't determine location of %s", path)
}

func p4Sync(path string, cl int) (string, error) {
	defer timeFunction(fmt.Sprintf("p4 sync %d", cl), time.Now())
	return perforceCommand("sync", fmt.Sprint(path, "@", cl))
}

func getLatestSubmittedCl(path string) (int, error) {
	return getChangesInfo(path)
}

func getLatestHaveCl(path string) (int, error) {
	return getChangesInfo(path + "#have")
}

func builder(inputPath string) error {
	var err error

	path := inputPath + "/..."

	submittedClChan := make(chan intResult)
	go func() {
		cl, err := getLatestSubmittedCl(path)
		submittedClChan <- intResult{Value: cl, Err: err}
	}()

	haveClChan := make(chan intResult)
	go func() {
		cl, err := getLatestHaveCl(path)
		haveClChan <- intResult{Value: cl, Err: err}
	}()

	submittedClResult := <-submittedClChan
	haveClResult := <-haveClChan

	if submittedClResult.Err != nil {
		return submittedClResult.Err
	}
	submittedCl := submittedClResult.Value

	if haveClResult.Err != nil {
		return haveClResult.Err
	}
	haveCl := haveClResult.Value

	pkgFileName, err := getWhere(inputPath + "/build/editor-package.json")
	if err != nil {
		return err
	}

	if submittedCl > haveCl {
		log.Printf("syncing cl %d\n", submittedCl)
		if _, err := p4Sync(path, submittedCl); err != nil {
			return err
		}
		haveCl = submittedCl
		if err := buildAndCook(inputPath); err != nil {
			return err
		}
	}

	packageMap := make(map[int]bool)
	if packageMap, err = getDistList(inputPath, pkgFileName); err != nil {
		return err
	}

	var wg sync.WaitGroup
	var jobs []func()

	uploadResults := make(chan error, 2)
	if _, found := packageMap[haveCl]; !found {
		doit := func() {
			defer wg.Done()
			uploadResults <- unrealDistributor(inputPath)
		}
		jobs = append(jobs, doit)
	}

	if len(jobs) > 0 {
		for _, j := range jobs {
			wg.Add(1)
			go j()
		}
		wg.Wait()
		close(uploadResults)
		var errs []error
		for e := range uploadResults {
			if e != nil {
				errs = append(errs, e)
			}
		}
		if len(errs) > 0 {
			return fmt.Errorf("%v", errs)
		}

	} else {
		defer timeFunction("sleeping", time.Now())
		time.Sleep(1 * time.Minute)
	}

	return nil
}

func buildAndCook(inputPath string) error {
	jobs := []struct {
		platform PlatformName
		command  []string
	}{
		{
			platform: Windows,
			command:  []string{"build", "editor"},
		},
		{
			platform: Windows,
			command:  []string{"package"},
		},
	}

	for _, job := range jobs {
		if err := unrealBuilder(inputPath, job.platform, job.command...); err != nil {
			return err
		}
	}

	return nil
}

func unrealBuilder(inputPath string, platform PlatformName, commands ...string) error {
	defer timeFunction(fmt.Sprintf("unreal builder --platform %s %s", platform, fmt.Sprint(commands)), time.Now())
	builder_exe, err := getWhere(inputPath + "/build/unreal-builder.exe")
	if err != nil {
		return err
	}

	var strings []string
	strings = append(strings, "--platform", string(platform))
	strings = append(strings, commands...)
	com := exec.Command(builder_exe, strings...)
	com.Stdout = os.Stdout
	com.Stderr = os.Stderr
	err = com.Run()
	return err
}

func getDistList(path string, builder_pkg string) (map[int]bool, error) {
	packageMap := make(map[int]bool)
	builder_exe, err := getWhere(path + "/build/build-dist.exe")
	if err != nil {
		return packageMap, err
	}

	com := exec.Command(builder_exe, "list-packages", builder_pkg)
	stdOutErr, err := com.CombinedOutput()
	lines := strings.Split(string(stdOutErr), "\n")
	for _, s := range lines {
		words := strings.Fields(s)
		if len(words) > 1 {
			clAscii := strings.TrimSpace(words[0])
			if clValue, err := strconv.Atoi(clAscii); err == nil {
				packageMap[clValue] = true
			} else {
				fmt.Println("can't convert ascii to int *", clAscii, "*")
			}
		}
	}
	return packageMap, err
}

func formatPackageName(path string) string {
	packageName := strings.ReplaceAll(path, "//", "")
	packageName = strings.ReplaceAll(packageName, "/", "_")
	return packageName
}

func unrealDistributor(path string) error {
	defer timeFunction("unreal-dist", time.Now())
	builder_exe, err := getWhere(path + "/build/build-dist.exe")
	if err != nil {
		return err
	}

	builder_pkg, err := getWhere(path + "/build/editor-package.json")
	if err != nil {
		return err
	}

	log.Println("making package")
	com := exec.Command(builder_exe, "make-package", builder_pkg)
	com.Stdout = os.Stdout
	com.Stderr = os.Stderr
	return com.Run()
}
