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
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"sge-monorepo/build/cicd/presubmit/check"

	"sge-monorepo/build/cicd/sgeb/protos/buildpb"

	"github.com/google/go-cmp/cmp"
)

func checkGazelle(helper check.Helper) (bool, error) {
	totalSuccess := true
	c := helper.OnlyCheck()

	// Run on all directories with BUILD or .go file changes.
	dirs := map[string]bool{}
	for _, f := range c.Files {
		dir := path.Dir(f.Path)
		dirs[dir] = true
	}
	// Has the whole directory been deleted or is otherwise empty? Skip gazelle check in this case.
	for dir := range dirs {
		if hasFiles, err := dirHasFiles(dir); !hasFiles {
			delete(dirs, dir)
		} else if err != nil {
			return false, err
		}
	}
	toolBin, err := helper.ResolvePath("//bin/windows/gazelle.exe")
	if err != nil {
		return false, err
	}
	args := []string{"-r=false", "-mode=print"}
	for d := range dirs {
		args = append(args, d)
	}
	cmd := exec.Command(toolBin, args...)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err == nil {
		// Separate output. Each file it printed starting with a ">>> <file name>" line.
		buildFiles := map[string]*strings.Builder{}
		var buildFile *strings.Builder
		for _, line := range strings.Split(string(output), "\n") {
			if strings.HasPrefix(line, ">>> ") {
				f := line[4:]
				buildFile = &strings.Builder{}
				buildFiles[f] = buildFile
				continue
			}
			if buildFile != nil {
				buildFile.WriteString(line)
				buildFile.WriteString("\n")
			}
		}
		for f, buildFile := range buildFiles {
			_, err := os.Stat(f)
			if err != nil {
				result := &buildpb.Result{
					Name:    f,
					Success: false,
					Logs:    check.LogsFromString("stderr", "BUILD file missing, did you forget a p4 add?"),
				}
				helper.AddResult(result)
				continue
			}
			gotBytes, err := ioutil.ReadFile(f)
			gotString := clean(string(gotBytes))
			wantString := clean(buildFile.String())
			diff := cmp.Diff(gotString, wantString)
			log := strings.Builder{}
			if diff != "" {
				log.WriteString("unexpected diff\n")
				log.WriteString(diff)
				log.WriteString("\n")
			}
			success := log.Len() == 0
			totalSuccess = totalSuccess && success
			var fix string
			if !success {
				fixBin, err := helper.ResolvePath("//bin/windows/gazelle_fixer.exe")
				if err != nil {
					return false, err
				}
				fix = fmt.Sprintf("%s %s", fixBin, filepath.Dir(f))
			}
			result := &buildpb.Result{
				Name:    f,
				Success: success,
				Logs:    check.LogsFromString("stderr", log.String()),
				Fix:     fix,
			}
			helper.AddResult(result)
		}
	} else {
		log := strings.Builder{}
		log.WriteString("gazelle failed\n")
		if exitErr, ok := err.(*exec.ExitError); ok {
			log.WriteString(string(exitErr.Stderr))
		} else {
			log.WriteString(err.Error())
		}
		log.WriteString("\n")
		result := &buildpb.Result{
			Name:    "gazelle",
			Success: false,
			Logs:    check.LogsFromString("stderr", log.String()),
		}
		helper.AddResult(result)
	}
	helper.MustWriteResult()
	return totalSuccess, nil
}

func clean(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.TrimSpace(s)
	return s
}

func dirHasFiles(p string) (bool, error) {
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	fileInfos, err := ioutil.ReadDir(p)
	if err != nil {
		return false, err
	}
	return len(fileInfos) > 0, nil
}

func main() {
	flag.Parse()
	ok, err := checkGazelle(check.MustLoad())
	if err != nil {
		fmt.Println(err)
	}
	if err != nil || !ok {
		os.Exit(1)
	}
}
