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

// Binary time_buildtool is meant to function as an arbitrary long target.

package main

import (
	"flag"
	"fmt"
	"time"

	"sge-monorepo/build/cicd/sgeb/buildtool"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
)

const (
	// updateInterval is how often we should show an update.
	updateInterval = 10
)

type conf struct {
	seconds int
	minutes int
	hours   int
	days    int
}

func main() {
	c := conf{}
	flag.IntVar(&c.seconds, "seconds", 0, "How many seconds should the buildtool wait")
	flag.IntVar(&c.minutes, "minutes", 0, "How many minutes should the buildtool wait")
	flag.IntVar(&c.hours, "hours", 0, "How many hours should the buildtool wait")
	flag.IntVar(&c.days, "days", 0, "How many days should the buildtool wait")
	flag.Parse()
	helper := buildtool.MustLoad()

	begin := time.Now()
	target := begin
	target = target.Add(time.Second * time.Duration(c.seconds))
	target = target.Add(time.Minute * time.Duration(c.minutes))
	target = target.Add(time.Hour * time.Duration(c.hours))
	target = target.Add(time.Hour * time.Duration(24*c.days))

	fmt.Printf("Running for %d day(s), %d hour(s), %d minute(s), %d second(s)\n", c.days, c.hours, c.minutes, c.seconds)
	for now := time.Now(); now.Before(target); now = time.Now() {
		elapsed := now.Sub(begin)
		remaining := target.Sub(now)
		fmt.Printf("%v: Time Elapsed: %v. Time remaining: %v\n", now, elapsed, remaining)

		nextUpdate := now.Add(time.Second * time.Duration(updateInterval))
		time.Sleep(nextUpdate.Sub(now))
	}
	helper.MustWriteBuildResult(&buildpb.BuildInvocationResult{
		Result: &buildpb.Result{
			Success: true,
		},
	})
}
