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
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"sge-monorepo/build/cicd/cirunner/runnertool"
	"sge-monorepo/build/cicd/jenkins"
	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/monorepo/p4path"
	"sge-monorepo/build/cicd/sgeb/build"
	"sge-monorepo/environment/envinstall"
	"sge-monorepo/libs/go/clock"
	"sge-monorepo/libs/go/email"
	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/p4lib"

	"sge-monorepo/build/cicd/cirunner/runners/postsubmit_runner/protos/postsubmitpb"
	"sge-monorepo/build/cicd/cirunner/runners/unit_runner/protos/unit_runnerpb"
	"sge-monorepo/build/cicd/sgeb/protos/sgebpb"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
	gouuid "github.com/nu7hatch/gouuid"
)

type postSubmitKind int

const (
	publishKind postSubmitKind = iota
	testKind                   = 1
	taskKind                   = 2
)

type postSubmitUnit struct {
	label      string
	dir        monorepo.Path
	kind       postSubmitKind
	postSubmit *sgebpb.PostSubmit
}

const (
	// Timeout until we give up queuing a task
	timeoutTaskQueueSeconds = 5 * 60
	// Task timeout
	timeoutTaskSecondsDefault = 2 * 60 * 60
	// Time between failure and retry.
	retrySeconds = 2 * 60 * 60
)

// runner is the postsubmit runner
type runner struct {
	p4            p4lib.P4
	emailClient   email.Client
	jenkinsRemote jenkins.Remote
	baseCL        int64
	clock         clock.Clock
	env           envinstall.EnvironmentType
}

func newRunner() (*runner, error) {
	r := &runner{}
	r.p4 = p4lib.New()
	r.clock = clock.New()
	if runnertool.HasInvocation() {
		helper := runnertool.MustLoad()
		r.baseCL = helper.Invocation().BaseCl
	}
	creds, err := runnertool.NewCredentials()
	if err != nil {
		return nil, fmt.Errorf("could not obtain credentials: %v", err)
	}
	if creds.Email == nil {
		return nil, fmt.Errorf("must have email credentials, none found")
	}
	r.emailClient = email.NewClientWithPlainAuth(
		creds.Email.Host,
		int(creds.Email.Port),
		creds.Email.Username,
		creds.Email.Password,
	)
	if jenkinsCreds, err := jenkins.CredentialsForProject(""); err != nil {
		return nil, fmt.Errorf("could not get Jenkins credentials: %w", err)
	} else {
		r.jenkinsRemote = jenkins.NewRemote(jenkinsCreds)
	}
	if r.env, err = envinstall.Environment(); err != nil {
		return nil, fmt.Errorf("could not get environment: %v", err)
	}
	return r, nil
}

func (r *runner) run() error {
	mr, _, err := monorepo.NewFromPwd()
	if err != nil {
		return err
	}
	bc, err := build.NewContext(mr)
	if err != nil {
		return err
	}
	defer bc.Cleanup()
	log.Info("Discovering postsubmits...")
	pus, err := discoverPostSubmits(mr, bc)
	if err != nil {
		return err
	}
	log.Infof("%d postsubmits found", len(pus))

	changedFiles, err := r.findChangedFiles(mr)
	if err != nil {
		return err
	}

	success := true
	for _, p := range pus {
		if err := r.processPostSubmitUnit(mr, p, changedFiles); err != nil {
			log.Error(err)
			success = false
		}
	}
	if !success {
		return &fail{}
	}
	return nil
}

