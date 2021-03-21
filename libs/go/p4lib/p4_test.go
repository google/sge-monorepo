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

// test p4lib functionality

package p4lib

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"sge-monorepo/libs/go/sgetest"

	"github.com/google/go-cmp/cmp"
)

func TestActions(t *testing.T) {
	testAtoS := []struct {
		input ActionType
		want  string
	}{
		{input: ActionAdd, want: "add"},
		{input: ActionArchive, want: "archive"},
		{input: ActionBranch, want: "branch"},
		{input: ActionDelete, want: "delete"},
		{input: ActionEdit, want: "edit"},
		{input: ActionIntegrate, want: "integrate"},
		{input: ActionMoveAdd, want: "move/add"},
		{input: ActionMoveDelete, want: "move/delete"},
		{input: ActionPurge, want: "purge"},
	}

	for _, ta := range testAtoS {
		a := ta.input.String()
		if a != ta.want {
			t.Errorf("failed to create action string. Got %q wanted %q", a, ta.want)
		}
	}

	for _, ta := range testAtoS {
		a, err := GetActionType(ta.want)
		if err != nil {
			t.Errorf("couldn't find action %v", err)
		} else if a != ta.input {
			t.Errorf("failed to parse action string. Got \"%v\" wanted \"%v\"", a, ta.input)
		}
	}

}

func TestChanges(t *testing.T) {
	testCases := []struct {
		stats map[string]string
		want  Change
	}{
		// vanilla
		{
			stats: map[string]string{
				"change": "7393",
				"user":   "cool-guy",
				"client": "cool-guy2-w_could-have-been",
				"desc":   "build trigger ",
				"time":   "1591056000",
			},
			want: Change{Cl: 7393, Client: "cool-guy2-w_could-have-been", Date: "2020/06/02 00:00:00", DateUnix: 1591056000, Description: "build trigger ", User: "cool-guy"},
		},
		// includes status of change (*pending*)
		{
			stats: map[string]string{
				"change": "8233",
				"user":   "egocentric-guy",
				"client": "egocentric-guy-could-have-been",
				"status": "pending",
				"desc":   "Support bazel test units in could-have-been",
				"time":   "1591660800",
			},
			want: Change{Cl: 8233, Client: "egocentric-guy-could-have-been", Date: "2020/06/09 00:00:00", DateUnix: 1591660800, Description: "Support bazel test units in could-have-been", Status: "pending", User: "egocentric-guy"},
		},
		// funky client name
		{
			stats: map[string]string{
				"change": "442",
				"user":   "cool-name",
				"client": "beehive-21748fdd-06db-a8ca-7f55-5eb315fb5805",
				"desc":   "[cool-project] Adding more generated fi",
				"time":   "1590019200",
			},
			want: Change{Cl: 442, Client: "beehive-21748fdd-06db-a8ca-7f55-5eb315fb5805", Date: "2020/05/21 00:00:00", DateUnix: 1590019200, Description: "[cool-project] Adding more generated fi", User: "cool-name"},
		},
		{
			stats: map[string]string{
				"change": "1",
				"user":   "abc",
				"client": "some-company-pacman-env-some-zone",
				"desc":   "move //some-depot/ue4/Release-4.2",
				"time":   "1589932800",
			},
			want: Change{Cl: 1, Client: "some-company-pacman-env-some-zone", Date: "2020/05/20 00:00:00", DateUnix: 1589932800, Description: "move //some-depot/ue4/Release-4.2", User: "abc"},
		},
		// -l long description format
		{
			stats: map[string]string{
				"change": "8090",
				"user":   "cool-guy",
				"client": "cool-guy2-w_could-have-been",
				"desc": `p4-lib: fstat & diff support
supports both diff (local vs server) and diff2 (server vs server) commands
unify diff query processing
support for fstat command and parsing metadata into structure
`,
				"time": "1591660800",
			},
			want: Change{Cl: 8090, Client: "cool-guy2-w_could-have-been", Date: "2020/06/09 00:00:00", DateUnix: 1591660800,
				Description: `p4-lib: fstat & diff support
supports both diff (local vs server) and diff2 (server vs server) commands
unify diff query processing
support for fstat command and parsing metadata into structure
`,
				Status: "", User: "cool-guy"},
		},
		// -t include time with date
		{
			stats: map[string]string{
				"change": "8274",
				"user":   "graphics-guy",
				"client": "graphics-guy_laptop",
				"status": "pending",
				"desc":   "adding bazel build project ",
				"time":   "1591753758",
			},
			want: Change{Cl: 8274, Client: "graphics-guy_laptop", Date: "2020/06/10 01:49:18", DateUnix: 1591753758, Description: "adding bazel build project ", Status: "pending", User: "graphics-guy"},
		},
	}

	for _, tc := range testCases {
		cb := changecb{}
		cb.outputStat(tc.stats)
		if len(cb) != 1 {
			t.Errorf("couldn't parse change %s", tc.stats)
		}
		if diff := cmp.Diff(cb[0], tc.want); diff != "" {
			t.Errorf("change parse error. got:\n%v, want:\n%v, diff (-want +got):\n%s", cb[0], tc.want, diff)
		}
	}

	combined := []map[string]string{}
	for _, tc := range testCases {
		combined = append(combined, tc.stats)
	}
	cb := changecb{}
	for _, stats := range combined {
		cb.outputStat(stats)
	}
	if len(testCases) != len(cb) {
		t.Errorf("wrong number of changes parsed. Got %d, expected %d", len(testCases), len(cb))
	}
	for i, tc := range testCases {
		if diff := cmp.Diff(cb[i], tc.want); diff != "" {
			t.Errorf("change parse error. got:\n%v, want:\n%v. Diff (-want +got):\n%s", cb[i], tc.want, diff)
		}
	}

	// No need to test empty since the outputStat callback won't be called in
	// that case.
}

