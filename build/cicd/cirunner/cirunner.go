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

// Binary cirunner takes care of all the tasks related to running within a Jenkins environment.
// It main job is to prepare the environment so that it can build and then run internal_runner,
// which holds most of the ci logic. This permits to update the cirunner without needing to publish
// a binary for every change.
//
// cirunner also has a "fork" mechanism, in which it copies its own binary into a temporary location
// within the monorepo. This is so that when syncing, it can sync over itself. Otherwise it could
// occur that new published versions of cirunner would hang because it cannot overwrite itself.
// After the fork, it will prewarm (sync and install dependencies) and then forward the call to
// the internal runner.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	"sge-monorepo/build/cicd/cirunner/runnertool"
	"sge-monorepo/build/cicd/monorepo/universe"
	"sge-monorepo/build/cicd/sgeb/build"
	"sge-monorepo/environment/envinstall"
	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/log/cloudlog"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/swarm"

	"sge-monorepo/build/cicd/cirunner/protos/cirunnerpb"

	"github.com/golang/protobuf/proto"
)

func printUsage() {
	fmt.Print(`
Usage:
    sge-ci-runner -invocation=<RUNNER_INVOCATION TEXTPB> [COMMAND]
    Commands:
        prewarm
            Performs "deps" followed by "sync".

        send-presubmit-email <start|fail> [mail]

        send-swarm-swarm <start|pass|fail>
            Sends an request to Swarm updating it about the state of the presubmit runs.

        <OTHER VALUES>
            All other values are informative, because the internal runner will be determined by
            the invocation proto.
`)
}

// findTargetForInvocation maps the invocation proto message to a target to build.
// THIS IS THE FUNCTION TO EDIT IN ORDER TO ADD A NEW INTERNAL RUNNER!
// It is assumed that the |RunnerInvocation| proto should only have one internal runner message
// present, so a simple check for existense is enough.
func findTargetForInvocation(invocation *cirunnerpb.RunnerInvocation) (string, error) {
	if invocation.Presubmit != nil {
		return "//build/cicd/cirunner/runners/presubmit_runner", nil
	} else if invocation.Postsubmit != nil {
		return "//build/cicd/cirunner/runners/postsubmit_runner", nil
	} else if invocation.Publish != nil {
		return "//build/cicd/cirunner/runners/publish_runner", nil
	} else if invocation.Cron != nil {
		return "//build/cicd/cirunner/runners/cron_runner", nil
	} else if invocation.Dev != nil {
		return "//build/cicd/cirunner/runners/dev_runner", nil
	} else if invocation.Prewarm != nil {
		return "//build/cicd/cirunner/runners/prewarm_runner", nil
	} else if invocation.Unit != nil {
		return "//build/cicd/cirunner/runners/unit_runner", nil
	}
	return "", fmt.Errorf("no valid invocation found")
}

var gInvocationPath = flag.String("invocation", "", "Path to the invocation text proto")

func loadInvocation() (*cirunnerpb.RunnerInvocation, error) {
	if *gInvocationPath == "" {
		return nil, fmt.Errorf("no --invocation flag provided")
	}
	data, err := ioutil.ReadFile(*gInvocationPath)
	if err != nil {
		return nil, fmt.Errorf("could not read file %s: %v", *gInvocationPath, err)
	}
	invocation := &cirunnerpb.RunnerInvocation{}
	if err := proto.UnmarshalText(string(data), invocation); err != nil {
		return nil, fmt.Errorf("could not unmarshal proto: %v\n%q", err, string(data))
	}
	return invocation, nil
}

// clSubmited returns whether the CL is submitting by querying perforce.
func clSubmitted(p4 p4lib.P4, change int) (bool, error) {
	if change == 0 {
		return false, nil
	}
	describes, err := p4.Describe([]int{change})
	if err != nil || len(describes) != 1 {
		return false, fmt.Errorf("could not obtain description for change %d: %v", change, err)
	}
	submitted := describes[0].Status != "pending"
	return submitted, nil
}

// Email -------------------------------------------------------------------------------------------

