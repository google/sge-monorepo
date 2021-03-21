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

use lazy_static::*;
use regex::Regex;
use std::process::Command;

#[derive(Clone, Debug, Default, PartialEq)]
pub struct Change {
    pub changelist: u32,
    pub user: String,
    pub client: String,
    pub date: String,
    pub description: String,
    pub status: String,
}

#[derive(Debug, Default, PartialEq)]
pub struct Client {
    pub access: String,
    pub alt_roots: Vec<String>,
    pub change_view: Vec<String>,
    pub client: String,
    pub description: String,
    pub host: String,
    pub line_end: String,
    pub options: Vec<String>,
    pub owner: String,
    pub root: String,
    pub server_id: String,
    pub stream: String,
    pub stream_at_change: String,
    pub submit_options: Vec<String>,
    pub update: String,
    pub view: Vec<ViewEntry>,
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct Description {
    pub changelist: u32,
    pub client: String,
    pub date: String,
    pub description: String,
    pub status: String,
    pub user: String,
    pub files: Vec<FileAction>,
}

#[derive(Debug, PartialEq)]
pub enum DiffType {
    None,
    Add,
    Change,
    Delete,
}
impl Default for DiffType {
    fn default() -> Self {
        DiffType::None
    }
}

#[derive(Debug, Default, PartialEq)]
pub struct Diff {
    pub left_line_start: u32,
    pub left_line_end: u32,
    pub right_line_start: u32,
    pub right_line_end: u32,
    pub diff_type: DiffType,
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct FileAction {
    pub depot_file: String,
    pub revision: String,
    pub action: String,
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct FileOpened {
    pub action: String,
    pub changelist: u32,
    pub depot_file: String,
    pub file_type: String,
    pub revision: u32,
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct Fstat {
    pub action: String,
    pub action_owner: String,
    pub change: u32,
    pub charset: String,
    pub client_file: String,
    pub depot_file: String,
    pub digest: String,
    pub file_size: u64,
    pub file_type: String,
    pub have_rev: u32,
    pub head_action: String,
    pub head_change: u32,
    pub head_charset: String,
    pub head_mod_time: u32,
    pub head_rev: u32,
    pub head_time: u32,
    pub head_type: String,
    pub is_mapped: bool,
    pub moved_file: String,
    pub moved_rev: u32,
    pub other_actions: Vec<String>,
    pub other_changes: Vec<u32>,
    pub other_lock: bool,
    pub other_lock0: String,
    pub other_open: u32,
    pub other_opens: Vec<String>,
    pub our_lock: bool,
    pub path: String,
    pub reresolvable: u32,
    pub resolve_actions: Vec<String>,
    pub resolve_base_actions: Vec<String>,
    pub resolve_base_files: Vec<String>,
    pub resolve_base_revs: Vec<u32>,
    pub resolve_from_files: Vec<String>,
    pub resolve_start_from_revs: Vec<u32>,
    pub resolve_end_from_revs: Vec<u32>,
    pub resolved: u32,
    pub rev_time: u32,
    pub shelved: bool,
    pub unresolved: u32,
    pub work_rev: u32,
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct FstatResult {
    pub fstats: Vec<Fstat>,
    pub desc: String,
    pub total_file_count: u32,
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct Info {
    pub case_handling: String,
    pub changelist_server: String,
    pub client_address: String,
    pub client_name: String,
    pub client_host: String,
    pub client_root: String,
    pub current_directory: String,
    pub peer_address: String,
    pub replica_of: String,
    pub server_address: String,
    pub server_cert_expires: String,
    pub server_date: String,
    pub server_encryption: String,
    pub server_license: String,
    pub server_root: String,
    pub server_uptime: String,
    pub server_version: String,
    pub server_id: String,
    pub server_services: String,
    pub user_name: String,
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct Size {
    pub depot_path: String,
    pub revision: u32,
    pub file_size: u64,
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct SizeCollection {
    pub sizes: Vec<Size>,
    pub depot_directory: String,
    pub total_file_count: u64,
    pub total_file_size: u64,
}

#[derive(Debug, Default, PartialEq)]
pub struct ViewEntry {
    pub source: String,
    pub destination: String,
}

#[derive(Clone, Debug, Default, PartialEq)]
pub struct Ticket {
    pub name: String,
    pub user: String,
    pub id: String,
}

impl ViewEntry {
    fn new(line: &str) -> Self {
        let s: Vec<&str> = line.split_whitespace().collect();
        if s.len() == 2 {
            ViewEntry {
                source: s[0].to_string(),
                destination: s[1].to_string(),
            }
        } else {
            Default::default()
        }
    }
}

#[derive(Default)]
pub struct Perforce {}

// Multiline iterator is a helper for iterating over perforce output
// in general, perforce output is in "key: value" pairs
// some fields span multiple lines, with tab starting each additional line
// this helper makes it easier to parse output that mixes single and multi line output
#[derive(Debug)]
struct MultiLineIterator<'a> {
    lines: Vec<&'a str>,
    index: usize,
}

// the return result will be a key will a vector of values, one per line
// some fields need to treat each line seperately
#[derive(Debug)]
struct MultiLineIteratorItem<'a> {
    key: &'a str,
    values: Vec<&'a str>,
}

impl<'a> MultiLineIterator<'a> {
    fn new(lines: Vec<&'a str>) -> Self {
        MultiLineIterator { lines, index: 0 }
    }
}

impl<'a> Iterator for MultiLineIterator<'a> {
    type Item = MultiLineIteratorItem<'a>;

    fn next(&mut self) -> Option<MultiLineIteratorItem<'a>> {
        loop {
            let index = self.index;
            self.index += 1;
            if index >= self.lines.len() {
                return None;
            }
            let b = self.lines[index].as_bytes();
            if !b.is_empty() && b[0] == b'#' {
                continue;
            }
            if let Some(colon) = self.lines[index].find(':') {
                let key = unsafe { std::str::from_utf8_unchecked(&b[..colon]) };
                let mut values = vec![];
                if b.len() > colon + 2 {
                    values.push(unsafe { std::str::from_utf8_unchecked(&b[colon + 2..]) })
                }
                loop {
                    if self.index >= self.lines.len() {
                        break;
                    }
                    let b = self.lines[self.index].as_bytes();
                    if b.len() < 2 || b[0] != b'\t' {
                        break;
                    }
                    values.push(unsafe { std::str::from_utf8_unchecked(&b[1..]) });
                    self.index += 1
                }
                return Some(MultiLineIteratorItem { key, values });
            }
        }
    }
}

pub trait PerforceTrait {
    // Add executes a p4 add, marking everything in paths for add in changelist cl.
    fn add(&self, paths: &[&str], changelist: u32) -> SgeResult<()> {
        let cl = changelist.to_string();
        let mut a = vec!["fstat", "-c", &cl];
        a.extend_from_slice(paths);
        self.exec(&a)?;
        Ok(())
    }

    // Add executes a p4 add, marking everything in paths for add in changelist cl.
    fn changes(&self, args: &[&str]) -> SgeResult<Vec<Change>> {
        let mut a = vec!["changes"];
        a.extend_from_slice(args);
        let out = self.exec(&a)?;

        lazy_static! {
            // Changes can be long or short form (hence the optional extraction at end of regex)
            // Short form is single line with truncated description
            // Long form has the description on mulitiple lines
            // Example short form:
            // Change 9395 on 2020/06/20 by boss-guy@boss-guy2-w_somecompany *pending* 'p4 lib rust 2 '
            // regex groups:
            // (changelist)(date)(user)(client)[status][description]
            static ref DESC_CHANGE_RX: Regex = Regex::new(
                r#"^Change\s+(\d+)[\D]+([\d/ :]+)\s+\S+\s+(\S+)@(\S+)\s*(?:\*(\S+)\*)?\s*(?:'(.*)')?$"#
            )
            .unwrap();
        }

        let mut changes = Vec::new();
        let mut c: Change = Default::default();
        let mut pending = false;
        for line in out.lines().filter(|&s| !s.is_empty()) {
            if let Some(groups) = regex_collector(&DESC_CHANGE_RX, line) {
                if pending {
                    changes.push(c.clone());
                }
                pending = true;
                c = Change {
                    changelist: groups[1].parse::<u32>().unwrap_or(0),
                    client: groups[4].into(),
                    date: groups[2].into(),
                    description: groups[6].into(),
                    status: groups[5].into(),
                    user: groups[3].into(),
                }
            // if not a change line, this may be an extension to the changelist description
            // in this, the line will start with a tab followed by more text of the description
            } else if line.as_bytes()[0] == b'\t' {
                if !c.description.is_empty() {
                    c.description += "\n";
                }
                c.description += &line[1..];
            }
        }
        if pending {
            changes.push(c);
        }
        Ok(changes)
    }

    // Client executes p4 client and returns details about the client
    // if client name is empty, it will return details about the default client
    fn client(&self, name: &str) -> SgeResult<Client> {
        let mut base_args = vec!["client", "-o"];
        if !name.is_empty() {
            base_args.push(name)
        }
        let out = self.exec(&base_args)?;
        let lines: Vec<&str> = out.lines().filter(|&s| !s.is_empty()).collect();
        let mut c: Client = Default::default();

        for chunk in MultiLineIterator::new(lines).filter(|s| !s.values.is_empty()) {
            match chunk.key {
                "AltRoots" => c.client = chunk.values[0].to_string(),
                "Client" => c.client = chunk.values[0].to_string(),
                "Description" => c.description = chunk.values.join("\n"),
                "Host" => c.host = chunk.values[0].to_string(),
                "LineEnd" => c.line_end = chunk.values[0].to_string(),
                "Options" => {
                    c.options = chunk.values[0]
                        .split_whitespace()
                        .map(|s| s.to_string())
                        .collect()
                }
                "Owner" => c.owner = chunk.values[0].to_string(),
                "Root" => c.root = chunk.values[0].to_string(),
                "ServerId" => c.server_id = chunk.values[0].to_string(),
                "SubmitOptions" => {
                    c.submit_options = chunk.values[0]
                        .split_whitespace()
                        .map(|s| s.to_string())
                        .collect()
                }
                "Stream" => c.stream = chunk.values[0].to_string(),
                "StreamAtChange" => c.stream_at_change = chunk.values[0].to_string(),
                "View" => c.view = chunk.values.iter().map(|s| ViewEntry::new(s)).collect(),
                _ => println!("key not matched: {}", chunk.key),
            }
        }

        Ok(c)
    }

    fn describe(&self, changelists: &[u32]) -> SgeResult<Vec<Description>> {
        let changes: Vec<String> = changelists.iter().map(|c| c.to_string()).collect();
        let c: Vec<&str> = changes.iter().map(String::as_str).collect();
        let args = [vec!["describe", "-s"], c].concat();
        let out = self.exec(&args)?;

        lazy_static! {
            // describe returns a short form description
            // (changelist)(date)(user)(client)[status][description]
            static ref DESC_CHANGE_RX: Regex = Regex::new(
                r#"^Change\s+(\d+)\s+\S+\s+([^@]+)@(\S+)\s+\S+\s+([\d/: ]+)(?:\s+\*([^\*]+)\*|$)"#
            )
            .unwrap();
            // split up details about files, stripping opening dots and extracting revision and action
            // example:
            // ... //shared/libs/go/p4lib/BUILD#4 edit
            // regex groups:
            // (filename)(revision)(status)
            static ref DESC_FILE_RX: Regex =
                Regex::new(r#"^\.\.\.\s+([^#]+)#(\d+)\s+(\S+)"#).unwrap();
        }

        let mut descs = Vec::new();
        let lines: Vec<&str> = out.lines().filter(|&s| !s.is_empty()).collect();

        let mut i = 0;
        let mut d: Description = Default::default();
        let mut pending = false;
        while i < lines.len() {
            if let Some(groups) = regex_collector(&DESC_CHANGE_RX, &lines[i]) {
                if pending {
                    descs.push(d.clone());
                    d = Default::default();
                }
                pending = true;
                d.changelist = groups[1].parse::<u32>().unwrap_or(0);
                d.user = groups[2].into();
                d.client = groups[3].into();
                d.date = groups[4].into();
                d.status = groups[5].into();
            } else if let Some(groups) = regex_collector(&DESC_FILE_RX, &lines[i]) {
                d.files.push(FileAction {
                    depot_file: groups[1].into(),
                    revision: groups[2].into(),
                    action: groups[3].into(),
                });
            } else if lines[i].as_bytes()[0] == b'\t' {
                if !d.description.is_empty() {
                    d.description += "\n";
                }
                d.description += &lines[i][1..];
            }
            i += 1;
        }
        if pending {
            descs.push(d)
        }

        Ok(descs)
    }

    fn diffs_build(&self, cmd: &str, file0: &str, file1: &str) -> SgeResult<Vec<Diff>> {
        let out = self.exec(&[cmd, file0, file1])?;

        lazy_static! {
            static ref DIFF_RX: Regex =
                // diffs are encoded in unix format, and comprise of a left range, right range and operation
                // example:
                // 346a351,354
                // regex groups
                // (left_start)[left_end](action)(right_start)[right_end]
                Regex::new(r#"^(\d+)(,(\d+))?([^,\d])(\d+)(,(\d+))?"#).unwrap();
        }

        let mut diffs = Vec::new();
        for line in out.lines().filter(|&s| !s.is_empty()) {
            if let Some(groups) = regex_collector(&DIFF_RX, line) {
                let left_line_start = groups[1].parse::<u32>().unwrap_or(0);
                let left_line_end =
                    std::cmp::max(groups[3].parse::<u32>().unwrap_or(0), left_line_start);
                let right_line_start = groups[5].parse::<u32>().unwrap_or(0);
                let right_line_end =
                    std::cmp::max(groups[7].parse::<u32>().unwrap_or(0), right_line_start);

                let diff_type = match groups[4] {
                    "a" => DiffType::Add,
                    "c" => DiffType::Change,
                    "d" => DiffType::Delete,
                    _ => DiffType::None,
                };

                diffs.push(Diff {
                    left_line_start,
                    left_line_end,
                    right_line_start,
                    right_line_end,
                    diff_type,
                });
            }
        }

        Ok(diffs)
    }

    fn diff(&self, file0: &str, file1: &str) -> SgeResult<Vec<Diff>> {
        self.diffs_build("diff", file0, file1)
    }

    fn diff2(&self, file0: &str, file1: &str) -> SgeResult<Vec<Diff>> {
        self.diffs_build("diff2", file0, file1)
    }

    fn dirs(&self, root: &str) -> SgeResult<Vec<String>> {
        let out = self.exec(&["dirs", root])?;
        Ok(out
            .lines()
            .map(|s| s.trim_start().to_owned())
            .filter(|s| !s.is_empty())
            .collect())
    }

    fn fstat(&self, args: &[&str]) -> SgeResult<FstatResult> {
        let mut a = vec!["fstat"];
        a.extend_from_slice(args);
        let out = self.exec(&a)?;

        lazy_static! {
            // fstat lines can have multiple elipses followed by key value pairs
            // example:
            //... headAction edit
            // regex groups:
            // (key) (value)
            static ref FSTAT_RX: Regex =
                Regex::new(r#"^\.\.\.\s+(?:\.\.\.\s+|)(\S+)\s*(.*)?\s*$"#).unwrap();

            // certain fstat fields contain arrays of values
            // this encoded by concatenating array index at end of variable name
            // this regex split this concatenatin into variable name,index
            // example:
            // resolveAction1
            // regex groups:
            // (variable_name)(index)
            static ref ARRAY_RX: Regex = Regex::new(r#"^(\D+)(\d+)$"#).unwrap();
        }

        let mut f: Fstat = Default::default();
        let mut result: FstatResult = Default::default();
        let mut pending = false;
        for line in out.lines().filter(|&s| !s.is_empty()) {
            if let Some(groups) = regex_collector(&FSTAT_RX, line) {
                match groups[1] {
                    "action" => f.action = groups[2].into(),
                    "actionOwner" => f.action_owner = groups[2].into(),
                    "change" => f.change = groups[2].parse::<u32>().unwrap_or(0),
                    "charset" => f.charset = groups[2].into(),
                    "clientFile" => f.client_file = groups[2].into(),
                    "depotFile" => {
                        if pending {
                            result.fstats.push(f.clone());
                            f = Default::default();
                        }
                        f.depot_file = groups[2].into();
                        pending = true;
                    }
                    "desc" => result.desc = groups[2].into(),
                    "digest" => result.desc = groups[2].into(),
                    "fileSize" => f.file_size = groups[2].parse::<u64>().unwrap_or(0),
                    "haveRev" => f.have_rev = groups[2].parse::<u32>().unwrap_or(0),
                    "headAction" => f.head_action = groups[2].into(),
                    "headChange" => f.head_change = groups[2].parse::<u32>().unwrap_or(0),
                    "headCharset" => f.head_charset = groups[2].into(),
                    "headModTime" => f.head_mod_time = groups[2].parse::<u32>().unwrap_or(0),
                    "headRev" => f.head_rev = groups[2].parse::<u32>().unwrap_or(0),
                    "headType" => f.head_type = groups[2].into(),
                    "headTime" => f.head_time = groups[2].parse::<u32>().unwrap_or(0),
                    "isMapped" => f.is_mapped = true,
                    "movedFile" => f.moved_file = groups[2].into(),
                    "movedRev" => f.moved_rev = groups[2].parse::<u32>().unwrap_or(0),
                    "otherLock" => f.other_lock = true,
                    "otherLock0" => f.other_lock0 = groups[2].into(),
                    "otherOpen" => f.other_open = groups[2].parse::<u32>().unwrap_or(0),
                    "ourLock" => f.our_lock = true,
                    "path" => f.path = groups[2].into(),
                    "resolved" => f.resolved = groups[2].parse::<u32>().unwrap_or(0),
                    "reresolvable" => f.reresolvable = groups[2].parse::<u32>().unwrap_or(0),
                    "shelved" => f.shelved = true,
                    "totalFileCount" => {
                        result.total_file_count = groups[2].parse::<u32>().unwrap_or(0)
                    }
                    "type" => f.file_type = groups[2].into(),
                    "unresolved" => f.unresolved = groups[2].parse::<u32>().unwrap_or(0),
                    "workRev" => f.work_rev = groups[2].parse::<u32>().unwrap_or(0),
                    _ => {
                        if let Some(g) = regex_collector(&ARRAY_RX, groups[1]) {
                            let index = g[2].parse::<usize>().unwrap_or(0);
                            match g[1] {
                                "otherAction" => {
                                    array_setter(&mut f.other_actions, index, groups[2].into())
                                }
                                "otherChange" => array_setter(
                                    &mut f.other_changes,
                                    index,
                                    groups[2].parse::<u32>().unwrap_or(0),
                                ),
                                "otherOpen" => {
                                    array_setter(&mut f.other_opens, index, groups[2].into())
                                }
                                "resolveAction" => {
                                    array_setter(&mut f.resolve_actions, index, groups[2].into())
                                }
                                "resolveBaseFile" => {
                                    array_setter(&mut f.resolve_base_files, index, groups[2].into())
                                }
                                "resolveBaseRev" => array_setter(
                                    &mut f.resolve_base_revs,
                                    index,
                                    groups[2].parse::<u32>().unwrap_or(0),
                                ),
                                "resolveEndFromRev" => array_setter(
                                    &mut f.resolve_start_from_revs,
                                    index,
                                    groups[2].parse::<u32>().unwrap_or(0),
                                ),
                                "resolveFromFile" => {
                                    array_setter(&mut f.resolve_from_files, index, groups[2].into())
                                }
                                "resolveStartFromRev" => array_setter(
                                    &mut f.resolve_start_from_revs,
                                    index,
                                    groups[2].parse::<u32>().unwrap_or(0),
                                ),
                                _ => {}
                            }
                        } else {
                            println!("unknown fstat key {}", groups[1]);
                        }
                    }
                }
            } else {
                println!("couldn't match {}", line);
            }
        }
        if pending {
            result.fstats.push(f)
        }

        Ok(result)
    }

    fn info(&self) -> SgeResult<Info> {
        let out = self.exec(&["info"])?;
        let mut info: Info = Default::default();
        for kv in out
            .lines()
            .map(|s| s.split(": ").collect::<Vec<&str>>())
            .filter(|v| v.len() > 1)
        {
            let value = kv[1].into();
            match kv[0] {
                "Case Handling" => info.case_handling = value,
                "Changelist server" => info.changelist_server = value,
                "Client address" => info.client_address = value,
                "Client name" => info.client_name = value,
                "Client host" => info.client_host = value,
                "Client root" => info.client_root = value,
                "Current directory" => info.current_directory = value,
                "Peer address" => info.peer_address = value,
                "Replica of" => info.replica_of = value,
                "Server address" => info.server_address = value,
                "Server cert expires" => info.server_cert_expires = value,
                "Server date" => info.server_date = value,
                "Server encryption" => info.server_encryption = value,
                "ServerID" => info.server_id = value,
                "Server license" => info.server_license = value,
                "Server root" => info.server_root = value,
                "Server services" => info.server_services = value,
                "Server uptime" => info.server_uptime = value,
                "Server version" => info.server_version = value,
                "User name" => info.user_name = value,
                _ => println!("unknown key {}", kv[0]),
            }
        }

        Ok(info)
    }

    fn opened(&self) -> SgeResult<Vec<FileOpened>> {
        lazy_static! {
            // opened contains details about all opened files
            // we have to differentiate between those in numbered CLs and those in default CL
            // examples:
            // //shared/libs/go/p4lib/p4-lib.go#11 - edit change 9381 (text)
            // //shared/WORKSPACE#45 - edit default change (text)
            // regex groups:
            // (depot_file)(revision)(action)(changelist)
            static ref OPENED_RX: Regex = Regex::new(
                r#"^([^#]+)#(\d+)\s+-\s+(\S+)\s+(?:(default) change|change (\d+))\s+\(([^\)]+)\)"#
            )
            .unwrap();
        }

        let out = self.exec(&["opened"])?;
        let mut opens = Vec::new();

        for groups in out
            .lines()
            .map(|s| regex_collector(&OPENED_RX, s))
            .filter_map(|g| g)
        {
            opens.push(FileOpened {
                action: groups[3].into(),
                changelist: groups[5].parse::<u32>().unwrap_or(0),
                depot_file: groups[1].into(),
                file_type: groups[6].into(),
                revision: groups[2].parse::<u32>().unwrap_or(0),
            });
        }

        Ok(opens)
    }

    fn sizes(&self, args: &[&str]) -> SgeResult<SizeCollection> {
        let mut a = vec!["fstat"];
        a.extend_from_slice(args);
        let out = self.exec(&a)?;

        let mut sizes: SizeCollection = Default::default();

        lazy_static! {
            // sizes file has information about each individual file
            // example:
            // //shared/tools/... 136 files 1840410 bytes
            // regex groups:
            // (depot_director)(file_count)(file_size)
            static ref TOTAL_RX: Regex =
                Regex::new(r#"^(.*)\s+(\d+)\s+\S+\s+(\d+)\s+\S+"#).unwrap();

            // sizes file has information about each individual file
            // example:
            // //shared/tools/gigantick/gigantick.go#2 7880 bytes
            // regex groups:
            // (depot_path)(revision)(file_size)
            static ref FILE_RX: Regex = Regex::new(r#"^(.*)#(\d+)\s+(\d+)\s\S+"#).unwrap();
        }

        for line in out.lines().filter(|s| !s.is_empty()) {
            if let Some(g) = regex_collector(&FILE_RX, line) {
                sizes.sizes.push(Size {
                    depot_path: g[1].into(),
                    revision: g[2].parse::<u32>().unwrap_or(0),
                    file_size: g[3].parse::<u64>().unwrap_or(0),
                });
            } else if let Some(g) = regex_collector(&TOTAL_RX, line) {
                sizes.depot_directory = g[1].into();
                sizes.total_file_count = g[2].parse::<u64>().unwrap_or(0);
                sizes.total_file_size = g[3].parse::<u64>().unwrap_or(0);
            }
        }

        Ok(sizes)
    }

    fn tickets(&self) -> SgeResult<Vec<Ticket>> {
        let out = self.exec(&["tickets"])?;
        let mut ticks = Vec::new();

        lazy_static! {
            // tickets returns a triple of values
            // example:
            // localhost:FAKE_AUTH_ID (notrealuser) 64578c65C39CB79DB7DD1B86016f25A7
            // regex groups:
            // (name)(user)(id)
            static ref TICKETS_RX: Regex = Regex::new(r#"^(\S+)\s\((\S+)\)\s(\S+)"#).unwrap();
        }
        for tokens in out.lines().filter_map(|s| regex_collector(&TICKETS_RX, s)) {
            ticks.push(Ticket {
                name: tokens[1].into(),
                user: tokens[2].into(),
                id: tokens[3].into(),
            });
        }
        Ok(ticks)
    }

    // interface for exec command
    fn exec(&self, args: &[&str]) -> SgeResult<String>;
}

// simple function to ensure that the array has enough capcity to set value at specified index
fn array_setter<T>(array: &mut Vec<T>, index: usize, value: T)
where
    T: Default,
{
    while array.len() <= index {
        array.push(T::default());
    }
    array[index] = value;
}

// runs a regex match and collects a vector of result options
// saves a lot of client unwrapping from stand regex calls
fn regex_collector<'a>(re: &Regex, input: &'a str) -> Option<Vec<&'a str>> {
    if let Some(groups) = re.captures(input) {
        Some(
            groups
                .iter()
                .map(|m| match m {
                    Some(m) => m.as_str(),
                    None => "",
                })
                .collect(),
        )
    } else {
        None
    }
}

// Main trait for (non-mocked) perforce interface
impl PerforceTrait for Perforce {
    // exec will execute passed in command use command line p4
    fn exec(&self, args: &[&str]) -> SgeResult<String> {
        let mut all_args = vec!["-c", "utf8"];
        all_args.extend_from_slice(args);
        let out = Command::new("p4").args(all_args).output()?;
        let cmd_stdout = String::from_utf8_lossy(&out.stdout);
        let cmd_stderr = String::from_utf8_lossy(&out.stderr);
        Ok((cmd_stdout + cmd_stderr).into())
    }
}

// Simple helper to construct a perforce object
impl Perforce {
    fn new() -> Self {
        Perforce {}
    }
}