func TestDescribe(t *testing.T) {
	testCases := []struct {
		stats []map[string]string
		want  []Description
	}{
		{
			stats: []map[string]string{
				{
					"change": "5663",
					"user":   "beehive",
					"client": "beehive-6a8b2bf0-522c-d817-e137-2799eb1756d5",
					"time":   fmt.Sprintf("%d", time.Date(2020, 5, 18, 22, 31, 45, 0, time.UTC).Unix()),
					"status": "pending",
					"desc":   `move amd beta drivers to //other-depot/third_party`,
				},
			},
			want: []Description{
				{
					Cl:          5663,
					User:        "beehive",
					Client:      "beehive-6a8b2bf0-522c-d817-e137-2799eb1756d5",
					Date:        "2020/05/18 22:31:45",
					DateUnix:    1589841105,
					Status:      "pending",
					Description: "move amd beta drivers to //other-depot/third_party",
				},
			},
		},
		{
			stats: []map[string]string{
				{
					"change":     "6000",
					"user":       "abc",
					"client":     "some-company-pacman-env-some-zone",
					"time":       fmt.Sprintf("%d", time.Date(2020, 5, 20, 17, 25, 35, 0, time.UTC).Unix()),
					"desc":       `move //some-depot/ue4/Release-4.24/Engine/Source/ThirdParty/ADO/... //other-depot/third_party/unreal/4.24/Engine/Source/ThirdParty/ADO/...`,
					"depotFile0": "//some-depot/ue4/Release-4.24/Engine/Source/ThirdParty/ADO/ADO.Build.cs",
					"rev0":       "2",
					"action0":    "move/delete",
					"depotFile1": "//some-depot/ue4/Release-4.24/Engine/Source/ThirdParty/ADO/ADO.tps",
					"rev1":       "2",
					"action1":    "move/delete",
					"depotFile2": "//other-depot/third_party/unreal/4.24/Engine/Source/ThirdParty/ADO/ADO.Build.cs",
					"rev2":       "1",
					"action2":    "move/add",
					"depotFile3": "//other-depot/third_party/unreal/4.24/Engine/Source/ThirdParty/ADO/ADO.tps",
					"rev3":       "1",
					"action3":    "move/add",
				},
			},
			want: []Description{
				{
					Cl:          6000,
					User:        "abc",
					Client:      "some-company-pacman-env-some-zone",
					Date:        "2020/05/20 17:25:35",
					DateUnix:    1589995535,
					Description: "move //some-depot/ue4/Release-4.24/Engine/Source/ThirdParty/ADO/... //other-depot/third_party/unreal/4.24/Engine/Source/ThirdParty/ADO/...",
					Files: []FileAction{
						{
							DepotPath: "//some-depot/ue4/Release-4.24/Engine/Source/ThirdParty/ADO/ADO.Build.cs",
							Revision:  2,
							Action:    "move/delete",
						},
						{
							DepotPath: "//some-depot/ue4/Release-4.24/Engine/Source/ThirdParty/ADO/ADO.tps",
							Revision:  2,
							Action:    "move/delete",
						},
						{
							DepotPath: "//other-depot/third_party/unreal/4.24/Engine/Source/ThirdParty/ADO/ADO.Build.cs",
							Revision:  1,
							Action:    "move/add",
						},
						{
							DepotPath: "//other-depot/third_party/unreal/4.24/Engine/Source/ThirdParty/ADO/ADO.tps",
							Revision:  1,
							Action:    "move/add",
						},
					},
				},
			},
		},
		{
			stats: []map[string]string{
				{
					"change": "9239",
					"user":   "cool-guy",
					"client": "cool-guy2-w_could-have-been",
					"time":   fmt.Sprintf("%d", time.Date(2020, 6, 18, 4, 29, 10, 0, time.UTC).Unix()),
					"desc": `p4lib: support for batching of p4describe operations
- update p4 api to return array of describe objects instead of pointer to single object'
- update mockapi to conform to new api
- add unit test featuring different describe results
- change test size to small as test suite was complaining`,
					"depotFile0": "//other-depot/libs/go/p4lib/BUILD",
					"rev0":       "4",
					"action0":    "edit",
					"depotFile1": "//other-depot/libs/go/p4lib/p4-lib-impl.go",
					"rev1":       "9",
					"action1":    "edit",
					"depotFile2": "//other-depot/libs/go/p4lib/p4-lib-test.go",
					"rev2":       "5",
					"action2":    "edit",
					"depotFile3": "//other-depot/libs/go/p4lib/p4-lib.go",
					"rev3":       "8",
					"action3":    "edit",
					"depotFile4": "//other-depot/libs/go/p4lib/p4mock/p4-mock.go",
					"rev4":       "7",
					"action4":    "edit",
				},
				{
					"change":     "9230",
					"user":       "cool-guy",
					"client":     "cool-guy2-w_could-have-been",
					"time":       fmt.Sprintf("%d", time.Date(2020, 6, 18, 2, 59, 52, 0, time.UTC).Unix()),
					"desc":       `beehive: add comments to structures. support creation of ballot for easy summaries of upvotes.`,
					"depotFile0": "//other-depot/libs/beehive-lib/go/beehive-lib.go",
					"rev0":       "3",
					"action0":    "edit",
				},
				{
					"change":     "9259",
					"user":       "egocentric-guy",
					"client":     "egocentric-guy-could-have-been",
					"time":       fmt.Sprintf("%d", time.Date(2020, 6, 18, 12, 24, 43, 0, time.UTC).Unix()),
					"desc":       `Delete build.bat. It is no longer needed.`,
					"depotFile0": "//other-depot/build/unreal-builder/unreal-builder-go/installation/build.bat",
					"rev0":       "2",
					"action0":    "edit",
				},
			},
			want: []Description{
				{
					Cl:       9239,
					User:     "cool-guy",
					Client:   "cool-guy2-w_could-have-been",
					Date:     "2020/06/18 04:29:10",
					DateUnix: 1592454550,
					Description: `p4lib: support for batching of p4describe operations
- update p4 api to return array of describe objects instead of pointer to single object'
- update mockapi to conform to new api
- add unit test featuring different describe results
- change test size to small as test suite was complaining`,
					Files: []FileAction{
						{
							DepotPath: "//other-depot/libs/go/p4lib/BUILD",
							Revision:  4,
							Action:    "edit",
						},
						{
							DepotPath: "//other-depot/libs/go/p4lib/p4-lib-impl.go",
							Revision:  9,
							Action:    "edit",
						},
						{
							DepotPath: "//other-depot/libs/go/p4lib/p4-lib-test.go",
							Revision:  5,
							Action:    "edit",
						},
						{
							DepotPath: "//other-depot/libs/go/p4lib/p4-lib.go",
							Revision:  8,
							Action:    "edit",
						},
						{
							DepotPath: "//other-depot/libs/go/p4lib/p4mock/p4-mock.go",
							Revision:  7,
							Action:    "edit",
						},
					},
				},
				{
					Cl:          9230,
					User:        "cool-guy",
					Client:      "cool-guy2-w_could-have-been",
					Date:        "2020/06/18 02:59:52",
					DateUnix:    1592449192,
					Description: `beehive: add comments to structures. support creation of ballot for easy summaries of upvotes.`,
					Files: []FileAction{
						{
							DepotPath: "//other-depot/libs/beehive-lib/go/beehive-lib.go",
							Revision:  3,
							Action:    "edit",
						},
					},
				},
				{
					Cl:          9259,
					User:        "egocentric-guy",
					Client:      "egocentric-guy-could-have-been",
					Date:        "2020/06/18 12:24:43",
					DateUnix:    1592483083,
					Description: `Delete build.bat. It is no longer needed.`,
					Files: []FileAction{
						{
							DepotPath: "//other-depot/build/unreal-builder/unreal-builder-go/installation/build.bat",
							Revision:  2,
							Action:    "edit",
						},
					},
				},
			},
		},
	}
	for _, tc := range testCases {
		descs := &describecb{}
		for _, stat := range tc.stats {
			err := descs.outputStat(stat)
			if err != nil {
				t.Errorf("Error parsing %v %v", stat, err)
			}
		}
		p4descs := make([]Description, 0, len(*descs))
		for _, desc := range *descs {
			p4descs = append(p4descs, desc)
		}
		if diff := cmp.Diff(p4descs, tc.want); diff != "" {
			t.Errorf("describe parse error. got:\n%v, want:\n%v. Diff (-want +got):\n%s", descs, tc.want, diff)
		}
	}
}

