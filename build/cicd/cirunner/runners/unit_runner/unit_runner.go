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

// Binary unit_runner is a runner that receives unit labels as arguments and executes them.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"sge-monorepo/build/cicd/cirunner/runnertool"
	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/sgeb/build"
	"sge-monorepo/environment/envinstall"
	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/log/cloudlog"
	"sge-monorepo/libs/go/p4lib"

	"sge-monorepo/build/cicd/cirunner/protos/cirunnerpb"
	"sge-monorepo/build/cicd/cirunner/runners/unit_runner/protos/unit_runnerpb"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
)

type logger struct{}

func (l logger) Write(p []byte) (n int, err error) {
	log.Info(string(p))
	return len(p), nil
}

func actionAndLabelFromInvocation(unitpb *cirunnerpb.RunnerInvocation_Unit) (string, string) {
	if unitpb.BuildUnit != "" {
		return "build", unitpb.BuildUnit
	} else if unitpb.PublishUnit != "" {
		return "publish", unitpb.PublishUnit
	} else if unitpb.TestUnit != "" {
		return "test", unitpb.TestUnit
	} else if unitpb.TaskUnit != "" {
		return "task", unitpb.TaskUnit
	} else {
		return "", ""
	}
}

func writeTask(p4 p4lib.P4, key string, task *unit_runnerpb.Task) error {
	value := proto.MarshalTextString(task)
	if err := p4.KeySet(key, value); err != nil {
		return fmt.Errorf("could not write task to P4 key %q: %w", key, err)
	}
	log.Infof("Wrote task into key %q:\n%s", key, value)
	return nil
}

func run(helper runnertool.Helper, cloudLogger cloudlog.CloudLogger) error {
	unitpb := helper.Invocation().Unit
	if unitpb == nil {
		return errors.New("no unit entry present in RunnerInvocation")
	}
	action, label := actionAndLabelFromInvocation(unitpb)
	if cloudLogger != nil {
		// Add the labels for correctly see this in the cloud log.
		cloudLogger.AddLabels(map[string]string{
			"base_cl":  strconv.Itoa(int(helper.Invocation().BaseCl)),
			"action":   action,
			"label":    label,
			"task_key": unitpb.TaskKey,
			"invoker":  unitpb.Invoker,
		})
	}
	if unitpb.Invoker != "" {
		log.Infof("Build requested by %s", unitpb.Invoker)
		if unitpb.InvokerUrl != "" {
			log.Infof("Associated URL: %s", unitpb.InvokerUrl)
		}
	}

	// Write the task to the expected key.
	p4 := p4lib.New()
	var task *unit_runnerpb.Task
	if unitpb.TaskKey != "" {
		task = &unit_runnerpb.Task{
			Action:     action,
			Label:      label,
			Start:      &timestamp.Timestamp{Seconds: time.Now().Unix()},
			ResultsUrl: unitpb.ResultsUrl,
			Status:     unit_runnerpb.TaskStatus_RUNNING,
		}
		if err := writeTask(p4, unitpb.TaskKey, task); err != nil {
			return err
		}
	} else {
		log.Infof("No task key provided. Ignoring.")
	}
	err := execute(helper, unitpb)
	if task != nil {
		task.Status = unit_runnerpb.TaskStatus_SUCCESS
		if err != nil {
			task.Status = unit_runnerpb.TaskStatus_FAILED
		}
		if err := writeTask(p4, unitpb.TaskKey, task); err != nil {
			return err
		}
	}
	if err != nil {
		return fmt.Errorf("failed executing \"%s %s\": %w", action, label, err)
	}
	return nil
}

func execute(helper runnertool.Helper, unitpb *cirunnerpb.RunnerInvocation_Unit) error {
	mr, _, err := monorepo.NewFromPwd()
	if err != nil {
		return fmt.Errorf("could not get monorepo: %w", err)
	}
	bc, err := build.NewContext(mr, func(options *build.Options) {
		if unitpb.LogLevel != "" {
			options.LogLevel = unitpb.LogLevel
		}
	})
	if err != nil {
		return fmt.Errorf("could not create build context: %w", err)
	}
	defer bc.Cleanup()
	if unitpb.BuildUnit != "" {
		label, err := mr.NewLabel("", unitpb.BuildUnit)
		if err != nil {
			return fmt.Errorf("could not get label for %q: %w", unitpb.BuildUnit, err)
		}
		log.Infof("Building %q", label)
		result, err := bc.Build(label)
		if result != nil {
			build.PrintBuildResult(logger{}, label, result, -1)
		}
		if err != nil {
			return fmt.Errorf("could not build %q: %w", label, err)
		}
	} else if unitpb.PublishUnit != "" {
		label, err := mr.NewLabel("", unitpb.PublishUnit)
		if err != nil {
			return fmt.Errorf("could not get label for %q: %w", unitpb.PublishUnit, err)
		}
		var args []string
		if unitpb.Args != "" {
			args = strings.Split(unitpb.Args, ";")
		}
		log.Infof("Publishing %q", label)
		results, err := bc.Publish(label, args, func(o *build.Options, po *build.PublishOptions) {
			o.Logs = logger{}
			po.BaseCl = helper.Invocation().BaseCl
		})
		if err != nil {
			return fmt.Errorf("could not publish: %w", err)
		}
		for _, result := range results {
			log.Infof("Name: %s", result.Name)
			log.Infof("Version: %s", result.Version)
			for _, file := range result.Files {
				log.Infof("Published file (bytes): %v", file.Size)
			}
		}
	} else if unitpb.TestUnit != "" {
		log.Info("Testing label %q", unitpb.TestUnit)
		te, err := mr.NewTargetExpressionWithShorthand("", unitpb.TestUnit, "test")
		if err != nil {
			return fmt.Errorf("could not get shorthand for %q: %w", unitpb.TestUnit, err)

		}
		testUnits, err := bc.ExpandTargetExpression(te)
		if err != nil {
			return fmt.Errorf("could not expand target expression: %w", err)
		}
		var errors []error
		for _, tu := range testUnits {
			log.Infof("Testing %q", tu)
			result, err := bc.Test(tu)
			if result != nil {
				build.PrintTestResult(logger{}, tu, result)
			}
			if err != nil {
				errors = append(errors, err)
				if result == nil {
					log.Errorf("Error for test unit %q: %v", tu, err)
				}
			}
		}
		if len(errors) != 0 {
			return fmt.Errorf("testing label %q FAILED", unitpb.TestUnit)
		}
	} else if unitpb.TaskUnit != "" {
		label, err := mr.NewLabel("", unitpb.TaskUnit)
		if err != nil {
			return fmt.Errorf("could not get label for %q: %w", unitpb.TaskUnit, err)
		}
		log.Infof("Running task %q", label)
		var args []string
		if unitpb.Args != "" {
			args = strings.Split(unitpb.Args, ";")
		}
		if err := bc.RunTask(label, args); err != nil {
			return fmt.Errorf("could not run task %q: %w", label, err)
		}
	}
	return nil
}

func internalMain() int {
	flag.Parse()
	helper := runnertool.MustLoad()
	log.AddSink(log.NewGlog())
	var cloudLogger cloudlog.CloudLogger
	if envinstall.IsCloud() {
		var err error
		cloudLogger, err = cloudlog.New("unit_runner")
		if err != nil {
			fmt.Printf("could not get cloud logger: %v\n", err)
			return 1
		}
		log.AddSink(cloudLogger)
	}
	defer log.Shutdown()

	if err := run(helper, cloudLogger); err != nil {
		log.Errorf("could not run unit_runner: %v", err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(internalMain())
}
