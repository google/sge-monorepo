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
	"bytes"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"sge-monorepo/build/cicd/cirunner/runnertool"
	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/sgeb/build"
	"sge-monorepo/libs/go/email"
	"sge-monorepo/libs/go/p4lib"

	"sge-monorepo/build/cicd/cirunner/runners/publish_runner/protos/publishpb"
	"sge-monorepo/build/cicd/sgeb/protos/sgebpb"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
)

type autoPublishUnit struct {
	label       monorepo.Label
	dir         monorepo.Path
	publishUnit *sgebpb.PublishUnit
}

// Recursively searches the monorepo for any publish units with auto_publish set.
func discoverAutoPublishUnits(mr monorepo.Monorepo, bc build.Context) ([]autoPublishUnit, error) {
	bufs, err := build.DiscoverBuildUnitFiles(mr, bc)
	if err != nil {
		return nil, err
	}
	var ret []autoPublishUnit
	for _, buf := range bufs {
		for _, pu := range buf.Proto.PublishUnit {
			if pu.AutoPublish == nil {
				continue
			}
			label, err := mr.NewLabel(buf.Dir, ":"+pu.Name)
			if err != nil {
				return nil, err
			}
			ret = append(ret, autoPublishUnit{
				label:       label,
				dir:         buf.Dir,
				publishUnit: pu,
			})
		}
	}
	return ret, nil
}

func internalMain() error {
	flag.Parse()
	var helper runnertool.Helper
	if runnertool.HasInvocation() {
		helper = runnertool.MustLoad()
	}
	creds, err := runnertool.NewCredentials()
	if err != nil {
		return fmt.Errorf("could not obtain credentials: %v", err)
	}
	if creds.Email == nil {
		return fmt.Errorf("must have email credentials, none found")
	}
	emailClient := email.NewClientWithPlainAuth(
		creds.Email.Host,
		int(creds.Email.Port),
		creds.Email.Username,
		creds.Email.Password,
	)
	mr, _, err := monorepo.NewFromPwd()
	if err != nil {
		return err
	}
	bc, err := build.NewContext(mr)
	if err != nil {
		return err
	}
	defer bc.Cleanup()
	apus, err := discoverAutoPublishUnits(mr, bc)
	if err != nil {
		return err
	}
	p4 := p4lib.New()
	overallSuccess := true
	for _, apu := range apus {
		glog.Infof("publishing %s\n", apu.label)
		key := p4Key(apu.label)
		keyVal, err := p4.KeyGet(key)
		if err != nil {
			glog.Errorf("Failed to get p4 key %s: %v\n", key, err)
			overallSuccess = false
			continue
		}
		keyVal = strings.TrimSpace(keyVal)
		state := publishpb.PublishState{
			Success: true,
		}
		// p4 returns "0" when it doesn't have a key
		validKey := keyVal != "0"
		if validKey {
			if err := proto.UnmarshalText(keyVal, &state); err != nil {
				glog.Errorf("Invalid p4 key %s: %v\n", key, err)
				// If the format has changed we just discard the old value.
			}
		}
		var logs bytes.Buffer
		results, err := bc.Publish(apu.label, apu.publishUnit.AutoPublish.Args, func(opts *build.Options, publishOpts *build.PublishOptions) {
			opts.Logs = &logs
			opts.LogLevel = "INFO"
			if helper != nil {
				publishOpts.BaseCl = helper.Invocation().BaseCl
				publishOpts.CiResultUrl = helper.Invocation().Publish.ResultsUrl
				opts.BazelBuildArgs = append(opts.BazelBuildArgs,
					"--stamp",
					fmt.Sprintf("--workspace_status_command=workspace_status_command.exe -base_cl=%d", helper.Invocation().BaseCl),
				)
			}
		})
		success := err == nil
		if err != nil {
			glog.Errorf("Could not publish %s: %v\n", apu.label, err)
			glog.Error(logs.String())
		}
		if len(results) > 0 {
			for _, r := range results {
				glog.Infof("Published %s version %s (%d files published)\n", r.Name, r.Version, len(r.Files))
			}
		} else {
			glog.Info("Nothing to publish\n")
		}
		notifier := runnertool.NewNotifier(emailClient, sgebpb.NotificationPolicy_NOTIFY_ON_FAILURE_AND_RECOVERY, apu.publishUnit.AutoPublish.Notify)
		now := time.Now()
		if success {
			// Clear the last email time which ensures we send an email immediately
			// if we transition back to unhealthy state.
			state.LastEmailTime = nil
			if len(results) > 0 {
				wasHealthy := state.Success
				e := email.Email{
					ContentType: email.ContentTypeText,
					Subject:     fmt.Sprintf("%s was published successfully", apu.label),
					EmailBody:   fmt.Sprintf("%s was published successfully.", apu.label),
				}
				if err := notifier.OnSuccess(&e, wasHealthy); err != nil {
					glog.Errorf("Failed to send email: %v\n", err)
				}
			}
		} else if shouldSendFailEmail(now, state.LastEmailTime) {
			state.LastEmailTime = &timestamp.Timestamp{Seconds: now.Unix()}
			sb := strings.Builder{}
			sb.WriteString(fmt.Sprintf("The publish status of %s has become unhealthy.\n\n", apu.label))
			if helper != nil {
				sb.WriteString(fmt.Sprintf("CI results page: %s\n", helper.Invocation().Publish.ResultsUrl))
			}
			sb.WriteString(fmt.Sprintf("Error: %v\n", err))
			sb.WriteString(fmt.Sprintf("Logs:\n%s", logs.String()))

			e := email.Email{
				ContentType: email.ContentTypeText,
				Subject:     fmt.Sprintf("%s is UNHEALTHY", apu.label),
				EmailBody:   sb.String(),
			}
			if err := notifier.OnFailure(&e); err != nil {
				glog.Errorf("Failed to send email: %v\n", err)
			}
		}
		state.Success = success
		newKeyVal := proto.MarshalTextString(&state)
		if err := p4.KeySet(key, newKeyVal); err != nil {
			glog.Errorf("Failed to set p4 key %s: %v\n", key, err)
			overallSuccess = false
		}
	}
	if !overallSuccess {
		return &fail{}
	}
	return nil
}

func p4Key(label monorepo.Label) string {
	// p4 keys may not contain slashes, skip leading // and replace the rest with ':'
	// Example key: sge-publish:build:cicd:sgeb:sgeb_publish
	return fmt.Sprintf("sge-publish:%s", strings.ReplaceAll(label.String()[2:], "/", ":"))
}

func shouldSendFailEmail(now time.Time, ts *timestamp.Timestamp) bool {
	// Rate limit the emails.
	twoHours := int64(2 * 60 * 60)
	return ts == nil || (now.Unix()-ts.Seconds) >= twoHours
}

func main() {
	if err := internalMain(); err != nil {
		if !isFailErr(err) {
			fmt.Println(err)
		}
		os.Exit(1)
	}
}

type fail struct{}

func (*fail) Error() string {
	return "publish runner failed"
}

func isFailErr(err error) bool {
	_, ok := err.(*fail)
	return ok
}