func TestFstat(t *testing.T) {
	testCases := []struct {
		stats []map[string]string
		want  FstatResult
	}{
		{
			stats: []map[string]string{
				{
					"depotFile": "//other-depot/libs/p4-lib/go/BUILD",
				},
			},
			want: FstatResult{
				FileStats: []FileStat{
					{
						DepotFile: "//other-depot/libs/p4-lib/go/BUILD",
					},
				},
			},
		},
		{
			stats: []map[string]string{
				{
					"depotFile":   "//other-depot/libs/p4-lib/go/p4-lib.go",
					"clientFile":  `d:\p4-could-have-been\shared\libs\p4-lib\go\p4-lib.go`,
					"isMapped":    "",
					"headAction":  "edit",
					"headType":    "text",
					"headTime":    "1591717743",
					"headRev":     "7",
					"headChange":  "8141",
					"headModTime": "1591709527",
					"haveRev":     "7",
					"action":      "edit",
					"change":      "8209",
					"type":        "text",
					"actionOwner": "cool-guy",
					"workRev":     "7",
				},
			},
			want: FstatResult{
				FileStats: []FileStat{
					{
						Action:      "edit",
						ActionOwner: "cool-guy",
						Change:      8209,
						ClientFile:  `d:\p4-could-have-been\shared\libs\p4-lib\go\p4-lib.go`,
						DepotFile:   "//other-depot/libs/p4-lib/go/p4-lib.go",
						IsMapped:    true,
						HeadAction:  "edit",
						HeadType:    "text",
						HeadTime:    1591717743,
						HeadRev:     7,
						HeadChange:  8141,
						HeadModTime: 1591709527,
						HaveRev:     7,
						Type:        "text",
						WorkRev:     7,
					},
				},
			},
		},
		{
			stats: []map[string]string{
				{
					"depotFile": "//file1.dat",
				},
				{
					"depotFile":  "//file2.dat",
					"headAction": "edit",
				},
				{
					"depotFile": "//file3.dat",
				},
				{
					"desc": "great changes afoot",
				},
			},
			want: FstatResult{
				FileStats: []FileStat{
					{
						DepotFile: "//file1.dat",
					},
					{
						DepotFile:  "//file2.dat",
						HeadAction: "edit",
					},
					{
						DepotFile: "//file3.dat",
					},
				},
				Desc: "great changes afoot",
			},
		},
		{
			stats: []map[string]string{
				{
					"depotFile":      "//this/is/a/file.ext",
					"otherLock0":     "",
					"otherLockOwner": "filehogger@workspace",
					"otherAction0":   "edit",
					"otherAction1":   "branch",
					"resolveAction1": "merge",
					"resolveAction0": "integrate",
				},
			},
			want: FstatResult{
				FileStats: []FileStat{
					{
						DepotFile:      "//this/is/a/file.ext",
						OtherLock0:     true,
						OtherLockOwner: "filehogger@workspace",
						OtherActions: []string{
							"edit",
							"branch",
						},
						ResolveActions: []string{
							"integrate",
							"merge",
						},
					},
				},
			},
		},
		// testing OtherOpen
		{
			stats: []map[string]string{
				{
					"depotFile":    "//other-depot/bin/windows/vendor-bender.exe",
					"clientFile":   `d:\p4-could-have-been\shared\bin\windows\vendor-bender.exe`,
					"isMapped":     "",
					"headAction":   "edit",
					"headType":     "binary+x",
					"headTime":     "1591233585",
					"headRev":      "3",
					"headChange":   "7617",
					"headModTime":  "1591231453",
					"haveRev":      "3",
					"otherOpen0":   "cloud-guy@cloud-guy_cloud-guy2-W_120",
					"otherAction0": "edit",
					"otherChange0": "8306",
					"otherOpen":    "1",
				},
				{
					"desc": "publishing vendor-bender.exe",
				},
			},
			want: FstatResult{
				FileStats: []FileStat{
					{
						ClientFile:  `d:\p4-could-have-been\shared\bin\windows\vendor-bender.exe`,
						DepotFile:   "//other-depot/bin/windows/vendor-bender.exe",
						IsMapped:    true,
						HeadAction:  "edit",
						HeadType:    "binary+x",
						HeadTime:    1591233585,
						HeadRev:     3,
						HeadChange:  7617,
						HeadModTime: 1591231453,
						HaveRev:     3,
						OtherOpen:   1,
						OtherOpens: []string{
							"cloud-guy@cloud-guy_cloud-guy2-W_120",
						},
						OtherChanges: []int{
							8306,
						},
						OtherActions: []string{
							"edit",
						},
					},
				},
				Desc: "publishing vendor-bender.exe",
			},
		},
	}

	for _, tc := range testCases {
		fs := &FstatResult{}
		for _, stat := range tc.stats {
			fs.outputStat(stat)
		}
		if diff := cmp.Diff(*fs, tc.want); diff != "" {
			t.Errorf("fstat parse error (%v). Diff (-want +got):\n%s", tc.stats, diff)
		}
	}

}

