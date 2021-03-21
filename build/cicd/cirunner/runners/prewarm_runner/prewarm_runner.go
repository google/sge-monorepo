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

// Binary presubmit_runner takes care of all the tasks related to bringing a CI machine to running
// condition.
package main

import (
	"flag"
	"fmt"
	"os"

	_ "sge-monorepo/build/cicd/cirunner/runnertool"
	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/sgeb/build"

	"github.com/golang/glog"
)

// targets is the list of targets that we want to eagerly build on prewarm stage.
var targets = []string{
	// Unity projects have a long initial step where they import the code, which then lives in a
	// cache. Building those projects at warmup will ensure fast builds after that.
}

func run() error {
	flag.Parse()
	defer glog.Flush()

	mr, rel, err := monorepo.NewFromPwd()
	if err != nil {
		return fmt.Errorf("could not locate WORKSPACE: %v", err)
	}
	bc, err := build.NewContext(mr)
	if err != nil {
		return fmt.Errorf("could not create build context: %v", err)
	}
	defer bc.Cleanup()
	for _, target := range targets {
		bu, err := mr.NewLabel(rel, target)
		if err != nil {
			return fmt.Errorf("could not obtain label %q: %v", target, err)
		}
		glog.Infof("Building %q\n", bu)
		// What should the logging options be here?
		result, err := bc.Build(bu, func(opts *build.Options) {
			opts.LogLevel = "INFO"
			opts.Logs = os.Stdout
		})
		if result != nil && !result.OverallResult.Success {
			build.PrintFailedBuildResult(os.Stdout, result)
		}
		if err != nil {
			return fmt.Errorf("could not build %q: %v", bu, err)
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		glog.Error(err)
		os.Exit(1)
	}
}
