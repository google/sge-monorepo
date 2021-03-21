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

// Package watcher is used to watch for Perforce or Swarm changes.
package watcher

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"sge-monorepo/libs/go/log"
	"sge-monorepo/tools/ebert/ebert"
	"sge-monorepo/tools/ebert/flags"
	"sge-monorepo/tools/ebert/handlers/trigger"
)

const (
	submittedInterval = 1 * time.Minute
	maxChangesPerPoll = 100
	lastCheckedClKey  = "ebert-last-submitted"
)

// Watch continuously scans for events that Ebert cares about.
// For now, we just watch for submitted changes so that we can resolve bugs.
func Watch(bgctx context.Context, ectx *ebert.Context) {
	lastChecked := 0
	max := fmt.Sprintf("%d", maxChangesPerPoll)
	submitted := time.NewTicker(submittedInterval)
	defer submitted.Stop()
	for {
		select {
		case <-bgctx.Done():
			return
		case <-submitted.C:
			// Handle newly submitted changes.
			old, err := ectx.P4.KeyGet(lastCheckedClKey)
			if err != nil {
				log.Errorf("failed to lookup last submitted, using %v: %v", lastChecked, err)
			}
			if i, err := strconv.Atoi(old); err == nil {
				if !flags.DevMode || i > lastChecked {
					// Since we don't update the key in dev mode, only update
					// lastChecked from the key if the key value is greater.
					lastChecked = i
				}
			}
			changes, err := ectx.P4.Changes("-r", "-s", "submitted", "-m", max, "-e", fmt.Sprintf("%d", lastChecked+1))
			if err != nil {
				log.Errorf("failed to retrieve changes: %v", err)
				continue
			}
			for _, change := range changes {
				go func(change int) {
					err := trigger.PostSubmit(ectx, change)
					if err != nil {
						log.Errorf("error processing submitted change %d: %v", change, err)
					}
				}(change.Cl)
				lastChecked = change.Cl
			}
			if flags.DevMode {
				// Don't update lastCheckedClKey if in dev mode.
				continue
			}
			if old == "0" {
				// Can't CAS when old value is '0'.
				err = ectx.P4.KeySet(lastCheckedClKey, fmt.Sprintf("%d", lastChecked))
			} else {
				err = ectx.P4.KeyCas(lastCheckedClKey, old, fmt.Sprintf("%d", lastChecked))
			}
			if err != nil {
				log.Warningf("failed to update last submitted: %v", err)
			}
		}
	}
}
