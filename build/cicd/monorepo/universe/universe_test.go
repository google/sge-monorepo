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

package universe

import (
	"fmt"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/p4lib/p4mock"
	"sge-monorepo/libs/go/sgetest"
)

var correctUniverse = Def{
	{
		Name: "some-project",
		Root: "//my-projects/some-project/dev",
		Excludes: []string{
			"ue4/intermediate/...",
			"ue4/UE4.sln",
			"shared/shared-projects/...",
		},
	},
	{
		Name: "shared",
		Root: "//shared",
		Excludes: []string{
			"shared-projects/unreal/ue4.24/...",
			"shared-projects/unreal/ue4.25/...",
			"shared-projects/unity/...",
			"experimental/...",
		},
	},
}

var incorrectUniverse = Def{
	{
		Name: "some-project",
		Root: "//my-projects/some-project/dev",
	},
	{
		Name: "some-project-services",
		Root: "//my-projects/some-project/dev/services",
	},
}

func TestNewUniverse(t *testing.T) {
	testCases := []struct {
		desc     string
		universe Def
		wantErr  string
	}{
		{
			desc:     "correct universe",
			universe: correctUniverse,
		},
		{
			desc:     "incorrect universe",
			universe: incorrectUniverse,
			wantErr:  "monorepo is contained by another monorepo",
		},
	}
	for _, tc := range testCases {
		_, err := NewFromDef(tc.universe)
		if err := sgetest.CmpErr(err, tc.wantErr); err != nil {
			t.Errorf("[%s]: %v", tc.desc, err)
		}
	}
}

func TestCreatePerforceViewFromUniverse(t *testing.T) {
	p4 := p4mock.New()
	wantClientName := "TEST-CLIENT-NAME"
	p4.ClientFunc = func(clientName string) (*p4lib.Client, error) {
		if clientName != wantClientName {
			return nil, fmt.Errorf("want ClientName %q, got %q", wantClientName, clientName)
		}

		return &p4lib.Client{
			Client: wantClientName,
			Root:   `C:\ws`,
		}, nil
	}

	u, err := NewFromDef(correctUniverse)
	if err != nil {
		t.Fatal(err)
	}
	client, err := u.createP4View(p4, wantClientName)
	if err != nil {
		t.Fatal(err)
	}
	got := client.View
	sort.Slice(got, func(i, j int) bool {
		return got[i].Source < got[j].Source
	})

	want := []p4lib.ViewEntry{
		{Source: "-//my-projects/some-project/dev/shared/shared-projects/...", Destination: "//TEST-CLIENT-NAME/my-projects/some-project/dev/shared/shared-projects/..."},
		{Source: "-//my-projects/some-project/dev/ue4/UE4.sln", Destination: "//TEST-CLIENT-NAME/my-projects/some-project/dev/ue4/UE4.sln"},
		{Source: "-//my-projects/some-project/dev/ue4/intermediate/...", Destination: "//TEST-CLIENT-NAME/my-projects/some-project/dev/ue4/intermediate/..."},

		{Source: "-//shared/experimental/...", Destination: "//TEST-CLIENT-NAME/shared/experimental/..."},
		{Source: "-//shared/shared-projects/unity/...", Destination: "//TEST-CLIENT-NAME/shared/shared-projects/unity/..."},
		{Source: "-//shared/shared-projects/unreal/ue4.24/...", Destination: "//TEST-CLIENT-NAME/shared/shared-projects/unreal/ue4.24/..."},
		{Source: "-//shared/shared-projects/unreal/ue4.25/...", Destination: "//TEST-CLIENT-NAME/shared/shared-projects/unreal/ue4.25/..."},

		{Source: "//my-projects/some-project/dev/...", Destination: "//TEST-CLIENT-NAME/my-projects/some-project/dev/..."},
		{Source: "//shared/...", Destination: "//TEST-CLIENT-NAME/shared/..."},
	}

	if diff := cmp.Diff(got, want); diff != "" {
		t.Errorf("got: %s, want: %s, diff: %s", got, want, diff)
	}
}

func TestUpdateClientFromUniverse(t *testing.T) {
	wantClientName := "TEST-CLIENT-NAME"
	wantView := []p4lib.ViewEntry{
		{Source: "-//my-projects/some-project/dev/shared/shared-projects/...", Destination: "//TEST-CLIENT-NAME/my-projects/some-project/dev/shared/shared-projects/..."},
		{Source: "-//my-projects/some-project/dev/ue4/UE4.sln", Destination: "//TEST-CLIENT-NAME/my-projects/some-project/dev/ue4/UE4.sln"},
		{Source: "-//my-projects/some-project/dev/ue4/intermediate/...", Destination: "//TEST-CLIENT-NAME/my-projects/some-project/dev/ue4/intermediate/..."},

		{Source: "-//shared/experimental/...", Destination: "//TEST-CLIENT-NAME/shared/experimental/..."},
		{Source: "-//shared/shared-projects/unity/...", Destination: "//TEST-CLIENT-NAME/shared/shared-projects/unity/..."},
		{Source: "-//shared/shared-projects/unreal/ue4.24/...", Destination: "//TEST-CLIENT-NAME/shared/shared-projects/unreal/ue4.24/..."},
		{Source: "-//shared/shared-projects/unreal/ue4.25/...", Destination: "//TEST-CLIENT-NAME/shared/shared-projects/unreal/ue4.25/..."},

		{Source: "//my-projects/some-project/dev/...", Destination: "//TEST-CLIENT-NAME/my-projects/some-project/dev/..."},
		{Source: "//shared/...", Destination: "//TEST-CLIENT-NAME/shared/..."},
	}

	p4 := p4mock.New()
	p4.ClientFunc = func(clientName string) (*p4lib.Client, error) {
		if clientName != wantClientName {
			return nil, fmt.Errorf("want ClientName %q, got %q", wantClientName, clientName)
		}
		return &p4lib.Client{
			Client: wantClientName,
			Root:   `C:\ws`,
		}, nil
	}
	p4.ClientSetFunc = func(client *p4lib.Client) (string, error) {
		if client.Client != wantClientName {
			return "", fmt.Errorf("want clientName %q, got %q", wantClientName, client.Client)
		}
		got := client.View
		sort.Slice(got, func(i, j int) bool {
			return got[i].Source < got[j].Source
		})
		if diff := cmp.Diff(got, wantView); diff != "" {
			err := fmt.Errorf("got: %s, want: %s, diff: %s", got, wantView, diff)
			t.Error(err)
			return "", err
		}
		return "OK", nil
	}
	u, err := NewFromDef(correctUniverse)

	if err != nil {
		t.Error(err)
	} else if err := u.UpdateClientView(p4, wantClientName); err != nil {
		t.Error(err)
	}
}
