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

// Binary sgeb build/tests SGE build and test units.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/sgeb/build"
	"sge-monorepo/libs/go/log"
)

const defaultMaxResults = 10

func printUsage() {
	fmt.Println(`Usage:
sgeb [-log_level=level -remote] build|test|publish|run <unit>`)
	fmt.Println("  -log_level: One of INFO, WARNING, ERROR, FATAL")
}

func sgeb() error {
	log.AddSink(log.NewGlog())
	defer log.Shutdown()
	flags := struct {
		logLevel string
		remote   bool
		change   int
	}{}
	flag.StringVar(&flags.logLevel, "log_level", "ERROR", "log level. One of INFO, WARNING, ERROR, FATAL")
	flag.BoolVar(&flags.remote, "remote", false, "Whether this should be run on a remote machine within the dev environment")
	flag.IntVar(&flags.change, "c", 0, "For remote runs, unshelve this CL before running the command on the remote machine.")
	flag.Parse()

	mr, rel, err := monorepo.NewFromPwd()
	if err != nil {
		return fmt.Errorf("could not locate WORKSPACE: %v", err)
	}
	bc, err := build.NewContext(mr, func(options *build.Options) {
		options.LogLevel = flags.logLevel
	})
	if err != nil {
		return fmt.Errorf("could not create build context: %v", err)
	}
	defer bc.Cleanup()

	if flag.NArg() == 0 {
		printUsage()
		return nil
	}
	action := flag.Arg(0)
	switch action {
	case "build":
		flagSet := flag.NewFlagSet("build", flag.ExitOnError)
		_ = flagSet.Parse(flag.Args()[1:])
		if flagSet.NArg() == 0 {
			return fmt.Errorf("must pass build unit to build command")
		}
		target := strings.ReplaceAll(flagSet.Arg(0), `\`, `/`)
		bu, err := mr.NewLabel(rel, target)
		if err != nil {
			return err
		}
		fmt.Printf("Building %s\n", bu)
		if flags.remote {
			return remote(remoteRequest{
				action:   action,
				label:    bu.String(),
				logLevel: flags.logLevel,
				change:   flags.change,
			})
		}
		result, err := bc.Build(bu)
		if result != nil {
			build.PrintBuildResult(os.Stderr, bu, result, defaultMaxResults)
		}
		return err
	case "test":
		flagSet := flag.NewFlagSet("test", flag.ExitOnError)
		_ = flagSet.Parse(flag.Args()[1:])
		if flagSet.NArg() == 0 {
			return fmt.Errorf("must pass test unit to test command")
		}
		target := strings.ReplaceAll(flagSet.Arg(0), `\`, `/`)
		if flags.remote {
			return remote(remoteRequest{
				action:   action,
				label:    target,
				logLevel: flags.logLevel,
				change:   flags.change,
			})
		}
		te, err := mr.NewTargetExpressionWithShorthand(rel, target, "test")
		if err != nil {
			return err
		}
		testUnits, err := bc.ExpandTargetExpression(te)
		if err != nil {
			return err
		}
		var errs []error
		for _, tu := range testUnits {
			fmt.Printf("Testing %s\n", tu)
			result, err := bc.Test(tu)
			if result != nil {
				build.PrintTestResult(os.Stderr, tu, result)
			}
			if err != nil {
				errs = append(errs, err)
				if result == nil {
					fmt.Println(err)
				}
			}
		}
		if len(errs) != 0 {
			return fmt.Errorf("sgeb test FAILED")
		}
		return nil
	case "publish":
		flagSet := flag.NewFlagSet("publish", flag.ExitOnError)
		_ = flagSet.Parse(flag.Args()[1:])
		// First argument is binary to run, all other arguments are forwarded to the binary.
		if flagSet.NArg() == 0 {
			return fmt.Errorf("must pass publish unit to publish command")
		}
		target := strings.ReplaceAll(flagSet.Arg(0), `\`, `/`)
		pu, err := mr.NewLabelWithShorthand(rel, target, "publish")
		if err != nil {
			return err
		}
		fmt.Printf("Publishing %s\n", pu)
		publishArgs := flagSet.Args()[1:]
		if flags.remote {
			return remote(remoteRequest{
				action:   action,
				label:    pu.String(),
				logLevel: flags.logLevel,
				change:   flags.change,
				args:     publishArgs,
			})
		}
		results, err := bc.Publish(pu, publishArgs)
		if err != nil {
			return err
		}
		if len(results) > 0 {
			for _, r := range results {
				fmt.Printf("Published %s successfully\n", r.Name)
			}
		} else {
			fmt.Println("Nothing to publish (no changes detected?)")
		}
		return nil
	case "run":
		if flags.remote {
			return errors.New("cannot use -remote with run")
		}
		flagSet := flag.NewFlagSet("run", flag.ExitOnError)
		_ = flagSet.Parse(flag.Args()[1:])
		// First argument is binary to run, all other arguments are forwarded to the binary.
		if flagSet.NArg() == 0 {
			return fmt.Errorf("must pass build unit to run command")
		}
		target := strings.ReplaceAll(flagSet.Arg(0), `\`, `/`)
		bu, err := mr.NewLabel(rel, target)
		if err != nil {
			return err
		}
		fmt.Printf("Building %s\n", bu)
		p, result, err := bc.ResolveBin("", bu.String())
		if err != nil && result != nil {
			build.PrintBuildResult(os.Stderr, bu, result, defaultMaxResults)
		}
		if err != nil {
			return err
		}
		runArgs := flagSet.Args()[1:]
		cmd := exec.Command(p, runArgs...)
		fmt.Printf("Running %s\n", cmd.String())
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	case "cron":
		if flags.remote {
			return errors.New("cannot use -remote with cron")
		}
		flagSet := flag.NewFlagSet("cron", flag.ExitOnError)
		_ = flagSet.Parse(flag.Args()[1:])
		// First argument is binary to run, all other arguments are forwarded to the binary.
		if flagSet.NArg() == 0 {
			return fmt.Errorf("must pass cron unit to cron command")
		}
		target := strings.ReplaceAll(flagSet.Arg(0), `\`, `/`)
		cu, err := mr.NewLabelWithShorthand(rel, target, "cron")
		if err != nil {
			return err
		}
		fmt.Printf("Running %s\n", cu)
		cronArgs := flagSet.Args()[1:]
		return bc.RunCron(cu, cronArgs)
	case "task":
		if flags.remote {
			return errors.New("cannot use -remote with task")
		}
		flagSet := flag.NewFlagSet("task", flag.ExitOnError)
		_ = flagSet.Parse(flag.Args()[1:])
		// First argument is binary to run, all other arguments are forwarded to the binary.
		if flagSet.NArg() == 0 {
			return fmt.Errorf("must pass task unit to task command")
		}
		target := strings.ReplaceAll(flagSet.Arg(0), `\`, `/`)
		cu, err := mr.NewLabelWithShorthand(rel, target, "task")
		if err != nil {
			return err
		}
		fmt.Printf("Running %s\n", cu)
		taskArgs := flagSet.Args()[1:]
		return bc.RunTask(cu, taskArgs)
	default:
		return fmt.Errorf("unknown command: %q", flag.Arg(0))
	}
}

func main() {
	if err := sgeb(); err == nil {
		fmt.Printf("sgeb %s succeeded\n", flag.Arg(0))
	} else {
		fmt.Println(err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Println(string(exitErr.Stderr))
		}
		os.Exit(1)
	}
}