func TestActionTypeLen(t *testing.T) {
	if len(ActionNames) != ActionLen {
		t.Errorf("wrong action names length. want %d, got %d", len(ActionNames), ActionLen)
	}
}

func TestStdoutOption(t *testing.T) {
	var output bytes.Buffer

	p4 := New()
	gotOutput, err := p4.ExecCmdWithOptions([]string{"THIS-WILL-FAIL"}, OutputOption(&output))
	if err == nil {
		t.Errorf("expected an error, got none")
	} else if gotOutput == "" {
		t.Errorf("expected return output, got none")
	} else if output.String() == "" {
		t.Errorf("expected output in either stdout/stderr, got none")
	}
}

func TestLoadClient(t *testing.T) {
	content := `
#A Perforce Client Specification.
#
#  Client:      The client name.
#  Update:      The date this specification was last modified.
#  Access:      The date this client was last used in any way.
#  Owner:       The Perforce user name of the user who owns the client
#               workspace. The default is the user who created the
#               client workspace.
#  Host:        If set, restricts access to the named host.
#  Description: A short description of the client (optional).
#  Root:        The base directory of the client workspace.
#  AltRoots:    Up to two alternate client workspace roots.
#  Options:     Client options:
#                      [no]allwrite [no]clobber [no]compress
#                      [un]locked [no]modtime [no]rmdir
#  SubmitOptions:
#                      submitunchanged/submitunchanged+reopen
#                      revertunchanged/revertunchanged+reopen
#                      leaveunchanged/leaveunchanged+reopen
#  LineEnd:     Text file line endings on client: local/unix/mac/win/share.
#  Type:        Type of client: writeable/readonly/graph/partitioned.
#  ServerID:    If set, restricts access to the named server.
#  View:        Lines to map depot files into the client workspace.
#  ChangeView:  Lines to restrict depot files to specific changelists.
#  Stream:      The stream to which this client's view will be dedicated.
#               (Files in stream paths can be submitted only by dedicated
#               stream clients.) When this optional field is set, the
#               View field will be automatically replaced by a stream
#               view as the client spec is saved.
#  StreamAtChange:  A changelist number that sets a back-in-time view of a
#                   stream ( Stream field is required ).
#                   Changes cannot be submitted when this field is set.
#
# Use 'p4 help client' to see more about client views and options.

Client:	test-Client_123

Update:	2020/06/03 21:12:40

Access:	2020/06/09 13:20:59

Owner:	test-Owner

Host:	some-Host

Description:
	Description line 1
  Another sentence

Root:	C:\some\root

Options:	noallwrite noclobber nocompress unlocked

SubmitOptions:	submitunchanged

LineEnd:	local

ServerID:	some-Server-1

View:
	//1p/abcgame/dev/... //test-Client_123/1p/abcgame/dev/...
	//other-depot/... //test-Client_123/shared/...
	-//other-depot/third_party/unreal/4.24/... //test-Client_123/shared/third_party/unreal/4.24/...
	-//other-depot/third_party/unreal/4.25/... //test-Client_123/shared/third_party/unreal/4.25/...
	-//other-depot/third_party/unity/... //test-Client_123/shared/third_party/unity/...


`
	got, err := parseClient(content)
	if err != nil {
		t.Error(err)
		return
	}
	want := Client{
		Client:        "test-Client_123",
		Owner:         "test-Owner",
		Host:          "some-Host",
		Root:          `C:\some\root`,
		Description:   "\tDescription line 1\nAnother sentence\n\n",
		Options:       []ClientOption{NoAllWrite, NoClobber, NoCompress, Unlocked},
		SubmitOptions: []string{"submitunchanged"},
		LineEnd:       "local",
		ServerId:      "some-Server-1",
		View: []ViewEntry{
			{"//1p/abcgame/dev/...", "//test-Client_123/1p/abcgame/dev/..."},
			{"//other-depot/...", "//test-Client_123/shared/..."},
			{"-//other-depot/third_party/unreal/4.24/...", "//test-Client_123/shared/third_party/unreal/4.24/..."},
			{"-//other-depot/third_party/unreal/4.25/...", "//test-Client_123/shared/third_party/unreal/4.25/..."},
			{"-//other-depot/third_party/unity/...", "//test-Client_123/shared/third_party/unity/..."},
		},
	}
	if diff := cmp.Diff(want, *got); diff != "" {
		t.Errorf("wrong View (%s). Diff (-want +got):\n%s", *got, diff)
	}
	wantStr := `Client:	test-Client_123
Owner:	test-Owner
Host:	some-Host
Root:	C:\some\root
Options:	noallwrite noclobber nocompress unlocked
SubmitOptions:	submitunchanged
LineEnd:	local
ServerID:	some-Server-1
View:
	//1p/abcgame/dev/... //test-Client_123/1p/abcgame/dev/...
	//other-depot/... //test-Client_123/shared/...
	-//other-depot/third_party/unreal/4.24/... //test-Client_123/shared/third_party/unreal/4.24/...
	-//other-depot/third_party/unreal/4.25/... //test-Client_123/shared/third_party/unreal/4.25/...
	-//other-depot/third_party/unity/... //test-Client_123/shared/third_party/unity/...
`
	gotStr := got.String()
	if diff := cmp.Diff(gotStr, wantStr); diff != "" {
		t.Errorf("wrong client (%s). Diff (-want +got):\n%s", gotStr, diff)
	}
}