func (r *runner) findChangedFiles(mr monorepo.Monorepo) ([]monorepo.Path, error) {
	if r.baseCL == 0 {
		return nil, nil
	}
	start := time.Now()
	key := fmt.Sprintf("sge-postsubmit-%s-last-cl", r.env)
	lastClStr, err := r.p4.KeyGet(key)
	if err != nil {
		return nil, fmt.Errorf("could not get CL key: %v", err)
	}
	var paths []monorepo.Path
	// "0" means no key, or the first run.
	if lastClStr != "0" {
		lastCl, err := strconv.Atoi(lastClStr)
		if err != nil {
			return nil, fmt.Errorf("invalid CL key %q: %v", lastClStr, err)
		}
		log.Infof("finding changed files from %d to %d", lastCl+1, r.baseCL)
		fileDetails, err := r.p4.Files(fmt.Sprintf("//...@%d,%d", lastCl+1, r.baseCL))
		if err != nil && !errors.Is(err, p4lib.ErrFileNotFound) {
			return nil, fmt.Errorf("could not issue p4 files: %v", err)
		}
		// Batch calls to p4 where to improve performance for large diffs.
		var batches [][]p4lib.FileDetails
		rem := fileDetails
		for len(rem) > 0 {
			L := 100
			if len(rem) < L {
				L = len(rem)
			}
			batches = append(batches, rem[0:L])
			rem = rem[L:]
		}
		for _, batch := range batches {
			var batchPaths []string
			for _, f := range batch {
				batchPaths = append(batchPaths, f.DepotFile)
			}
			absPaths, err := r.p4.WhereEx(batchPaths)
			if err != nil {
				log.Warningf("could not p4 where: %v", err)
				continue
			}
			for _, absPath := range absPaths {
				p, err := mr.RelPath(absPath)
				if err != nil {
					log.Warningf("could not find monorepo path for %q: %v", absPath, err)
					continue
				}
				paths = append(paths, p)
			}
		}
	}
	if err := r.p4.KeySet(key, strconv.Itoa(int(r.baseCL))); err != nil {
		return nil, fmt.Errorf("could not set CL key: %v", err)
	}
	log.Infof("%d changed files found", len(paths))
	if len(paths) <= 20 {
		for _, f := range paths {
			log.Infof("  %s", f)
		}
	} else {
		log.Info("too many files to print")
	}
	log.Infof("findChangedFiles took %s", time.Since(start))
	return paths, nil
}

// discoverPostSubmits searches the monorepo for any units with postsubmit set.
func discoverPostSubmits(mr monorepo.Monorepo, bc build.Context) ([]postSubmitUnit, error) {
	start := time.Now()
	bufs, err := build.DiscoverBuildUnitFiles(mr, bc)
	if err != nil {
		return nil, err
	}
	var ret []postSubmitUnit
	for _, buf := range bufs {
		for _, pu := range buf.Proto.PublishUnit {
			if pu.PostSubmit == nil {
				continue
			}
			label, err := mr.NewLabel(buf.Dir, ":"+pu.Name)
			if err != nil {
				return nil, err
			}
			ret = append(ret, postSubmitUnit{
				label:      label.String(),
				dir:        buf.Dir,
				kind:       publishKind,
				postSubmit: pu.PostSubmit,
			})
		}
		for _, tu := range buf.Proto.TestUnit {
			if tu.PostSubmit == nil {
				continue
			}
			label, err := mr.NewLabel(buf.Dir, ":"+tu.Name)
			if err != nil {
				return nil, err
			}
			ret = append(ret, postSubmitUnit{
				label:      label.String(),
				dir:        buf.Dir,
				kind:       testKind,
				postSubmit: tu.PostSubmit,
			})
		}
		for _, tu := range buf.Proto.TaskUnit {
			if tu.PostSubmit == nil {
				continue
			}
			label, err := mr.NewLabel(buf.Dir, ":"+tu.Name)
			if err != nil {
				return nil, err
			}
			ret = append(ret, postSubmitUnit{
				label:      label.String(),
				dir:        buf.Dir,
				kind:       taskKind,
				postSubmit: tu.PostSubmit,
			})
		}
	}
	log.Infof("discoverPostSubmits took %s", time.Since(start))
	return ret, nil
}

func (r *runner) processPostSubmitUnit(mr monorepo.Monorepo, p postSubmitUnit, changedFiles []monorepo.Path) error {
	log.Infof("%s: processing", p.label)
	key := r.stateKey(p.label)
	state, err := readState(r.p4, key)
	if err != nil {
		return err
	}
	if err := r.updateState(mr, state, p, changedFiles); err != nil {
		return err
	}
	if state.Dirty {
		state.Dirty = false
		if err := writeState(r.p4, key, state); err != nil {
			return err
		}
	}
	return nil
}

