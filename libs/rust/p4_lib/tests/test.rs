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

use error_lib::*;
use p4_lib::*;
use std::cell::RefCell;

#[derive(Default)]
struct PerforceMock {
    // we use a refcell here to give the mock interior mutability
    // this means we can use it even in trait functions that use immutable references
    inputs: RefCell<Vec<cool-companyResult<String>>>,
}

// the perforce mock interface is used by passing a slice of inputs
// you then invoke the perforce operation and will pop inputs in the exec function
impl PerforceMock {
    fn new(inputs: &[&cool-companyResult<String>]) -> Self {
        let v: Vec<cool-companyResult<String>> = inputs.iter().map(|&r| r.to_owned()).collect();
        PerforceMock {
            inputs: RefCell::from(v),
        }
    }
}

impl PerforceTrait for PerforceMock {
    // perforce mock exec function. instead of actually executing p4, it will return a prebacked stdout string
    // you can sequence this with a slice of strings for functions that repeatedly call exec()
    fn exec(&self, _args: &[&str]) -> cool-companyResult<String> {
        if let Some(result) = self.inputs.borrow_mut().pop() {
            return result;
        }
        Err(cool-companyError::Literal("not enough inputs in mock"))
    }
}

#[test]
fn test_changes() {
    do_test_changes();
}