func TestAddClientOption(t *testing.T) {
	options := []ClientOption{}
	options, err := AppendClientOption(options, AllWrite)
	if err != nil {
		t.Fatal(err)
	}
	want := []ClientOption{AllWrite}
	if diff := cmp.Diff(options, want); diff != "" {
		t.Fatalf("Wrong client options. Diff (-want + got): %s\n", diff)
	}
	// Adding the same option should be no-op.
	options, err = AppendClientOption(options, AllWrite)
	if err != nil {
		t.Fatal(err)
	}
	want = []ClientOption{AllWrite}
	if diff := cmp.Diff(options, want); diff != "" {
		t.Fatalf("Wrong client options. Diff (-want + got): %s\n", diff)
	}
	// Adding another option should work.
	options, err = AppendClientOption(options, Unlocked)
	if err != nil {
		t.Fatal(err)
	}
	want = []ClientOption{AllWrite, Unlocked}
	if diff := cmp.Diff(options, want); diff != "" {
		t.Fatalf("Wrong client options. Diff (-want + got): %s\n", diff)
	}
	// Adding inverse should replace.
	options, err = AppendClientOption(options, Locked)
	if err != nil {
		t.Fatal(err)
	}
	want = []ClientOption{AllWrite, Locked}
	if diff := cmp.Diff(options, want); diff != "" {
		t.Fatalf("Wrong client options. Diff (-want + got): %s\n", diff)
	}
	options, err = AppendClientOption(options, NoAllWrite)
	if err != nil {
		t.Fatal(err)
	}
	want = []ClientOption{NoAllWrite, Locked}
	if diff := cmp.Diff(options, want); diff != "" {
		t.Fatalf("Wrong client options. Diff (-want + got): %s\n", diff)
	}
}

