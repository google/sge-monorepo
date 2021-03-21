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

	"sge-monorepo/build/cicd/cirunner/runners/cron_runner/protos/cronpb"
	"sge-monorepo/build/cicd/sgeb/protos/sgebpb"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
)

type cronUnit struct {
	label monorepo.Label
	dir   monorepo.Path
	pb    *sgebpb.CronUnit
}

type triggeredCronUnit struct {
	cronUnit
	state *cronpb.CronState
}

// Recursively searches the monorepo for any cron units with cron config set.
func discoverCronUnits(mr monorepo.Monorepo, bc build.Context) ([]cronUnit, error) {
	start := time.Now()
	defer func() {
		fmt.Printf("Discovery phase took %v\n", time.Since(start))
	}()
	bufs, err := build.DiscoverBuildUnitFiles(mr, bc)
	if err != nil {
		return nil, err
	}
	var ret []cronUnit
	for _, buf := range bufs {
		for _, cu := range buf.Proto.CronUnit {
			// nil config indicates that the cron unit is disabled.
			if cu.Config == nil {
				continue
			}
			label, err := mr.NewLabel(buf.Dir, ":"+cu.Name)
			if err != nil {
				return nil, err
			}
			ret = append(ret, cronUnit{
				label: label,
				dir:   buf.Dir,
				pb:    cu,
			})
		}
	}
	return ret, nil
}

func internalMain() error {
	flag.Parse()
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
	cus, err := discoverCronUnits(mr, bc)
	if err != nil {
		return err
	}
	fmt.Printf("Discovered %d cron units\n", len(cus))
	p4 := p4lib.New()
	overallSuccess := true
	var triggered []triggeredCronUnit
	for _, cu := range cus {
		key := p4Key(cu.label)
		keyVal, err := p4.KeyGet(key)
		if err != nil {
			glog.Errorf("Failed to get p4 key %s: %v\n", key, err)
			overallSuccess = false
			continue
		}
		keyVal = strings.TrimSpace(keyVal)
		state := cronpb.CronState{
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
		now := time.Now()
		if !shouldRunCronUnit(cu, now, &state) {
			continue
		}
		triggered = append(triggered, triggeredCronUnit{
			cronUnit: cu,
			state:    &state,
		})
	}
	glog.Infof("%d/%d cron units triggered\n", len(triggered), len(cus))

	for _, cu := range triggered {
		now := time.Now()
		notifier := runnertool.NewNotifier(emailClient, sgebpb.NotificationPolicy_NOTIFY_ON_FAILURE_AND_RECOVERY, cu.pb.Config.Notify)
		glog.Infof("running cron unit %s\n", cu.label)
		var logs bytes.Buffer
		err = bc.RunCron(cu.label, cu.pb.Args, func(opts *build.Options) {
			opts.Logs = &logs
			opts.LogLevel = "INFO"
		})
		if err == nil {
			wasHealthy := cu.state.Success
			e := email.Email{
				ContentType: email.ContentTypeText,
				Subject:     fmt.Sprintf("%s ran successfully", cu.label),
				EmailBody:   fmt.Sprintf("cron unit %s ran successfully.\nLogs: \n%s", cu.label, logs.String()),
			}
			if err := notifier.OnSuccess(&e, wasHealthy); err != nil {
				glog.Errorf("Failed to send email: %v\n", err)
				overallSuccess = false
			}
		} else {
			glog.Errorf("Failed to run cron job %s: %v\n", cu.label, err)
			glog.Error(logs.String())
			e := email.Email{
				ContentType: email.ContentTypeText,
				Subject:     fmt.Sprintf("%s failed to run", cu.label),
				EmailBody:   fmt.Sprintf("cron unit %s failed to run.\n\nError: %v\nLogs: \n%s", cu.label, err, logs.String()),
			}
			if err := notifier.OnFailure(&e); err != nil {
				glog.Errorf("Failed to send email: %v\n", err)
				overallSuccess = false
			}
		}
		state := cronpb.CronState{
			LastCronTime: &timestamp.Timestamp{
				Seconds: now.Unix(),
			},
			Success: err == nil,
		}
		key := p4Key(cu.label)
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

func shouldRunCronUnit(cu cronUnit, now time.Time, state *cronpb.CronState) bool {
	if cu.pb.Config == nil {
		return false
	}
	if state.LastCronTime == nil {
		return true
	}
	return now.Unix() >= state.LastCronTime.Seconds+cu.pb.Config.FrequencyMinutes*60
}

func p4Key(label monorepo.Label) string {
	// p4 keys may not contain slashes, skip leading // and replace the rest with ':'
	// Example key: sge-cron:build:cicd:sgeb:cron
	return fmt.Sprintf("sge-cron:%s", strings.ReplaceAll(label.String()[2:], "/", ":"))
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
	return "cron runner failed"
}

func isFailErr(err error) bool {
	_, ok := err.(*fail)
	return ok
}
