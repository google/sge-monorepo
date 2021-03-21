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

// Package mockclock mocks the clock.
package mockclock

import (
	"time"

	"sge-monorepo/libs/go/clock"
)

var _ clock.Clock = (*MockClock)(nil)

// MockClock is a mock that mocks the clock.
type MockClock struct {
	time time.Time
}

func (mc *MockClock) Now() time.Time {
	return mc.time
}

// SetTime sets the current time.
func (mc *MockClock) SetTime(t time.Time) {
	mc.time = t
}

// Advance moves the clock forwards.
func (mc *MockClock) Advance(seconds int) {
	mc.time = mc.time.Add(time.Second * time.Duration(seconds))
}

// New returns a new mock clock.
func New() *MockClock {
	return &MockClock{}
}
