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
	"fmt"
	"os"
	"os/user"
	"strings"
	"time"

    "sge-monorepo/libs/go/p4lib"
)

func buildDateP4(t time.Time) string {
	return fmt.Sprintf("@%d/%d/%d", t.Year(), int(t.Month()), t.Day())
}

func buildSnippets() error {
	// get username without domain
	current, err := user.Current()
	if err != nil {
		return err
	}
	lastSlash := strings.LastIndex(current.Username, "\\") + 1
	username := string(current.Username[lastSlash:])

	//	calculate monday-sunday date range for snippet CLs
	now := time.Now()
	weekday := ((int(now.Weekday()) + 5) % 7) + 1

	monday := now.AddDate(0, 0, -weekday+1)
	sunday := monday.AddDate(0, 0, 7)
	dateRange := fmt.Sprintf("%s,%s", buildDateP4(monday), buildDateP4(sunday))

    p4 := p4lib.New()
	changes, err := p4.Changes("-s", "submitted", "-u", username, "-l", dateRange)
	if err != nil {
		return err
	}

	for _, c := range changes {
		fmt.Printf("\n* [change %d] %s", c.Cl, c.Description)
	}

	return nil
}

func main() {
	if err := buildSnippets(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
