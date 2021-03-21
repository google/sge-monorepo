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
	"os"
	"os/exec"
	"strings"
	"syscall"

	"sge-monorepo/build/cicd/cicdfile"
	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/monorepo/universe"
	"sge-monorepo/build/cicd/presubmit"
	"sge-monorepo/libs/go/p4lib"

	"sge-monorepo/build/cicd/presubmit/protos/presubmitpb"
)

var flags = struct {
	change   string
	logLevel string
}{}

func sgep() int {
	u, err := universe.New()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	p4 := p4lib.New()
	printer := presubmit.NewPrinter(func(opts *presubmit.PrinterOpts) {
		opts.Verbose = flags.logLevel != "ERROR"
	})
	runner := presubmit.NewRunner(u, p4, cicdfile.NewProvider(), func(opts *presubmit.Options) {
		opts.LogLevel = flags.logLevel
		opts.Change = flags.change
		opts.Listeners = append(opts.Listeners, printer)
	})
	success, err := runner.Run()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	if !success {
		return 1
	}
	return 0
}

type fixCollector struct {
	fixes []string
}

func (f *fixCollector) OnPresubmitStart(mr monorepo.Monorepo, presubmitId string, checks []presubmit.Check) {
}

func (f *fixCollector) OnCheckStart(check presubmit.Check) {
}

func (f *fixCollector) OnCheckResult(mdPath monorepo.Path, check presubmit.Check, result *presubmitpb.CheckResult) {
	for _, sr := range result.SubResults {
		if sr.Fix != "" {
			f.fixes = append(f.fixes, sr.Fix)
		}
	}
}

func (f *fixCollector) OnPresubmitEnd(success bool) {
}

func (f *fixCollector) applyFixes() error {
	if len(f.fixes) == 0 {
		fmt.Println("no fixes to apply")
		return nil
	}
	for _, fix := range f.fixes {
		fmt.Printf("applying fix %s\n", fix)
		parts := strings.Split(fix, " ")
		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

func sgepFix() int {
	u, err := universe.New()
	if err != nil {
		fmt.Println(err)
		return 1
	}
	p4 := p4lib.New()
	fixes := fixCollector{}
	runner := presubmit.NewRunner(u, p4, cicdfile.NewProvider(), func(opts *presubmit.Options) {
		opts.FixOnly = true
		opts.Listeners = append(opts.Listeners, &fixes)
	})
	if _, err := runner.Run(); err != nil {
		fmt.Println(err)
		return 1
	}
	if err := fixes.applyFixes(); err != nil {
		fmt.Println(err)
		return 1
	}
	return 0
}

func main() {
	const changeDesc = "change to restrict the presubmit run to"
	flag.StringVar(&flags.change, "change", "", changeDesc)
	flag.StringVar(&flags.change, "c", "", changeDesc+" (shorthand)")
	flag.StringVar(&flags.logLevel, "log_level", "ERROR", "glog log level")
	flag.Parse()
	if flag.NArg() == 0 {
		os.Exit(sgep())
	} else if flag.NArg() == 1 && flag.Arg(0) == "fix" {
		os.Exit(sgepFix())
	} else {
		fmt.Println("unsupported command")
	}
}