func (r *runner) updateState(mr monorepo.Monorepo, state *postsubmitpb.PostSubmitState, p postSubmitUnit, changedFiles []monorepo.Path) error {
	now := r.clock.Now().In(time.UTC)
	switch state.Status {
	case postsubmitpb.PostSubmitStatus_SUCCESS:
		if triggered, err := r.checkTriggered(mr, state, now, p, changedFiles); err != nil {
			return err
		} else if !triggered {
			return nil
		}
		log.Infof("%s: triggered, start postsubmit action", p.label)
	case postsubmitpb.PostSubmitStatus_FAILED:
		if state.NextRetry != nil && now.Unix() < state.NextRetry.Seconds {
			log.Infof("%s: in a failed state but not ready to retry yet", p.label)
			return nil
		}
		state.NextRetry = nil
		state.Dirty = true
		log.Infof("%s: retrying", p.label)
	case postsubmitpb.PostSubmitStatus_PENDING:
		if err := r.processPendingTask(state, now, p); err != nil {
			return err
		}
		// We do not want to run postsubmit again immediately, so return here.
		return nil
	}
	if err := r.createPostSubmitTask(state, now, p); err != nil {
		return err
	}
	return nil
}

func (r *runner) checkTriggered(mr monorepo.Monorepo, state *postsubmitpb.PostSubmitState, now time.Time, p postSubmitUnit, changedFiles []monorepo.Path) (bool, error) {
	triggered := false
	if p.postSubmit.TriggerPaths != nil {
		set, err := p4path.NewExprSet(mr, p.dir, p.postSubmit.TriggerPaths.Path)
		if err != nil {
			return false, fmt.Errorf("invalid trigger paths for %s: %v", p.label, err)
		}
		match := false
		for _, changedFile := range changedFiles {
			if m, err := set.Matches(changedFile); err != nil {
				return false, err
			} else if m {
				match = true
				break
			}
		}
		if match {
			log.Infof("%s: Triggered by p4 changes", p.label)
			triggered = true
		} else {
			log.Infof("%s: No triggering path found", p.label)
		}
	}
	if p.postSubmit.Frequency != nil {
		dailyAtUtc := p.postSubmit.Frequency.DailyAtUtc
		if dailyAtUtc != "" {
			colonIdx := strings.Index(dailyAtUtc, ":")
			if colonIdx == -1 {
				return false, fmt.Errorf("invalid daily_at_utc value: %q. Format must be HH:00", dailyAtUtc)
			}
			hourString := dailyAtUtc[0:colonIdx]
			hour, err := strconv.Atoi(hourString)
			if err != nil || hour < 0 || hour > 23 {
				return false, fmt.Errorf("invalid daily_at_utc value: %q. Format must be HH:00", dailyAtUtc)
			}
			h := now.Hour()
			if h != hour {
				log.Infof("%s: executed daily, but not time yet", p.label)
			} else if state.LastPostsubmitTime != nil && (now.Sub(timeFromTimestamp(state.LastPostsubmitTime)).Hours() < 4) {
				log.Infof("%s: Already executed recently, skipping", p.label)
			} else {
				log.Infof("%s: starting daily postsubmit", p.label)
				triggered = true
			}
		}
	}
	if p.postSubmit.TriggerAlwaysForTesting {
		triggered = true
	}
	return triggered, nil
}

