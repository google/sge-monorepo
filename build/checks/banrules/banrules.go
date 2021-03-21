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

// Binary banrules returns an error if a -rule_matcher finds a matching Bazel rule.

// This is implemented using 'bazel query kind()'
// https://docs.bazel.build/versions/master/query-how-to.html
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"sge-monorepo/build/cicd/presubmit/check"

	"sge-monorepo/build/cicd/presubmit/check/protos/checkpb"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
)

var ruleMatcherFlag = flag.String("rule_matcher", "", "bazel query rule matcher expression")

func checkBannedRules(helper check.Helper) (bool, error) {
	totalSuccess := true
	c := helper.OnlyCheck()

	// Run on all changed directories.
	dirs := map[string]bool{}
	for _, f := range c.Files {
		// If the file is deleted, we don't care about checking for rules.
		if f.Status == checkpb.Status_Delete {
			continue
		}
		dir := path.Dir(f.Path)
		dirs[dir] = true
	}

	toolBin, err := helper.ResolvePath("//bin/windows/bazel.exe")
	if err != nil {
		return false, err
	}
	for d := range dirs {
		d, err := helper.RelPath(d)
		if err != nil {
			return false, err
		}
		if d == "." {
			d = ""
		}
		query := fmt.Sprintf(`kind(%s, %s:*)`, *ruleMatcherFlag, d)
		cmd := exec.Command(toolBin, "query", query)
		output, err := cmd.Output()
		log := strings.Builder{}
		success := err == nil && len(output) == 0
		totalSuccess = totalSuccess && success
		if err != nil {
			log.WriteString("bazel query failed\n")
			if exitErr, ok := err.(*exec.ExitError); ok {
				log.WriteString(string(exitErr.Stderr))
			} else {
				log.WriteString(err.Error())
			}
			log.WriteString("\n")
		} else if len(output) > 0 {
			log.WriteString(fmt.Sprintf("Found rule(s) matching banned expression %q:\n", *ruleMatcherFlag))
			log.WriteString(string(output))
			log.WriteString("\n")
		}
		result := &buildpb.Result{
			Name:    "banrules",
			Success: success,
			Logs:    check.LogsFromString("stderr", log.String()),
		}
		helper.AddResult(result)
	}
	helper.MustWriteResult()
	return totalSuccess, nil
}

func main() {
	flag.Parse()
	if *ruleMatcherFlag == "" {
		fmt.Println("missing --rule_matcher")
		os.Exit(1)
	}
	ok, err := checkBannedRules(check.MustLoad())
	if err != nil {
		fmt.Println(err)
	}
	if err != nil || !ok {
		os.Exit(1)
	}
}