func sendPresubmitEmail() error {
	invocation, err := loadInvocation()
	if err != nil {
		return fmt.Errorf("could not load invocation proto: %v", err)
	}
	if invocation.Presubmit == nil {
		return fmt.Errorf("no presubmit invocation proto present")
	}
	credentials, err := runnertool.NewCredentials()
	if err != nil {
		return fmt.Errorf("could not load credentials: %v", err)
	}
	presubmitContext, err := NewPresubmitContext(credentials, invocation.Presubmit)
	if err != nil {
		return fmt.Errorf("could not create presubmit context: %v", err)
	}
	emailFlagSet := flag.NewFlagSet("send-presubmit-email", flag.ExitOnError)
	if err := emailFlagSet.Parse(flag.Args()[1:]); err != nil {
		return err
	}
	if emailFlagSet.NArg() < 1 {
		printUsage()
		return fmt.Errorf("email command receives at least 1 argument")
	}
	cmd := emailFlagSet.Arg(0)
	// Presubmit emails are only sent if the CL is not submitted.
	p4 := p4lib.New()
	submitted, err := clSubmitted(p4, int(invocation.Change))
	if err != nil {
		return fmt.Errorf("could not query if CL is submitted: %v", err)
	}
	if submitted {
		return nil
	}
	switch cmd {
	case "start":
		return presubmitContext.SendStartEmail()
	case "fail":
		return presubmitContext.SendFailEmail()
	}
	printUsage()
	return fmt.Errorf("unknown email cmd: %s", cmd)
}

// Prewarm -----------------------------------------------------------------------------------------

// prewarm ensures that the p4 client has the correct setup, the machine environment has the correct
// dependencies installed and finally it syncs perforce.
func prewarm(p4 p4lib.P4, invocation *cirunnerpb.RunnerInvocation) (int, error) {
	if err := ensureClobberClient(p4); err != nil {
		return 0, fmt.Errorf(`could not ensure "clobber" p4 client: %v`, err)
	}
	m, err := envinstall.NewManager(p4)
	if err != nil {
		return 0, err
	}
	uptodate, err := m.UpToDate()
	if err != nil {
		return 0, err
	}
	if !uptodate {
		log.Info("Environment not up to date. Installing dependencies.")
		if err := m.SyncAndInstallDependencies(); err != nil {
			return 0, err
		}
	}
	return sync(p4, invocation)
}

func ensureClobberClient(p4 p4lib.P4) error {
	client, err := p4.Client("")
	if err != nil {
		return fmt.Errorf("could not obtain P4DEFAULT client: %v", err)
	}
	// Look for the clobber option.
	for _, option := range client.Options {
		if option == p4lib.Clobber {
			return nil
		}
	}
	// Ensure that we have the clobber option. If it's already there, this is a no-op.
	// If the client was noclobber, this will override.
	options, err := p4lib.AppendClientOption(client.Options, p4lib.Clobber)
	if err != nil {
		return fmt.Errorf(`could not set "clobber" option: %v`, err)
	}
	client.Options = options
	out, err := p4.ClientSet(client)
	if err != nil {
		log.Error(out)
		return fmt.Errorf("could not set client: %v", err)
	}
	log.Infof("Changed p4 client to clobber mode. New Client:\n%s", client.String())
	return nil
}

// sync runs p4 sync to sync to HEAD. Returns the CL it synced to.
func sync(p4 p4lib.P4, invocation *cirunnerpb.RunnerInvocation) (int, error) {
	u, err := universe.New()
	if err != nil {
		return 0, err
	}
	// The universe has an explicit mapping on what should be in the CI machines' client.
	// We update the client to match that.
	if err := u.UpdateClientView(p4, ""); err != nil {
		return 0, err
	}
	// If no explicit base CL to sync to was provided, we obtain the latest one from perforce.
	baseCl := int(invocation.BaseCl)
	if baseCl == 0 {
		log.Info("No explicit CL to sync to provided. Syncing to HEAD.")
		changes, err := p4.Changes("-s", "submitted", "-m", "1")
		if err != nil {
			return 0, err
		}
		if len(changes) != 1 {
			return 0, fmt.Errorf("%d changes returned by p4 changes, want 1", len(changes))
		}
		baseCl = changes[0].Cl
	} else {
		log.Infof("Explicit CL to sync to provided: %d", baseCl)
	}
	// The universe already defined all the code that we care about, so we can issue a
	// blanket sync and that will get the correct code. We use a stdout option to stream to the
	// caller, in order not to give the impression of being frozen without any output.
	syncArgs := []string{"sync", "--parallel", fmt.Sprintf("threads=%d", runtime.NumCPU()), fmt.Sprintf("@%d", baseCl)}
	// Create a new counter that prints how many files we've synced instead of spewing all the
	// information to stdout.
	size, err := p4.SyncSize([]string{"//..."})
	if err != nil {
		return 0, fmt.Errorf("error getting sync size: %v", err)
	}
	log.Infof("Syncing to CL/%d", baseCl)
	log.Infof("Syncing %d files...", size.FilesAdded+size.FilesUpdated+size.FilesDeleted)
	_, err = p4.ExecCmdWithOptions(syncArgs)
	if err != nil {
		return 0, err
	}
	return baseCl, nil
}

