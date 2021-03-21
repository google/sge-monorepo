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

import "time"

type gigantickGoroutine struct {
	name      string
	args      string
	startTime time.Time
	dur       time.Duration
	completed bool
}

// system for tracking and timing goroutines
func goRoutineRegister(ctx *gigantickContext, funcName string, args string) int {
	rout := gigantickGoroutine{
		name:      funcName,
		args:      args,
		startTime: time.Now(),
	}
	ctx.goRoutMutex.Lock()
	defer ctx.goRoutMutex.Unlock()
	ctx.goRoutDetails = append(ctx.goRoutDetails, rout)
	index := len(ctx.goRoutDetails) - 1
	return index
}

func goRoutineUnregister(ctx *gigantickContext, index int) {
	ctx.goRoutMutex.Lock()
	defer ctx.goRoutMutex.Unlock()
	ctx.goRoutDetails[index].dur = time.Now().Sub(ctx.goRoutDetails[index].startTime)
	ctx.goRoutDetails[index].completed = true
}
