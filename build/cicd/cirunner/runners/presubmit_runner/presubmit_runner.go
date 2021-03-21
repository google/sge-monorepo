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

// Binary presubmit_runner takes care of all the tasks related to running within a CI environment.
// It assumes it is being called in the context of a cirunner run, which already prepared the
// environment. In particular it assumes that the monorepos are already synced.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"

	"sge-monorepo/build/cicd/cicdfile"
	"sge-monorepo/build/cicd/cirunner/runnertool"
	"sge-monorepo/build/cicd/jenkins"
	"sge-monorepo/build/cicd/monorepo/universe"
	"sge-monorepo/build/cicd/presubmit"
	"sge-monorepo/libs/go/cloud/monitoring"
	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/log/cloudlog"
	"sge-monorepo/libs/go/p4lib"

	"sge-monorepo/build/cicd/cirunner/protos/cirunnerpb"

	gouuid "github.com/nu7hatch/gouuid"
)

func performPresubmit(cloudLogger cloudlog.CloudLogger) error {
	// Attempt to get the presubmit invocation.
	helper := runnertool.MustLoad()
	presubmitpb := helper.Invocation().Presubmit
	if presubmitpb == nil {
		return fmt.Errorf("no presubmit entry present in RunnerInvocation")
	}
	if presubmitpb.Change == 0 {
		return fmt.Errorf("presubmit requires a valid change within context")
	}
	// Add the labels.
	cloudLogger.AddLabels(map[string]string{
		"base_cl": strconv.Itoa(int(helper.Invocation().BaseCl)),
		"review":  strconv.Itoa(int(presubmitpb.Review)),
		"change":  strconv.Itoa(int(presubmitpb.Change)),
	})
	// Print a link to the review (this is useful for debugging/reference purposes).
	log.Infof("Review: <REVIEW URL>/%d\n", int(presubmitpb.Review))
	p4 := p4lib.New()
	// The CI system issues a presubmit run when the CL is submited. If that is the case, we don't
	// want to do a presubmit run.
	describes, err := p4.Describe([]int{int(presubmitpb.Change)})
	if err != nil || len(describes) != 1 {
		return fmt.Errorf("could not obtain description for change %d: %v", presubmitpb.Change, err)
	}
	if describes[0].Status != "pending" {
		log.Infof("change %d already submitted. Skipping presubmit.\n", presubmitpb.Change)
		return nil
	}
	credentials, err := runnertool.NewCredentials()
	if err != nil {
		return fmt.Errorf("could not obtain credentials: %v", err)
	}
	// We send a presubmit request to a (possible) shadow jenkins system.
	// Note that this is best effort and we don't block on any errors.
	if credentials.ShadowJenkins != nil {
		jenkinsRemote := jenkins.NewRemote(credentials.ShadowJenkins)
		if err := jenkinsRemote.SendPresubmitRequest(presubmitpb); err != nil {
			log.Warning("could not send shadow ci request: %v", err)
		} else {
			log.Info("Successfully sent shadow presubmit request.")
		}
	}
	// Actually issue the presubmit.
	u, err := universe.New()
	if err != nil {
		return fmt.Errorf("could not create univserse: %v", err)
	}
	presubmitContext, err := NewPresubmitContext(credentials, presubmitpb)
	if err != nil {
		return fmt.Errorf("could not obtain presubmit context: %v", err)
	}
	// We only want metrics in dev for now.
	var metrics *monitoring.Client
	if credentials.Environment.Env == cirunnerpb.Environment_DEV {
		if m, err := monitoring.NewFromDefaultProject(); err != nil {
			log.Warningf("Could not get monitoring client: %v", err)
		} else {
			metrics = m
		}
	}
	var clDescription string
	if presubmitpb.Change != 0 {
		descs, err := p4.Describe([]int{int(presubmitpb.Change)})
		if err != nil {
			return fmt.Errorf("could not get cl description for cl/%d", presubmitpb.Change)
		}
		clDescription = descs[0].Description
	}
	presubmitId := newUuid()
	listener := NewPresubmitListener(metrics)
	printer := presubmit.NewPrinter(func(opts *presubmit.PrinterOpts) {
		opts.Logs = func(s string) {
			log.Info(s)
		}
	})
	runner := presubmit.NewRunner(u, p4, cicdfile.NewProvider(), func(options *presubmit.Options) {
		options.CLDescription = clDescription
		options.PresubmitId = presubmitId
		options.Listeners = append(options.Listeners, listener, printer)
	})
	success, err := runner.Run()
	if err != nil {
		return fmt.Errorf("could not run presubmit: %v", err)
	}
	if success {
		// We don't want dev environment emailing people.
		if credentials.Environment.Env == cirunnerpb.Environment_PROD {
			if err := presubmitContext.SendPassEmail(listener.results); err != nil {
				return fmt.Errorf("could not send pass email: %v", err)
			}
			if err := presubmitContext.SendSwarmPass(); err != nil {
				return fmt.Errorf("could not send swarm pass: %v", err)
			}
		}
	} else {
		log.Error("Presubmit FAILED.")
		// We don't want dev environment emailing people.
		if credentials.Environment.Env == cirunnerpb.Environment_PROD {
			if err := presubmitContext.SendFailEmail(listener.results); err != nil {
				return fmt.Errorf("could not send fail email: %v", err)
			}
			if err := presubmitContext.SendSwarmFail(); err != nil {
				return fmt.Errorf("could not send swarm fail: %v", err)
			}
		}
		if err == nil {
			err = &fail{}
		}
	}
	listener.PrintTimings()
	listener.WaitForMetrics()
	return err
}

func internalMain() int {
	flag.Parse()
	cloudLogger, err := cloudlog.New("presubmit_runner")
	if err != nil {
		fmt.Printf("could not get cloud logger: %v\n", err)
		return 1
	}
	log.AddSink(&logSink{}, cloudLogger)
	defer log.Shutdown()

	if err := performPresubmit(cloudLogger); err != nil {
		// For a "failed" error we have already printed all the input we need and we consider it
		// a "successful" CI run.
		if isFailErr(err) {
			return 0
		}
		log.Error(err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(internalMain())
}

type fail struct{}

func (*fail) Error() string {
	return "Presubmit failed"
}

func isFailErr(err error) bool {
	_, ok := err.(*fail)
	return ok
}

type logSink struct{}

func (l *logSink) DebugDepth(depth int, msg string) {
	fmt.Println(msg)
}

func (l *logSink) InfoDepth(depth int, msg string) {
	fmt.Println(msg)
}

func (l *logSink) WarningDepth(depth int, msg string) {
	fmt.Println("WARNING: " + msg)
}

func (l *logSink) ErrorDepth(depth int, msg string) {
	fmt.Println("ERROR: " + msg)
}

func (l *logSink) Close() {}

func newUuid() string {
	uuid, err := gouuid.NewV4()
	if err != nil {
		panic("could not construct UUID: " + err.Error())
	}
	return uuid.String()
}
