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
	"strings"

	"github.com/google/go-cmp/cmp"

	"sge-monorepo/build/cicd/presubmit/check"
	"sge-monorepo/libs/go/sgeflag"

	"sge-monorepo/build/cicd/presubmit/check/protos/checkpb"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
)

func checkfmt(helper check.Helper, fixCmd, toolPath string, args []string) (bool, error) {
	totalSuccess := true
	c := helper.OnlyCheck()
    toolBin, err := helper.ResolvePath(toolPath)
    if err != nil {
        return false, fmt.Errorf("could not resolve tool %q: %w", toolPath, err)
    }
	for _, f := range c.Files {
		log := strings.Builder{}
		if f.Status == checkpb.Status_Delete {
			continue
		}
		input, err := os.Open(f.Path)
		if err != nil {
			return false, err
		}
		defer input.Close()
		cmd := exec.Command(toolBin, args...)
		cmd.Stdin = input
		wantBytes, err := cmd.Output()
		var fix string
		if err == nil {
			gotBytes, err := ioutil.ReadFile(f.Path)
			if err != nil {
				return false, err
			}
			wantString := clean(string(wantBytes))
			gotString := clean(string(gotBytes))
			diff := cmp.Diff(gotString, wantString)
			if diff != "" {
				log.WriteString("unexpected diff\n")
				expandedFixCmd := strings.ReplaceAll(fixCmd, "$tool", toolBin)
				fix = fmt.Sprintf("%s %s", expandedFixCmd, f.Path)
				log.WriteString(diff)
				log.WriteString("\n")
			}
		} else {
			log.WriteString(fmt.Sprintf("%s has an error:\n", f.Path))
			if exitErr, ok := err.(*exec.ExitError); ok {
				log.WriteString(string(exitErr.Stderr))
			} else {
				log.WriteString(err.Error())
			}
			log.WriteString("\n")
		}
		success := log.Len() == 0
		totalSuccess = totalSuccess && success
		result := &buildpb.Result{
			Name:    f.Path,
			Success: success,
			Logs:    check.LogsFromString("stderr", log.String()),
			Fix:     fix,
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

var toolArgs sgeflag.StringList

func main() {
	fix := flag.String("fix", "", "command to run to fix")
	toolPath := flag.String("tool_path", "", "path to formatting tool")
	flag.Var(&toolArgs, "tool_arg", "argument passed to the formatting tool")
	flag.Parse()
	ok, err := checkfmt(check.MustLoad(), *fix, *toolPath, toolArgs)
	if err != nil {
		fmt.Println(err)
	}
	if err != nil || !ok {
		os.Exit(1)
	}
}