// Swarm -------------------------------------------------------------------------------------------

func sendSwarmRequest(p4 p4lib.P4) error {
	invocation, err := loadInvocation()
	if err != nil {
		return fmt.Errorf("could not load invocation proto: %v", err)
	}
	if invocation.Presubmit == nil {
		return fmt.Errorf("no presubmit invocation proto present")
	}
	credentials, err := runnertool.NewCredentials()
	if err != nil {
		return fmt.Errorf("could not load credentials: %v", err)
	}
	presubmitContext, err := NewPresubmitContext(credentials, invocation.Presubmit)
	if err != nil {
		return fmt.Errorf("could not create presubmit context: %v", err)
	}
	swarmFlagSet := flag.NewFlagSet("swarm", flag.ExitOnError)
	requestType, err := processSwarmFlags(swarmFlagSet)
	if err != nil {
		return err
	}
	// We only communicate with Swarm if the CL is not submitted.
	submitted, err := clSubmitted(p4, int(invocation.Change))
	if err != nil {
		return err
	}
	if submitted {
		return nil
	}
	update := invocation.Presubmit.UpdateUrl
	results := invocation.Presubmit.ResultsUrl
	if _, err := swarm.SendTestRunRequest(presubmitContext.swarmContext, requestType, update, results); err != nil {
		return fmt.Errorf("error sending swarm request: %v", err)
	}
	return nil
}

func processSwarmFlags(flags *flag.FlagSet) (swarm.TestRunResponseType, error) {
	if err := flags.Parse(flag.Args()[1:]); err != nil {
		return 0, err
	}
	if flags.NArg() != 1 {
		printUsage()
		return 0, fmt.Errorf("swarm command only receives one argument")
	}
	var responseType swarm.TestRunResponseType
	swarmCmd := flags.Arg(0)
	switch swarmCmd {
	case "start":
		responseType = swarm.TestRunStart
	case "pass":
		responseType = swarm.TestRunPass
	case "fail":
		responseType = swarm.TestRunFail
	default:
		printUsage()
		return 0, fmt.Errorf("unknown swarm command: %s", swarmCmd)
	}
	return responseType, nil
}

// Forward Command ---------------------------------------------------------------------------------

func forwardToInternalRunner(p4 p4lib.P4, cloudLogger cloudlog.CloudLogger) error {
	invocation, err := loadInvocation()
	if err != nil {
		return fmt.Errorf("could not load invocation proto: %v", err)
	}
	log.Infof("Invocation proto:\n%s", proto.MarshalTextString(invocation))
	// Not matter what happens, we always revert the p4 state to a clean slate.
	defer func() {
		out, err := p4.ExecCmd("revert", "-w", "//...")
		if err != nil {
			log.Warningf("Could not revert CL on cleanup: %v: %s", err, out)
		}
	}()
	// Before we build the internal_runner, we sync and unshelve the code, so that the current state
	// also applies to the internal_runner itself.
	baseCl, err := syncP4State(p4, invocation)
	if err != nil {
		return fmt.Errorf("syncing P4 state: %v", err)
	}
	invocation.BaseCl = int64(baseCl)
	if cloudLogger != nil {
		cloudLogger.AddLabels(map[string]string{
			"change":  strconv.Itoa(int(invocation.Change)),
			"base_cl": strconv.Itoa(int(invocation.BaseCl)),
		})
	}
	if err := runInternalRunner(p4, invocation); err != nil {
		return err
	}
	return nil
}