fn do_test_changes() {
    struct ChangeTestItem {
        input: cool-companyResult<String>,
        want: cool-companyResult<Vec<Change>>,
    };

    let items : &[ChangeTestItem] = &[ ChangeTestItem {
        input: Ok(r#"Change 9395 on 1997/06/20 by cool-guy@cool-guy2-w_cool-company *pending* 'p4 lib rust 2 '
Change 9346 on 1997/06/19 by beehive@beehive-3a2c3885-f56e-c6b7-7034-363230f06114 *pending* 'Add check proto and implement g'
Change 9252 on 1997/06/18 by da-mastah@da-mastah_da-mastah2-WS '[cicd] Glob maching for PathExp'
Change 8970 on 1997/06/15 by egoistic-but-true@egoistic-but-true-cool-company 'Remove unused dep. '"#.into()),
		want: Ok(vec![
			Change{
				changelist: 9395,
				client: "cool-guy2-w_cool-company".into(),
				date: "1997/06/20".into(),
				description: "p4 lib rust 2 ".into(),
				status: "pending".into(),
				user: "cool-guy".into(),
			},
			Change{
				changelist: 9346,
				client: "beehive-3a2c3885-f56e-c6b7-7034-363230f06114".into(),
				date: "1997/06/19".into(),
				description: "Add check proto and implement g".into(),
				status: "pending".into(),
				user: "beehive".into(),
			},
			Change{
				changelist: 9252,
				client: "da-mastah_da-mastah2-WS".into(),
				date: "1997/06/18".into(),
				description: "[cicd] Glob maching for PathExp".into(),
				user: "da-mastah".into(),
				..Default::default()
			},
			Change{
				changelist: 8970,
				client: "egoistic-but-true-cool-company".into(),
				date: "1997/06/15".into(),
				description: "Remove unused dep. ".into(),
				user: "egoistic-but-true".into(),
				..Default::default()
			},
		])
	},

	ChangeTestItem {
		input: Ok(r#"Change 8209 on 1997/06/09 by cool-guy@cool-guy2-w_cool-company *pending*

	some-project: unit tests for changes command

Change 8141 on 1997/06/09 by cool-guy@cool-guy2-w_cool-company

	some-project: support for p4 describe, command batching for Sizes and Dirs, and optimize grep

Change 8090 on 1997/06/09 by cool-guy@cool-guy2-w_cool-company

	some-project: fstat & diff support
	supports both diff (local vs server) and diff2 (server vs server) commands
	unify diff query processing
	support for fstat command and parsing metadata into structure

Change 7988 on 1997/06/07 by cool-guy@cool-guy2-w_cool-company

	some-project: support perforce diff2 command

"#.into()),
		want: Ok(vec![
			Change{
				changelist: 8209,
				client: "cool-guy2-w_cool-company".into(),
				date: "1997/06/09".into(),
				description: "some-project: unit tests for changes command".into(),
				status: "pending".into(),
				user: "cool-guy".into(),
			},
			Change{
				changelist: 8141,
				client: "cool-guy2-w_cool-company".into(),
				date: "1997/06/09".into(),
				description: "some-project: support for p4 describe, command batching for Sizes and Dirs, and optimize grep".into(),
				user: "cool-guy".into(),
				..Default::default()
			},
			Change{
				changelist: 8090,
				client: "cool-guy2-w_cool-company".into(),
				date: "1997/06/09".into(),
				description: r#"some-project: fstat & diff support
supports both diff (local vs server) and diff2 (server vs server) commands
unify diff query processing
support for fstat command and parsing metadata into structure"#.into(),
				user: "cool-guy".into(),
				..Default::default()
			},
			Change{
				changelist: 7988,
				client: "cool-guy2-w_cool-company".into(),
				date: "1997/06/07".into(),
				description: "some-project: support perforce diff2 command".into(),
				user: "cool-guy".into(),
				..Default::default()
			},
		])
	}];

    for d in items {
        let p = PerforceMock::new(&[&d.input]);
        let c = p.changes(&[""]);
        assert_eq!(c, d.want);
    }
}

#[test]
fn test_client() {
    do_test_client();
}

fn do_test_client() {
    struct ClientTestItem {
        input: cool-companyResult<String>,
        want: cool-companyResult<Client>,
    };

    let obj = ClientTestItem {
        input: Ok(r#"# A Perforce Client Specification.
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

Client:	utf8

Owner:	cool-guy

Host:	cool-guy2-W

Description:
	Created by cool-guy.

Root:	d:\p4-cool-company\shared

Options:	noallwrite noclobber nocompress unlocked nomodtime normdir

SubmitOptions:	submitunchanged

LineEnd:	local

View:
	//another-depot/... //utf8/external/...
	//experimental/... //utf8/experimental/...
	//.beehive/... //utf8/.beehive/...

"#
        .to_string()),
        want: Ok(Client {
            client: "utf8".to_string(),
            description: "Created by cool-guy.".to_string(),
            host: "cool-guy2-W".to_string(),
            line_end: "local".to_string(),
            options: vec![
                "noallwrite",
                "noclobber",
                "nocompress",
                "unlocked",
                "nomodtime",
                "normdir",
            ]
            .iter()
            .map(|&s| s.to_string())
            .collect(),
            owner: "cool-guy".to_string(),
            root: r#"d:\p4-cool-company\shared"#.to_string(),
            submit_options: vec!["submitunchanged".to_string()],
            view: vec![
                ViewEntry {
                    source: "//another-depot/...".to_string(),
                    destination: "//utf8/external/...".to_string(),
                },
                ViewEntry {
                    source: "//experimental/...".to_string(),
                    destination: "//utf8/experimental/...".to_string(),
                },
                ViewEntry {
                    source: "//.beehive/...".to_string(),
                    destination: "//utf8/.beehive/...".to_string(),
                },
            ],
            ..Default::default()
        }),
    };

    let p = PerforceMock::new(&[&obj.input]);
    let c = p.client("");

    assert_eq!(c, obj.want);
}

#[test]
fn test_describe() {
    do_test_describe();
}

fn do_test_describe() {
    struct DescribeTestItem {
        input: cool-companyResult<String>,
        want: cool-companyResult<Vec<Description>>,
    };

    let items : &[DescribeTestItem] = &[ DescribeTestItem {
		input:Ok(r#"Change 5663 by beehive@beehive-6a8b2bf0-522c-d817-e137-2799eb1756d5 on 1997/05/18 22:31:45 *pending*

	move amd beta drivers to //some-depot/third_party

Affected files ...

"#.to_string()),
		want: Ok(vec![Description{
			changelist: 5663,
			client: "beehive-6a8b2bf0-522c-d817-e137-2799eb1756d5".into(),
			date: "1997/05/18 22:31:45".into(),
			description: r#"move amd beta drivers to //some-depot/third_party"#.into(),
			status: "pending".into(),
			user: "beehive".into(),
			..Default::default()
		}]),
	},

	DescribeTestItem {
		input:Ok(r#"Change 6000 by super@da-server-p4-edge-some-region-a on 1997/05/20 17:25:35

	move //another-depot/ue4/Release-4.24/Engine/Source/ThirdParty/ADO/... //some-depot/third_party/unreal/4.24/Engine/Source/ThirdParty/ADO/...

Affected files ...

... //another-depot/ue4/Release-4.24/Engine/Source/ThirdParty/ADO/ADO.Build.cs#2 move/delete
... //another-depot/ue4/Release-4.24/Engine/Source/ThirdParty/ADO/ADO.tps#2 move/delete
... //some-depot/third_party/unreal/4.24/Engine/Source/ThirdParty/ADO/ADO.Build.cs#1 move/add
... //some-depot/third_party/unreal/4.24/Engine/Source/ThirdParty/ADO/ADO.tps#1 move/add

"#.to_string()),
		want: Ok(vec![Description{
			changelist: 6000,
			client: "da-server-p4-edge-some-region-a".into(),
			date: "1997/05/20 17:25:35".into(),
			description: r#"move //another-depot/ue4/Release-4.24/Engine/Source/ThirdParty/ADO/... //some-depot/third_party/unreal/4.24/Engine/Source/ThirdParty/ADO/..."#.into(),
			user: "super".into(),
			files: vec![
				FileAction{
					depot_file: "//another-depot/ue4/Release-4.24/Engine/Source/ThirdParty/ADO/ADO.Build.cs".into(),
					revision: "2".into(),
					action: "move/delete".into(),
				},
				FileAction{
					depot_file: "//another-depot/ue4/Release-4.24/Engine/Source/ThirdParty/ADO/ADO.tps".into(),
					revision: "2".into(),
					action: "move/delete".into(),
				},
				FileAction{
					depot_file: "//some-depot/third_party/unreal/4.24/Engine/Source/ThirdParty/ADO/ADO.Build.cs".into(),
					revision: "1".into(),
					action: "move/add".into(),
				},
				FileAction{
					depot_file: "//some-depot/third_party/unreal/4.24/Engine/Source/ThirdParty/ADO/ADO.tps".into(),
					revision: "1".into(),
					action: "move/add".into(),
				},
			],
			..Default::default()
		}]),
	},


	DescribeTestItem {
		input:Ok(r#"Change 9239 by cool-guy@cool-guy2-w_cool-company on 1997/06/18 04:29:10

	some-project: support for batching of p4describe operations
	- update p4 api to return array of describe objects instead of pointer to single object'
	- update mockapi to conform to new api
	- add unit test featuring different describe results
	- change test size to small as test suite was complaining

Affected files ...

... //some-depot/some/path/BUILD#4 edit
... //some-depot/some/path/some-project-impl.go#9 edit
... //some-depot/some/path/some-project-test.go#5 edit
... //some-depot/some/path/some-project.go#8 edit
... //some-depot/some/path/p4mock/p4-mock.go#7 edit

Change 9230 by cool-guy@cool-guy2-w_cool-company on 1997/06/18 02:59:52

	beehive: add comments to structures. support creation of ballot for easy summaries of upvotes.

Affected files ...

... //some-depot/libs/beehive-lib/go/beehive-lib.go#3 edit

Change 9259 by egoistic-but-true@egoistic-but-true-cool-company on 1997/06/18 12:24:43

	Delete build.bat. It is no longer needed.

Affected files ...

... //some-depot/build/big-builder/big-builder-go/installation/build.bat#2 edit

"#.to_string()),
		want: Ok(vec![Description{
			changelist: 9239,
			client: "cool-guy2-w_cool-company".into(),
			date: "1997/06/18 04:29:10".into(),
			description: r#"some-project: support for batching of p4describe operations
- update p4 api to return array of describe objects instead of pointer to single object'
- update mockapi to conform to new api
- add unit test featuring different describe results
- change test size to small as test suite was complaining"#.into(),
			user: "cool-guy".into(),
			files: vec![
				FileAction{
					depot_file: "//some-depot/some/path/BUILD".into(),
					revision: "4".into(),
					action: "edit".into(),
				},
				FileAction{
					depot_file: "//some-depot/some/path/some-project-impl.go".into(),
					revision: "9".into(),
					action: "edit".into(),
				},
				FileAction{
					depot_file: "//some-depot/some/path/some-project-test.go".into(),
					revision: "5".into(),
					action: "edit".into(),
				},
				FileAction{
					depot_file: "//some-depot/some/path/some-project.go".into(),
					revision: "8".into(),
					action: "edit".into(),
				},
				FileAction{
					depot_file: "//some-depot/some/path/p4mock/p4-mock.go".into(),
					revision: "7".into(),
					action: "edit".into(),
				},
			],
			..Default::default()
		},

		Description{
			changelist: 9230,
			client: "cool-guy2-w_cool-company".into(),
			date: "1997/06/18 02:59:52".into(),
			description: r#"beehive: add comments to structures. support creation of ballot for easy summaries of upvotes."#.into(),
			user: "cool-guy".into(),
			files: vec![
				FileAction{
					depot_file: "//some-depot/libs/beehive-lib/go/beehive-lib.go".into(),
					revision: "3".into(),
					action: "edit".into(),
				},
			],
			..Default::default()
		},

		Description{
			changelist: 9259,
			client: "egoistic-but-true-cool-company".into(),
			date: "1997/06/18 12:24:43".into(),
			description: r#"Delete build.bat. It is no longer needed."#.into(),
			user: "egoistic-but-true".into(),
			files: vec![
				FileAction{
					depot_file: "//some-depot/build/big-builder/big-builder-go/installation/build.bat".into(),
					revision: "2".into(),
					action: "edit".into(),
				},
			],
			..Default::default()
		},

		]),
	},


	];

    for d in items {
        let p = PerforceMock::new(&[&d.input]);
        let c = p.describe(&[5663]);
        assert_eq!(c, d.want);
    }
}

#[test]
fn test_diff() {
    do_test_diff();
}

fn do_test_diff() {
    struct DiffTestItem {
        input: cool-companyResult<String>,
        want: cool-companyResult<Vec<Diff>>,
    };

    let items : &[DiffTestItem] = &[ DiffTestItem {
		input: Ok(r#"==== //some-depot/some/path/some-project.go#3 (text) - //some-depot/some/path/some-project.go#4 (text) ==== content
64a65,68
> 	// SetClient commits the given client configuration into the server.
> 	// Whether there is an error or not, the command returns stdout/stderr.
> 	SetClient(client *P4Client) (string, error)
>
346a351,354
> func (p4 P4Impl) SetClient(client *P4Client) (string, error) {
> 	return p4SetClient(client)
> }
>
"#.into()),
		want: Ok(vec![
			Diff {
				left_line_start: 64,
				left_line_end: 64,
				right_line_start: 65,
				right_line_end: 68,
				diff_type: DiffType::Add,
			},
			Diff {
				left_line_start: 346,
				left_line_end: 346,
				right_line_start: 351,
				right_line_end: 354,
				diff_type: DiffType::Add,
			},
		])
	},
	];

    for d in items {
        let p = PerforceMock::new(&[&d.input]);
        let c = p.diff("a", "b");
        assert_eq!(c, d.want);
    }
}

#[test]
fn test_dirs() {
    do_test_dirs();
}

fn do_test_dirs() {
    struct DirsTestItem {
        input: cool-companyResult<String>,
        want: cool-companyResult<Vec<String>>,
    };

    let items: &[DirsTestItem] = &[DirsTestItem {
        input: Ok(r#"//some-depot/.vscode
//some-depot/build
//some-depot/libs
//some-depot/third_party

"#
        .into()),
        want: Ok(vec![
            "//some-depot/build".into(),
            "//some-depot/libs".into(),
            "//some-depot/third_party".into(),
        ]),
    }];

    for d in items {
        let p = PerforceMock::new(&[&d.input]);
        let c = p.dirs("");
        assert_eq!(c, d.want);
    }
}

#[test]
fn test_fstat() {
    do_test_fstat()
}

fn do_test_fstat() {
    struct DirsTestItem {
        input: cool-companyResult<String>,
        want: cool-companyResult<FstatResult>,
    };

    let items: &[DirsTestItem] = &[
        DirsTestItem {
            input: Ok(r#"... depotFile //some-depot/libs/some-project/go/some-project.go
... clientFile d:\p4-cool-company\shared\libs\some-project\go\some-project.go
... isMapped
... headAction edit
... headType text
... headTime 1591717743
... headRev 7
... headChange 8141
... headModTime 1591709527
... haveRev 7
... action edit
... change 8209
... type text
... actionOwner cool-guy
... workRev 7
"#
            .into()),
            want: Ok(FstatResult {
                fstats: vec![Fstat {
                    action: "edit".into(),
                    action_owner: "cool-guy".into(),
                    change: 8209,
                    client_file: r#"d:\p4-cool-company\shared\libs\some-project\go\some-project.go"#.into(),
                    depot_file: r#"//some-depot/libs/some-project/go/some-project.go"#.into(),
                    file_type: "text".into(),
                    have_rev: 7,
                    head_action: "edit".into(),
                    head_type: "text".into(),
                    head_mod_time: 1_591_709_527,
                    head_time: 1_591_717_743,
                    head_rev: 7,
                    head_change: 8141,
                    is_mapped: true,
                    work_rev: 7,

                    ..Default::default()
                }],
                ..Default::default()
            }),
        },
        DirsTestItem {
            input: Ok(r#"... depotFile //file1.dat

... depotFile //file2.dat
... headAction edit

... depotFile //file3.dat

... desc great changes afoot"#
                .into()),
            want: Ok(FstatResult {
                desc: "great changes afoot".into(),
                fstats: vec![
                    Fstat {
                        depot_file: "//file1.dat".into(),
                        ..Default::default()
                    },
                    Fstat {
                        depot_file: "//file2.dat".into(),
                        head_action: "edit".into(),
                        ..Default::default()
                    },
                    Fstat {
                        depot_file: "//file3.dat".into(),
                        ..Default::default()
                    },
                ],
                ..Default::default()
            }),
        },
        DirsTestItem {
            input: Ok(r#"... depotFile //this/is/a/file.ext
... otherLock
... ... otherLock0 filehogger@workspace
... ... otherOpen0 cloud-guy@cloud-guy_cloud-guy2-W_120
... ... otherOpen 1
... ... otherAction0 edit
... ... otherAction1 branch
... ... otherChange0 8306
... ... resolveAction1 merge
... ... resolveAction0 integrate"#
                .into()),
            want: Ok(FstatResult {
                fstats: vec![Fstat {
                    depot_file: "//this/is/a/file.ext".into(),
                    other_changes: vec![8306],
                    other_lock: true,
                    other_lock0: "filehogger@workspace".into(),
                    other_open: 1,
                    other_opens: vec!["cloud-guy@cloud-guy_cloud-guy2-W_120".into()],
                    other_actions: vec!["edit".into(), "branch".into()],
                    resolve_actions: vec!["integrate".into(), "merge".into()],
                    ..Default::default()
                }],
                ..Default::default()
            }),
        },
    ];

    for d in items {
        let p = PerforceMock::new(&[&d.input]);
        let c = p.fstat(&[""]);
        assert_eq!(c, d.want);
    }
}

#[test]
fn test_info() {
    do_test_info()
}

fn do_test_info() {
    struct InfoTestItem {
        input: cool-companyResult<String>,
        want: cool-companyResult<Info>,
    };

    let items: &[InfoTestItem] = &[
        InfoTestItem {
			input: Ok(r#"User name: cool-guy
Client name: cool-guy2-w_cool-company
Client host: cool-guy2-W
Client root: d:\p4-cool-company
Current directory: d:\p4-cool-company\shared\some\path
Peer address: 10.224.1.2:36280
Client address: 10.224.1.2
Server address: da-server-p4-edge-some-region-a.c.da-server.internal:1666
Server root: /some/path/cool-company-mon-edge-1
Server date: 1997/06/22 02:12:43 +0000 UTC
Server uptime: 324:36:08
Server version: SOME_VERSION
Server encryption: encrypted
Server cert expires: Apr 26 19:18:51 2021 GMT
ServerID: cool-company-mon-edge-1
Server services: edge-server
Replica of: ssl:cool-company-commit:1666
Changelist server: ssl:cool-company-commit:1666
Server license: none
Case Handling: sensitive
"#.into()),
			want : Ok(Info{
				case_handling: "sensitive".into(),
				client_name: "cool-guy2-w_cool-company".into(),
				client_host: "cool-guy2-W".into(),
				client_root: r#"d:\p4-cool-company"#.into(),
				current_directory: r#"d:\p4-cool-company\shared\some\path"#.into(),
				peer_address: "10.224.1.2:36280".into(),
				client_address: "10.224.1.2".into(),
				server_address: "da-server-p4-edge-some-region-a.c.da-server.internal:1666".into(),
				server_root: "/some/path/cool-company-mon-edge-1".into(),
				server_date: "1997/06/22 02:12:43 +0000 UTC".into(),
				server_uptime: "324:36:08".into(),
				server_version: "SOME_VERSION".into(),
				server_encryption: "encrypted".into(),
				server_cert_expires: "Apr 26 19:18:51 2021 GMT".into(),
				server_id: "cool-company-mon-edge-1".into(),
				server_services: "edge-server".into(),
				replica_of: "ssl:cool-company-commit:1666".into(),
				changelist_server: "ssl:cool-company-commit:1666".into(),
				server_license: "none".into(),
				user_name: "cool-guy".into(),
				})
			}
		];

    for d in items {
        let p = PerforceMock::new(&[&d.input]);
        let c = p.info();
        assert_eq!(c, d.want);
    }
}

#[test]
fn test_opened() {
    do_test_opened();
}

fn do_test_opened() {
    struct OpenedTestItem {
        input: cool-companyResult<String>,
        want: cool-companyResult<Vec<FileOpened>>,
    };

    let items: &[OpenedTestItem] = &[OpenedTestItem {
        input: Ok(r#"//some-depot/WORKSPACE#45 - edit default change (text)
//some-depot/build/build-dist/BUILD#2 - edit default change (text)
//some-depot/experimental/api_vulkan.rs#5 - edit change 6496 (text)
//some-depot/some/path/some-project.go#11 - edit change 9381 (text)
"#
        .into()),
        want: Ok(vec![
            FileOpened {
                changelist: 0,
                depot_file: "//some-depot/WORKSPACE".into(),
                revision: 45,
                action: "edit".into(),
                file_type: "text".into(),
            },
            FileOpened {
                changelist: 0,
                depot_file: "//some-depot/build/build-dist/BUILD".into(),
                revision: 2,
                action: "edit".into(),
                file_type: "text".into(),
            },
            FileOpened {
                changelist: 6496,
                depot_file: "//some-depot/experimental/api_vulkan.rs"
                    .into(),
                revision: 5,
                action: "edit".into(),
                file_type: "text".into(),
            },
            FileOpened {
                changelist: 9381,
                depot_file: "//some-depot/some/path/some-project.go".into(),
                revision: 11,
                action: "edit".into(),
                file_type: "text".into(),
            },
        ]),
    }];

    for d in items {
        let p = PerforceMock::new(&[&d.input]);
        let c = p.opened();
        assert_eq!(c, d.want);
    }
}

#[test]
fn test_sizes() {
    do_test_sizes();
}

fn do_test_sizes() {
    struct SizesTestItem {
        input: cool-companyResult<String>,
        want: cool-companyResult<SizeCollection>,
    };

    let items: &[SizesTestItem] = &[
        SizesTestItem {
            input: Ok(r#"//some-depot/tools/some-tool/file.go#2 2275 bytes
//some-depot/tools/some-tool/some-tool.go#2 7880 bytes
//some-depot/tools/incredible/Cargo.toml#1 226 bytes
"#
            .into()),
            want: Ok(SizeCollection {
                sizes: vec![
                    Size {
                        depot_path: "//some-depot/tools/some-tool/file.go".into(),
                        revision: 2,
                        file_size: 2275,
                    },
                    Size {
                        depot_path: "//some-depot/tools/some-tool/some-tool.go".into(),
                        revision: 2,
                        file_size: 7880,
                    },
                    Size {
                        depot_path: "//some-depot/tools/incredible/Cargo.toml".into(),
                        revision: 1,
                        file_size: 226,
                    },
                ],
                ..Default::default()
            }),
        },
        SizesTestItem {
            input: Ok(r#"//some-depot/tools/... 136 files 1840410 bytes
"#
            .into()),
            want: Ok(SizeCollection {
                depot_directory: "//some-depot/tools/...".into(),
                total_file_count: 136,
                total_file_size: 1840410,
                ..Default::default()
            }),
        },
    ];

    for d in items {
        let p = PerforceMock::new(&[&d.input]);
        let c = p.sizes(&[""]);
        assert_eq!(c, d.want);
    }
}

#[test]
fn test_tickets() {
    do_test_tickets();
}

fn do_test_tickets() {
    struct TicketsTestItem {
        input: cool-companyResult<String>,
        want: cool-companyResult<Vec<Ticket>>,
    };

    // note - this tickets contain purely ficiontal randomly generated data
    let items: &[TicketsTestItem] = &[
        TicketsTestItem {
            input: Ok(
                r#"localhost:FAKE_AUTH_ID (notrealuser) 64578c65C39CB79DB7DD1B86016f25A7
"#
                .into(),
            ),
            want: Ok(vec![Ticket {
                name: "localhost:FAKE_AUTH_ID".into(),
                user: "notrealuser".into(),
                id: "64578c65C39CB79DB7DD1B86016f25A7".into(),
            }]),
        },
        TicketsTestItem {
            input: Ok(
                r#"whereami:WHO_KNOWS (lostinspace) 3567515a4e2a83a866b384f9aa164a32
anotherplace:ANOTHER_TIME (theheroweneed) 4004383f55a5e5cb3fca694a92f2b0fe
finalcountdown:EUROPE (emerald) 7cc1b78f4573035f11682eb96a40d182
"#
                .into(),
            ),
            want: Ok(vec![
                Ticket {
                    name: "whereami:WHO_KNOWS".into(),
                    user: "lostinspace".into(),
                    id: "3567515a4e2a83a866b384f9aa164a32".into(),
                },
                Ticket {
                    name: "anotherplace:ANOTHER_TIME".into(),
                    user: "theheroweneed".into(),
                    id: "4004383f55a5e5cb3fca694a92f2b0fe".into(),
                },
                Ticket {
                    name: "finalcountdown:EUROPE".into(),
                    user: "emerald".into(),
                    id: "7cc1b78f4573035f11682eb96a40d182".into(),
                },
            ]),
        },
    ];

    for d in items {
        let p = PerforceMock::new(&[&d.input]);
        let c = p.tickets();
        assert_eq!(c, d.want);
    }
}