func TestClients(t *testing.T) {
	input := `
Client presubmit-05zy14-presubmits-presubmit-0 2020/08/15 root C:\path\ ''
Client presubmit-13lc7i-presubmits-presubmit-0 2020/08/17 root C:\path\ ''
Client presubmit-1dqiph-presubmits-presubmit-0 2020/08/14 root C:\path\ ''
Client presubmit-35jcut-presubmits-presubmit-0 2020/08/15 root C:\path\ ''
Client presubmit-8a98gx-presubmits-presubmit-0 2020/08/17 root C:\path\ ''
Client presubmit-a8faiq-presubmits-presubmit-0 2020/08/20 root C:\path\ ''
Client presubmit-b0xxdh-presubmits-presubmit-0 2020/08/13 root C:\path\ ''
Client presubmit-mw8czd-presubmits-presubmit-0 2020/08/20 root C:\path\ ''
Client presubmit-ntpiil-presubmits-presubmit-0 2020/08/14 root C:\path\ ''
Client presubmit-o2zgbi-presubmits-presubmit-0 2020/08/15 root C:\path\ ''
Client presubmit-p66hf5-presubmits-presubmit-0 2020/08/10 root C:\path\ ''
Client presubmit-qu4u9j-presubmits-presubmit-0 2020/08/10 root C:\path\ ''
Client presubmit-s6cd5m-presubmits-presubmit-0 2020/08/14 root C:\path\ ''
Client presubmit-shd5yg-presubmits-presubmit-0 2020/08/11 root C:\path\ ''
Client presubmit-sraoga-presubmits-presubmit-0 2020/08/10 root C:\path\ ''
Client presubmit-uor0oo-presubmits-presubmit-0 2020/08/10 root C:\path\ ''
Client presubmit-uw01g1-presubmits-presubmit-0 2020/08/20 root C:\path\ ''
Client presubmit-v196vh-presubmits-presubmit-0 2020/08/21 root C:\path\ ''
Client presubmit-w0vkh1-presubmits-presubmit-0 2020/08/17 root C:\path\ ''
Client presubmit-wfltnk-presubmits-presubmit-0 2020/08/11 root C:\path\ ''
Client presubmit-wu35um-presubmits-presubmit-0 2020/08/19 root C:\path\ ''
Client presubmit-xbtsjv-presubmits-presubmit-0 2020/08/12 root C:\path\ ''
Client presubmit-xjjgu7-presubmits-presubmit-0 2020/08/17 root C:\path\ ''
Client presubmit-xvm92a-presubmits-presubmit-0 2020/08/17 root C:\path\ ''
`
	got, err := parseClients(input)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"presubmit-05zy14-presubmits-presubmit-0",
		"presubmit-13lc7i-presubmits-presubmit-0",
		"presubmit-1dqiph-presubmits-presubmit-0",
		"presubmit-35jcut-presubmits-presubmit-0",
		"presubmit-8a98gx-presubmits-presubmit-0",
		"presubmit-a8faiq-presubmits-presubmit-0",
		"presubmit-b0xxdh-presubmits-presubmit-0",
		"presubmit-mw8czd-presubmits-presubmit-0",
		"presubmit-ntpiil-presubmits-presubmit-0",
		"presubmit-o2zgbi-presubmits-presubmit-0",
		"presubmit-p66hf5-presubmits-presubmit-0",
		"presubmit-qu4u9j-presubmits-presubmit-0",
		"presubmit-s6cd5m-presubmits-presubmit-0",
		"presubmit-shd5yg-presubmits-presubmit-0",
		"presubmit-sraoga-presubmits-presubmit-0",
		"presubmit-uor0oo-presubmits-presubmit-0",
		"presubmit-uw01g1-presubmits-presubmit-0",
		"presubmit-v196vh-presubmits-presubmit-0",
		"presubmit-w0vkh1-presubmits-presubmit-0",
		"presubmit-wfltnk-presubmits-presubmit-0",
		"presubmit-wu35um-presubmits-presubmit-0",
		"presubmit-xbtsjv-presubmits-presubmit-0",
		"presubmit-xjjgu7-presubmits-presubmit-0",
		"presubmit-xvm92a-presubmits-presubmit-0",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("wrong clients list. Diff (-want +got):\n%s", diff)
	}
}