func syncP4State(p4 p4lib.P4, invocation *cirunnerpb.RunnerInvocation) (int, error) {
	// The first thing we do is sync the machine state up to date.
	baseCl, err := prewarm(p4, invocation)
	if err != nil {
		return 0, fmt.Errorf("error prewarming: %v", err)
	}
	// Just in case we revert anything already opened.
	_, err = p4.ExecCmd("revert", "-w", "//...")
	if err != nil {
		return 0, fmt.Errorf("could not revert files: %v", err)
	}
	// If there is a valid change associated with this run, we unshelve it.
	if invocation.Change != 0 {
		change := int(invocation.Change)
		submitted, err := clSubmitted(p4, change)
		if err != nil {
			return 0, err
		}
		if submitted {
			log.Infof("change %d already submitted. Skipping unshelving.", change)
		} else {
			// Unshelve the change we want to review.
			log.Infof("Unshelving changelist %d.", change)
			out, err := p4.VerifiedUnshelve(int(change))
			if err != nil {
				return 0, fmt.Errorf("could not unshelve CL: %v", err)
			}
			// We count the amount of files that were unshelved and print them.
			threshold := 25
			numFiles := strings.Count(out, "\n")
			if numFiles <= threshold {
				log.Infof("Unshelved:\n%s", out)
			} else {
				log.Infof("Unshelved %d files.", numFiles)
			}
		}
	}
	return baseCl, nil
}

func runInternalRunner(p4 p4lib.P4, invocation *cirunnerpb.RunnerInvocation) error {
	target, err := findTargetForInvocation(invocation)
	if err != nil {
		return fmt.Errorf("could not find valid internal runner: %v", err)
	}
	log.Infof("Building %q", target)
	u, err := universe.New()
	if err != nil {
		return fmt.Errorf("could not obtain universe: %v", err)
	}
	sge := u.GetMonorepo("sge")
	if sge == nil {
		return fmt.Errorf("could not find sge monorepo")
	}
	mr, err := sge.Resolve(p4)
	if err != nil {
		return fmt.Errorf("could not resolve monorepo: %v", err)
	}
	var logs bytes.Buffer
	bcOptions := func(options *build.Options) {
		options.Logs = &logs
	}
	bc, err := build.NewContext(mr, bcOptions)
	if err != nil {
		return fmt.Errorf("could not create build context: %v", err)
	}
	internalRunner, result, err := bc.ResolveBin("", target)
	if err != nil {
		if result != nil {
			writer := errorWriter{}
			build.PrintFailedBuildResult(writer, result)
		}
		return fmt.Errorf("could not resolve %s: %v. Logs: %s", target, err, logs.String())
	}
	defer func() {
		if err := bc.Cleanup(); err != nil {
			log.Errorf("could not cleanup internal_runner: %v. Logs: %s", err, logs.String())
		}
	}()
	invFile, err := ioutil.TempFile("", "runner-invocation-*.textpb")
	if err != nil {
		return fmt.Errorf("could not write invocation proto: %v", err)
	}
	defer os.Remove(invFile.Name())
	if _, err := invFile.WriteString(proto.MarshalTextString(invocation)); err != nil {
		return fmt.Errorf("could not write invocation proto: %v", err)
	}
	var args []string
	args = append(args, "-logtostderr")
	args = append(args, "--runner-invocation", invFile.Name())
	log.Infof("Forwarding to %q: %s", target, args)
	cmd := exec.Command(internalRunner, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type errorWriter struct{}

func (e errorWriter) Write(p []byte) (n int, err error) {
	log.Error(string(p))
	return len(p), nil
}

// Main --------------------------------------------------------------------------------------------

// internalMain is used so that we can run defer calls and have "exit" semantics. This is because
// os.Exit does not honor pending defers.
func internalMain() int {
	flag.Parse()
	log.AddSink(log.NewGlog())
	var cloudLogger cloudlog.CloudLogger
	if envinstall.IsCloud() {
		var err error
		cloudLogger, err = cloudlog.New("cirunner")
		if err != nil {
			fmt.Printf("could not obtain cloud logger: %v\n", err)
			return 1
		}
		log.AddSink(cloudLogger)
	}
	defer log.Shutdown()
	p4 := p4lib.New()
	// In general what cirunner does comes from the invocation that indicates which internal_runner
	// to invoke. It is possible to override what cirunner does by passing a command as first
	// argument.
	cmd := ""
	if len(flag.Args()) > 0 {
		cmd = flag.Arg(0)
	}
	var err error
	switch cmd {
	case "send-presubmit-email":
		err = sendPresubmitEmail()
	case "send-swarm-request":
		err = sendSwarmRequest(p4)
	default:
		if err := forwardToInternalRunner(p4, cloudLogger); err != nil {
			log.Errorf("error forwarding command: %v", err)
			return 1
		}
	}
	if err != nil {
		log.Errorf("error running %q: %v", cmd, err)
		return 1
	}
	return 0
}

func main() {
	os.Exit(internalMain())
}