func (r *runner) createPostSubmitTask(state *postsubmitpb.PostSubmitState, now time.Time, p postSubmitUnit) error {
	// If we reach here, it's time to issue a postsubmit request.
	taskKey, err := newTaskKey()
	if err != nil {
		return err
	}
	taskOpts := func(opts *jenkins.UnitOptions) {
		opts.BaseCl = int(r.baseCL)
		opts.TaskKey = taskKey
		opts.LogLevel = "INFO"
		opts.Args = p.postSubmit.Args
	}
	log.Infof("%s: sending postsubmit request with key %s", p.label, taskKey)
	switch p.kind {
	case publishKind:
		if err := r.jenkinsRemote.SendPublishRequest(p.label, taskOpts); err != nil {
			return fmt.Errorf("could not send publish request: %w", err)
		}
	case testKind:
		if err := r.jenkinsRemote.SendTestRequest(p.label, taskOpts); err != nil {
			return fmt.Errorf("could not send test request: %w", err)
		}
	case taskKind:
		if err := r.jenkinsRemote.SendTaskRequest(p.label, taskOpts); err != nil {
			return fmt.Errorf("could not send task request: %w", err)
		}
	}
	state.Status = postsubmitpb.PostSubmitStatus_PENDING
	state.Task = &postsubmitpb.Task{
		Key:       taskKey,
		StartTime: timestampFromTime(now),
		Cl:        r.baseCL,
	}
	state.PendingPathTrigger = false // No longer used, but clear to eventually allow us to remove field from proto.
	state.Dirty = true
	return nil
}

func (r *runner) processPendingTask(state *postsubmitpb.PostSubmitState, now time.Time, p postSubmitUnit) error {
	// We have a unit_runner task pending. Check if it's done and process the result if so.
	task, ok, err := readTask(r.p4, state.Task.Key)
	isTimeOut := false
	if err != nil {
		return fmt.Errorf("could not read task for %s: %v", p.label, err)
	} else if !ok {
		duration := now.Unix() - state.Task.StartTime.Seconds
		log.Infof("%s: Waiting for task acknowledgement, in queue for %ds", p.label, duration)
		if duration < timeoutTaskQueueSeconds {
			return nil
		}
		log.Infof("%s: stuck in task queue for too long, retrying", p.label)
		return r.createPostSubmitTask(state, now, p)
	} else if task.Status == unit_runnerpb.TaskStatus_RUNNING {
		duration := now.Unix() - state.Task.StartTime.Seconds
		log.Infof("%s: waiting for task to complete, ran for %ds. Logs: %s", p.label, duration, task.ResultsUrl)
		timeoutSeconds := int64(p.postSubmit.TimeoutMinutes * 60)
		if timeoutSeconds == 0 {
			timeoutSeconds = timeoutTaskSecondsDefault
		}
		if duration < timeoutSeconds {
			return nil
		}
		// Move it into the failed state, job timed out.
		log.Infof("%s: task timed out", p.label)
		task.Status = unit_runnerpb.TaskStatus_FAILED
		isTimeOut = true
	}
	notifier := runnertool.NewNotifier(r.emailClient, sgebpb.NotificationPolicy_NOTIFY_ALWAYS, p.postSubmit.Notify)
	if task.Status == unit_runnerpb.TaskStatus_SUCCESS {
		log.Infof("%s: postsubmit succeeded", p.label)
		wasHealthy := state.Success
		state.Status = postsubmitpb.PostSubmitStatus_SUCCESS
		state.Success = true
		state.LastPostsubmitCl = state.Task.Cl
		state.LastPostsubmitTime = timestampFromTime(now)
		state.Dirty = true
		sb := strings.Builder{}
		sb.WriteString(fmt.Sprintf("%s postsubmit ran successfully.\n", p.label))
		sb.WriteString(fmt.Sprintf("CI results page: %s\n", task.ResultsUrl))
		e := email.Email{
			ContentType: email.ContentTypeText,
			Subject:     fmt.Sprintf("%s postsubmit ran successfully", p.label),
			EmailBody:   sb.String(),
		}
		if err := notifier.OnSuccess(&e, wasHealthy); err != nil {
			log.Errorf("Failed to send email: %v\n", err)
		}
	} else {
		log.Infof("%s: failed to build, logs at %s", p.label, task.ResultsUrl)
		state.Status = postsubmitpb.PostSubmitStatus_FAILED
		state.Success = false
		state.NextRetry = timestampFromTime(now.Add(time.Second * retrySeconds))
		state.Dirty = true
		sb := strings.Builder{}
		sb.WriteString(fmt.Sprintf("%s failed to run postsubmit.\n\n", p.label))
		if isTimeOut {
			sb.WriteString("The task timed out.\n")
		}
		sb.WriteString(fmt.Sprintf("CI results page: %s\n", task.ResultsUrl))
		sb.WriteString(fmt.Sprintf("Retrying in %d minutes.\n", retrySeconds/60))
		e := email.Email{
			ContentType: email.ContentTypeText,
			Subject:     fmt.Sprintf("%s FAILED", p.label),
			EmailBody:   sb.String(),
		}
		if err := notifier.OnFailure(&e); err != nil {
			log.Errorf("Failed to send email: %v\n", err)
		}
	}
	state.Task = nil
	state.Dirty = true
	return nil
}

