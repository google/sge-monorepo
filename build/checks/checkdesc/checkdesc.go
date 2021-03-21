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

// Binary checkdesc checks CL descriptions for regexes.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strings"

	"sge-monorepo/build/cicd/presubmit/check"
	"sge-monorepo/libs/go/log"

	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
)

var flags = struct {
	requiredRegexp    string
	prohibitedRegexp  string
	suppressionRegexp string
}{}

func match(regex, desc string) (bool, error) {
	lines := strings.Split(desc, "\n")
	r, err := regexp.Compile(regex)
	if err != nil {
		return false, fmt.Errorf("invalid regex %q: %v", regex, err)
	}
	for _, line := range lines {
		if r.MatchString(line) {
			return true, nil
		}
	}
	return false, nil
}

func checkdesc(helper check.Helper) (bool, error) {
	desc := helper.Invocation().ClDescription
	if desc == "" {
		return false, errors.New("check_description run on empty CL description")
	}
	logs := strings.Builder{}
	if flags.suppressionRegexp != "" {
		if m, err := match(flags.suppressionRegexp, desc); err != nil {
			return false, err
		} else if m {
			return true, nil
		}
	}
	if flags.prohibitedRegexp != "" {
		if m, err := match(flags.prohibitedRegexp, desc); err != nil {
			return false, err
		} else if m {
			logs.WriteString(fmt.Sprintf("CL description matches prohibited regex: %q\n", flags.prohibitedRegexp))
		}
	}
	if flags.requiredRegexp != "" {
		if m, err := match(flags.requiredRegexp, desc); err != nil {
			return false, err
		} else if !m {
			logs.WriteString(fmt.Sprintf("CL description does not match required regex: %q\n", flags.requiredRegexp))
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
	flag.StringVar(&flags.requiredRegexp, "required_regexp", "", "regex that is required")
	flag.StringVar(&flags.prohibitedRegexp, "prohibited_regexp", "", "regex that is prohibited")
	flag.StringVar(&flags.suppressionRegexp, "suppression_regexp", "", "regex that suppresses the check")
	flag.Parse()
	log.AddSink(log.NewGlog())
	defer log.Shutdown()
	ok, err := checkdesc(check.MustLoad())
	if err != nil {
		log.Error(err)
	}
	if err != nil || !ok {
		os.Exit(1)
	}
}
