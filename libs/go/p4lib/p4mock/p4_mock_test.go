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

package p4mock

import (
	"fmt"
	"strings"
	"testing"

	"sge-monorepo/libs/go/p4lib"

	"github.com/google/go-cmp/cmp"
)

// This test is here to verify that the mock compiles. It won't when the P4 interface doesn't
// have a function implemented, so this will stop any changes to the interface that do not update
// the mock as well.
func TestMock(t *testing.T) {
	want := p4lib.Client{
		Client: "some-client",
		Root:   `C:\p4`,
	}

	p4Mock := New()
	p4Mock.ClientFunc = func(clientName string) (*p4lib.Client, error) {
		if clientName == want.Client {
			return &want, nil
		}
		return nil, fmt.Errorf("unexpected Client call for %s", clientName)
	}

	// Casting it to p4lib.P4 forces the compiler to evaluate whether it can.
	p4 := p4lib.P4(&p4Mock)
	got, err := p4.Client("some-client")
	if err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(*got, want); diff != "" {
		t.Errorf("want: %s, got: %s, diff: %s", want, got, diff)
	}

	// A non-set expectation should fail.
	wantErr := "InfoFunc not set"
	_, err = p4.Info()
	if err == nil || !strings.Contains(err.Error(), wantErr) {
		t.Errorf("expected info call to fail. want: %q, got: %s", wantErr, err)
	}
}
