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

// Binary checkbuildunit verifies that a BUILDUNIT file loads.
package main

import (
	"flag"
	"os"
	"strings"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/presubmit/check"
	"sge-monorepo/build/cicd/sgeb/build"
	"sge-monorepo/libs/go/log"

	"sge-monorepo/build/cicd/presubmit/check/protos/checkpb"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
)

func checkBuildUnit(helper check.Helper) (bool, error) {
	mr, _, err := monorepo.NewFromPwd()
	if err != nil {
		return false, err
	}
	bc, err := build.NewContext(mr)
	if err != nil {
		return false, err
	}

	logs := strings.Builder{}
	c := helper.OnlyCheck()
	for _, f := range c.Files {
		if f.Status == checkpb.Status_Delete {
			continue
		}
		buPath, err := mr.RelPath(f.Path)
		if err != nil {
			return false, err
		}
		buDir := buPath.Dir()
		_, err = bc.LoadBuildUnits(buDir)
		if err != nil {
			logs.WriteString(err.Error())
			logs.WriteRune('\n')
		}
	}

	success := logs.Len() == 0
	result := &buildpb.Result{
		Name:    "check_description",
		Success: success,
		Logs:    check.LogsFromString("stderr", logs.String()),
	}
	helper.AddResult(result)
	helper.MustWriteResult()
	return success, nil
}

func main() {
	flag.Parse()
	log.AddSink(log.NewGlog())
	defer log.Shutdown()
	ok, err := checkBuildUnit(check.MustLoad())
	if err != nil {
		log.Error(err)
	}
	if err != nil || !ok {
		os.Exit(1)
	}
}