// stateKey returns a p4 key for the postsubmit state.
func (r *runner) stateKey(label string) string {
	// p4 keys may not contain slashes, skip leading // and replace the rest with ':'
	// Example key: sge-postsubmit:build:cicd:sgeb:publish
	return fmt.Sprintf("sge-postsubmit-%s:%s", r.env, strings.ReplaceAll(label[2:], "/", ":"))
}

func readState(p4 p4lib.P4, key string) (*postsubmitpb.PostSubmitState, error) {
	keyVal, err := p4.KeyGet(key)
	if err != nil {
		return nil, fmt.Errorf("failed to get p4 key %s: %v\n", key, err)
	}
	keyVal = strings.TrimSpace(keyVal)
	state := &postsubmitpb.PostSubmitState{}
	// p4 returns "0" when it doesn't have a key
	validKey := keyVal != "0"
	if validKey {
		if err := proto.UnmarshalText(keyVal, state); err != nil {
			log.Errorf("Invalid p4 key %s: %v\n", key, err)
			// If the format has changed we just discard the old value.
		} else {
			// Migrate to new proto names
			if state.LastPostsubmitCl == 0 {
				state.LastPostsubmitCl = state.LastPublishedCl
			}
			if state.LastPostsubmitTime == nil {
				state.LastPostsubmitTime = state.LastPublishTime
			}
		}
	}
	return state, nil
}

func writeState(p4 p4lib.P4, key string, state *postsubmitpb.PostSubmitState) error {
	newKeyVal := proto.MarshalTextString(state)
	if err := p4.KeySet(key, newKeyVal); err != nil {
		return fmt.Errorf("failed to set p4 key %s: %v\n", key, err)
	}
	return nil
}

// newTaskKey constructs a new unique unit_runner task key.
func newTaskKey() (string, error) {
	uuid, err := gouuid.NewV4()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("sge-postsubmit-task:%s", uuid.String()), nil
}

func readTask(p4 p4lib.P4, key string) (*unit_runnerpb.Task, bool, error) {
	if val, err := p4.KeyGet(key); err != nil {
		return nil, false, err
	} else if val == "0" {
		return nil, false, nil
	} else {
		ret := &unit_runnerpb.Task{}
		if err := proto.UnmarshalText(val, ret); err != nil {
			return nil, false, err
		}
		return ret, true, nil
	}
}

func timeFromTimestamp(t *timestamp.Timestamp) time.Time {
	return time.Unix(t.Seconds, 0)
}

func timestampFromTime(t time.Time) *timestamp.Timestamp {
	return &timestamp.Timestamp{
		Seconds: t.Unix(),
	}
}

type fail struct{}

func (*fail) Error() string {
	return "postsubmit runner failed"
}

func isFailErr(err error) bool {
	_, ok := err.(*fail)
	return ok
}

func internalMain() error {
	flag.Parse()
	log.AddSink(log.NewGlog())
	defer log.Shutdown()
	log.Info("postsubmit runner starting")

	r, err := newRunner()
	if err != nil {
		return err
	}
	return r.run()
}

func main() {
	if err := internalMain(); err != nil {
		if !isFailErr(err) {
			fmt.Println(err)
		}
		os.Exit(1)
	}
}