func TestSyncSize(t *testing.T) {
	line := `Server network estimates: files added/updated/deleted=1234/5678/9012, bytes added/updated=10241024/20482048`
	got, err := syncSizeParse(line)
	if err != nil {
		t.Fatal(err)
	}
	want := SyncSize{
		FilesAdded:   1234,
		FilesUpdated: 5678,
		FilesDeleted: 9012,
		BytesAdded:   10241024,
		BytesDeleted: 20482048,
	}
	if diff := cmp.Diff(want, *got); diff != "" {
		t.Fatalf("wrong sync size. Diff (-want, +got):\n%s", diff)
	}
}

func TestHaveParse(t *testing.T) {
	data := `
//other-depot/libs/go/p4lib/BUILD#6 - C:\some\path\libs\go\p4lib\BUILD
//other-depot/libs/go/p4lib/p4.go#13 - C:\some\path\libs\go\p4lib\p4.go
//other-depot/libs/go/p4lib/p4_impl.go#5 - C:\some\path\libs\go\p4lib\p4_impl.go
//other-depot/libs/go/p4lib/p4_test.go#2 - C:\some\path\libs\go\p4lib\p4_test.go
//other-depot/libs/go/p4lib/p4mock/BUILD#4 - C:\some\path\libs\go\p4lib\p4mock\BUILD
//other-depot/libs/go/p4lib/p4mock/p4_mock.go#4 - C:\some\path\libs\go\p4lib\p4mock\p4_mock.go
//other-depot/libs/go/p4lib/p4mock/p4_mock_test.go#1 - C:\some\path\libs\go\p4lib\p4mock\p4_mock_test.go
//other-depot/libs/go/p4lib/experimental - File(s) not in client
`
	got, err := haveParse(data)
	if err != nil {
		t.Fatal(err)
	}
	want := []File{
		{DepotPath: "//other-depot/libs/go/p4lib/BUILD", Revision: 6, LocalPath: `C:\some\path\libs\go\p4lib\BUILD`},
		{DepotPath: "//other-depot/libs/go/p4lib/p4.go", Revision: 13, LocalPath: `C:\some\path\libs\go\p4lib\p4.go`},
		{DepotPath: "//other-depot/libs/go/p4lib/p4_impl.go", Revision: 5, LocalPath: `C:\some\path\libs\go\p4lib\p4_impl.go`},
		{DepotPath: "//other-depot/libs/go/p4lib/p4_test.go", Revision: 2, LocalPath: `C:\some\path\libs\go\p4lib\p4_test.go`},
		{DepotPath: "//other-depot/libs/go/p4lib/p4mock/BUILD", Revision: 4, LocalPath: `C:\some\path\libs\go\p4lib\p4mock\BUILD`},
		{DepotPath: "//other-depot/libs/go/p4lib/p4mock/p4_mock.go", Revision: 4, LocalPath: `C:\some\path\libs\go\p4lib\p4mock\p4_mock.go`},
		{DepotPath: "//other-depot/libs/go/p4lib/p4mock/p4_mock_test.go", Revision: 1, LocalPath: `C:\some\path\libs\go\p4lib\p4mock\p4_mock_test.go`},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("wrong have. Diff (-want, +got):\n%s", diff)
	}
}

func TestVerifyCL(t *testing.T) {
	testCases := []struct {
		clFiles     []FileAction
		clientFiles []File
		wantErr     string
	}{
		{
			clFiles: []FileAction{
				{DepotPath: "//foo/foo.go", Revision: 7, Action: "edit"},
				{DepotPath: "//foo/bar.go", Revision: 6, Action: "edit"},
			},
			clientFiles: []File{
				{DepotPath: "//foo/foo.go", Revision: 5},
				{DepotPath: "//foo/bar.go", Revision: 6},
			},
		},
		{
			// Note that the point of this test case is to show that a missing file is not a
			// failure case.
			clFiles: []FileAction{
				{DepotPath: "//foo/foo.go", Revision: 5, Action: "edit"},
				{DepotPath: "//foo/bar.go", Revision: 6, Action: "edit"},
			},
			clientFiles: []File{
				{DepotPath: "//foo/foo.go", Revision: 5},
			},
		},
		{
			clFiles: []FileAction{
				{DepotPath: "//foo/foo.go", Revision: 5, Action: "edit"},
				{DepotPath: "//foo/bar.go", Revision: 6, Action: "edit"},
			},
			clientFiles: []File{
				{DepotPath: "//foo/foo.go", Revision: 5},
				{DepotPath: "//foo/bar.go", Revision: 7},
			},
			wantErr: "Change contains out of date files",
		},
		{
			clFiles: []FileAction{
				{DepotPath: "//foo/foo.go", Revision: 5, Action: "edit"},
				{DepotPath: "//foo/bar.go", Revision: 6, Action: "delete"},
			},
			clientFiles: []File{
				{DepotPath: "//foo/foo.go", Revision: 5},
			},
		},
	}

	for _, tc := range testCases {
		err := verifyCL(tc.clFiles, tc.clientFiles)
		if err := sgetest.CmpErr(err, tc.wantErr); err != nil {
			t.Error(err)
		}
	}
}
