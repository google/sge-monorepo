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

//-----------------------------------------------------------------------------
//	Incredible - Include Scanner | Leon O'Reilly
//
//	Retrieves all #includes from input file
//	Resolved paths using supplied inclued directories
//	Recurses through all includes and builds full list of dependents
//	Resolves basic macros
//	Able to run single or multi-threaded
//
//	Syntax
//	incredible -f=input_file -o=output_file <-i=include_dir0 ... -i=include_dirN>  <-dDEFINE_KEY_1=DEFINE_VALUE_1 .... -dDEFINE_KEY_N-=DEFINE_VALUE_N>

//-----------------------------------------------------------------------------

//-----------------------------------------------------------------------------
//	Using
//-----------------------------------------------------------------------------

use std::collections::{HashMap, HashSet, VecDeque};
use std::fs;
use std::fs::File;
use std::io::Write;
use std::path::{Path, PathBuf, MAIN_SEPARATOR};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;

//	Generic Eror Type

#[derive(Debug)]
pub enum IncError {
    IO(std::io::Error),
    StdErr(Box<dyn std::error::Error>),
    Literal(&'static str),
}

pub type IncResult<T> = Result<T, IncError>;

impl From<std::io::Error> for IncError {
    fn from(e: std::io::Error) -> Self {
        IncError::IO(e)
    }
}

impl From<Box<dyn std::error::Error>> for IncError {
    fn from(e: Box<dyn std::error::Error>) -> Self {
        IncError::StdErr(e)
    }
}

impl From<&'static str> for IncError {
    fn from(e: &'static str) -> Self {
        IncError::Literal(e)
    }
}

//	Enum for include types (quote vs angle brackets)

#[derive(PartialEq)]
enum IncludeSearch {
    Local,
    System,
}

struct ResolvedPathCollection {
    resolved: Arc<Mutex<HashMap<PathBuf, HashSet<String>>>>,
}

impl ResolvedPathCollection {
    pub fn new() -> Self {
        ResolvedPathCollection {
            resolved: Arc::new(Mutex::new(HashMap::<PathBuf, HashSet<String>>::new())),
        }
    }
}

struct ResolvedPaths {
    local: ResolvedPathCollection,
    system: ResolvedPathCollection,
}

// counter for amount of active job threads

static GLOBAL_JOB_COUNT: AtomicUsize = AtomicUsize::new(0);

//-----------------------------------------------------------------------------
// helper to ensure path is formatted correctly for platform
//-----------------------------------------------------------------------------

fn path_sanitise(src: &str) -> PathBuf {
    let alt_seperator = match MAIN_SEPARATOR {
        '/' => '\\',
        _ => '/',
    };

    let cleaned = &src.replace(alt_seperator, &MAIN_SEPARATOR.to_string());
    Path::new(cleaned).to_path_buf()
}

//-----------------------------------------------------------------------------
// Create absolute path from relative
//-----------------------------------------------------------------------------

fn path_absolute(src: &Path) -> PathBuf {
    let mut pb = PathBuf::new();
    let mut v = Vec::new();

    for c in src.components() {
        if ".." == c.as_os_str() {
            v.pop();
        } else {
            v.push(c);
        }
    }

    for cv in v {
        pb.push(cv);
    }

    pb
}

//-----------------------------------------------------------------------------
//	Resolve path of include file
//-----------------------------------------------------------------------------

fn include_resolve_path(
    base_dir: &Path,
    filename: &str,
    search_type: IncludeSearch,
    includes: &[PathBuf],
) -> Option<PathBuf> {
    let pfname = path_sanitise(filename);

    // if include is quoted, start by searching relative
    if IncludeSearch::Local == search_type {
        let abs_path = base_dir.join(&pfname);
        let abs_path = path_absolute(&abs_path);
        if let Ok(md) = fs::metadata(&abs_path) {
            if md.is_file() {
                return Some(abs_path);
            }
        }
    }

    // search by prepending include paths
    for inc_path in includes.iter() {
        let abs_path = Path::new(&inc_path).join(filename);
        let abs_path = path_absolute(&abs_path);
        if let Ok(md) = fs::metadata(&abs_path) {
            if md.is_file() {
                return Some(abs_path);
            }
        }
    }

    let abs_path = base_dir.join(&pfname);
    let abs_path = path_absolute(&abs_path);
    if let Ok(md) = fs::metadata(&abs_path) {
        if md.is_file() {
            return Some(abs_path);
        }
    }

    println!("warning: file not found {}", filename);
    None
}

//-----------------------------------------------------------------------------
//	add file to list to be processed if not processed already
//-----------------------------------------------------------------------------

fn file_add(
    base_dir: &Path,
    filename: &str,
    search_type: IncludeSearch,
    includes: &[PathBuf],
    processsed: &Arc<Mutex<HashSet<String>>>,
    queued: &Arc<Mutex<VecDeque<PathBuf>>>,
    rp: &mut ResolvedPathCollection,
) {
    // we want to minimise the amount of times we need to hit file system. lets see if base path+include filename has already been resolved
    {
        let rp_guard = rp.resolved.lock();
        if let Ok(rp_c) = rp_guard {
            if let Some(rp_k) = rp_c.get(base_dir.into()) {
                if rp_k.contains(filename) {
                    return;
                }
            }
        }
    }

    if let Some(inc_result) = include_resolve_path(base_dir, filename, search_type, includes) {
        let abs_path = inc_result.to_str().unwrap_or_default();

        // if we haven't already processed this path, add it to queue to process
        let proc_guard = processsed.lock();
        if let Ok(mut p) = proc_guard {
            if !p.contains(abs_path) {
                p.insert(abs_path.into());
                let q_guard = queued.lock();
                if let Ok(mut q) = q_guard {
                    q.push_back(inc_result);
                }

                {
                    let rp_guard = rp.resolved.lock();
                    if let Ok(mut rp_c) = rp_guard {
                        rp_c.entry(base_dir.to_path_buf())
                            .or_insert_with(HashSet::new)
                            .insert(filename.to_string());
                    }
                }
            }
        }
    }
}

//-----------------------------------------------------------------------------
// process file and find includes
//-----------------------------------------------------------------------------

fn file_process(
    full_path: &Path,
    includes: &[PathBuf],
    processsed: Arc<Mutex<HashSet<String>>>,
    queued: Arc<Mutex<VecDeque<PathBuf>>>,
    defines: &mut HashMap<String, String>,
    rp: &mut ResolvedPaths,
) -> IncResult<()> {
    let base_dir = full_path.parent().unwrap_or(Path::new(""));

    let filename_string = full_path.to_str().ok_or("")?;
    let data = fs::read(filename_string)?;

    enum SearchMode {
        Hash,
        Directive,
        WhiteSpace,
        DefineKey,
        DefineValue,
        Quote,
        Arrow,
        Macro,
    }

    enum DirectiveType {
        DefineKey,
        DefineValue,
        Include,
    }

    let mut directive_type = DirectiveType::Include;
    let mut search_mode = SearchMode::Hash;
    let mut start_index = 0;
    let mut define_key = "";
    //	let mut line_index = 1;

    for (cursor, cc) in data.iter().enumerate() {
        let character = *cc as char;
        /*
                if 10 == *cc {
                    line_index += 1;
                }
        */
        match search_mode {
            SearchMode::Hash => {
                if '#' == character {
                    search_mode = SearchMode::Directive;
                    start_index = cursor;
                }
            }
            SearchMode::Directive => match character {
                ' ' | '\t' => {
                    let directive = std::str::from_utf8(&data[start_index..cursor]).unwrap();
                    match directive {
                        "#include" => {
                            directive_type = DirectiveType::Include;
                            search_mode = SearchMode::WhiteSpace;
                        }
                        "#define" => {
                            directive_type = DirectiveType::DefineKey;
                            search_mode = SearchMode::WhiteSpace;
                        }
                        _ => {
                            search_mode = SearchMode::Hash;
                        }
                    }
                }
                _ => {}
            },
            SearchMode::WhiteSpace => {
                start_index = cursor;

                match character {
                    ' ' | '\t' => {}
                    '\r' | '\n' => search_mode = SearchMode::Hash,
                    _ => match directive_type {
                        DirectiveType::Include => {
                            search_mode = match character {
                                ' ' | '\t' => SearchMode::WhiteSpace,
                                '"' => SearchMode::Quote,
                                '<' => SearchMode::Arrow,
                                _ => SearchMode::Macro,
                            };
                        }
                        DirectiveType::DefineKey => {
                            search_mode = SearchMode::DefineKey;
                        }
                        DirectiveType::DefineValue => {
                            search_mode = SearchMode::DefineValue;
                        }
                    },
                }
            }
            SearchMode::DefineKey => match character {
                ' ' | '\t' | '\r' | '\n' => {
                    define_key = std::str::from_utf8(&data[start_index..cursor]).unwrap();
                    directive_type = DirectiveType::DefineValue;
                    search_mode = SearchMode::WhiteSpace;
                }
                _ => {}
            },
            SearchMode::DefineValue => match character {
                ' ' | '\t' | '\r' | '\n' => {
                    let define_value = std::str::from_utf8(&data[start_index..cursor]).unwrap();
                    defines.insert(define_key.to_string(), define_value.to_string());
                    search_mode = SearchMode::Hash;
                }
                _ => {}
            },
            SearchMode::Quote => {
                if '"' == character {
                    file_add(
                        base_dir,
                        std::str::from_utf8(&data[start_index + 1..cursor]).unwrap(),
                        IncludeSearch::Local,
                        includes,
                        &processsed,
                        &queued,
                        &mut rp.local,
                    );
                    search_mode = SearchMode::Hash
                }
            }
            SearchMode::Arrow => {
                if '>' == character {
                    file_add(
                        base_dir,
                        std::str::from_utf8(&data[start_index + 1..cursor]).unwrap(),
                        IncludeSearch::System,
                        includes,
                        &processsed,
                        &queued,
                        &mut rp.system,
                    );
                    search_mode = SearchMode::Hash
                }
            }
            SearchMode::Macro => match character {
                ' ' | '\t' | '\r' | '\n' => {
                    let macro_key = std::str::from_utf8(&data[start_index..cursor]).unwrap();
                    if let Some(mv) = defines.get(macro_key) {
                        if mv.len() > 1 {
                            let stripped = &mv[1..mv.len() - 1];
                            match mv.chars().next().unwrap() {
                                '"' => {
                                    file_add(
                                        base_dir,
                                        &stripped,
                                        IncludeSearch::Local,
                                        includes,
                                        &processsed,
                                        &queued,
                                        &mut rp.local,
                                    );
                                }
                                '<' => {
                                    file_add(
                                        base_dir,
                                        &stripped,
                                        IncludeSearch::System,
                                        includes,
                                        &processsed,
                                        &queued,
                                        &mut rp.system,
                                    );
                                }
                                _ => {
                                    println!("malformed filename : {}", mv);
                                }
                            }
                        }
                    } else {
                        println!("couldn't find macro: {}", macro_key);
                    }
                    search_mode = SearchMode::Hash
                }
                _ => {}
            },
        }
    }

    Ok(())
}

//-----------------------------------------------------------------------------
// parse all command line options into map (options should be in the form of ikey=value)
//-----------------------------------------------------------------------------

pub fn command_line_parse() -> HashMap<String, Vec<Option<String>>> {
    let mut hm = HashMap::<String, Vec<Option<String>>>::new();

    // first argument is executable name, so we skip this
    for arg in std::env::args().skip(1) {
        let sp: Vec<&str> = arg.split('=').collect();
        if !sp.is_empty() {
            // trim whitespace and leading hyphen
            let mut k = sp[0].trim();
            if k.starts_with('-') {
                k = &k[1..];
            }

            let value = if sp.len() > 1 {
                Some(sp[1].to_string())
            } else {
                None
            };

            hm.entry(k.into()).or_insert_with(Vec::new).push(value);
        }
    }
    hm
}

//-----------------------------------------------------------------------------
//	main - entry point
//-----------------------------------------------------------------------------

fn main() {
    println!("Incredible: Include Scanner");

    // parse command line
    let command_line = command_line_parse();

    // parse all includes and collect into vector
    let mut includes = Vec::<PathBuf>::new();
    if let Some(incs) = command_line.get("i") {
        for val in incs.iter() {
            if let Some(v) = val {
                includes.push(path_sanitise(v));
            }
        }
    }
    dbg!(&includes);
    let arc_includes = Arc::new(includes);

    // a deque for work jobs, to be consumed by job system
    let work = Arc::new(Mutex::new(VecDeque::<PathBuf>::new()));

    // markers to ensure each file is only processed once
    let processed = Arc::new(Mutex::new(HashSet::new()));

    // queue all input files for processing
    if let Some(input_files) = command_line.get("f") {
        for i in input_files {
            if let Some(i_file) = i {
                let maybe_work = work.lock();
                if let Ok(mut q) = maybe_work {
                    q.push_back(Path::new(i_file).to_path_buf());
                }
            }
        }
    }

    // parse defines
    let mut defines = HashMap::<String, String>::new();
    for (cl_key, cl_values) in command_line.iter() {
        if cl_key.starts_with("d") {
            if let Some(cl_last) = cl_values.last() {
                if let Some(cl_last_value) = cl_last {
                    let def_key = &cl_key[1..];
                    defines.insert(def_key.to_string(), cl_last_value.to_string());
                }
            }
        }
    }
    dbg!(&defines);

    // vector to contain all thread handles
    let mut threads = Vec::new();

    // optional single threaded mode, useful for debugging
    let single_threaded = command_line.contains_key("st");

    loop {
        let f_path = work.lock().unwrap().pop_front();
        match f_path {
            Some(f) => {
                GLOBAL_JOB_COUNT.fetch_add(1, Ordering::SeqCst);

                let processed = processed.clone();
                let work = work.clone();
                let includes = arc_includes.clone();
                let mut rp = ResolvedPaths {
                    local: ResolvedPathCollection::new(),
                    system: ResolvedPathCollection::new(),
                };

                let mut defines2 = defines.clone();
                if single_threaded {
                    let _ = file_process(&f, &includes, processed, work, &mut defines2, &mut rp);
                    GLOBAL_JOB_COUNT.fetch_sub(1, Ordering::SeqCst);
                } else {
                    let handle = thread::spawn(move || {
                        let _ =
                            file_process(&f, &includes, processed, work, &mut defines2, &mut rp);
                        GLOBAL_JOB_COUNT.fetch_sub(1, Ordering::SeqCst);
                    });
                    threads.push(handle);
                }
            }
            None => {
                let jobs = GLOBAL_JOB_COUNT.load(Ordering::SeqCst);
                if 0 == jobs {
                    break;
                } else {
                    //std::thread::yield_now();
                }
            }
        }
    }

    // wait for all threads to complete
    for handle in threads {
        handle.join().unwrap();
    }

    // create sorted list of includes
    let mut sorted = Vec::new();
    let pro = processed.lock();
    if let Ok(p) = pro {
        for pi in p.iter() {
            sorted.push(pi.clone());
        }
    }
    sorted.sort();

    //	dbg!(&sorted);

    // write dependencies to specified output files (-o="output_file.txt")
    if let Some(output_files) = command_line.get("o") {
        for of in output_files {
            if let Some(o_file) = of {
                if let Ok(mut f) = File::create(o_file) {
                    for inc in sorted.iter() {
                        if !writeln!(f, "{}", inc).is_ok() {
                            println!("coudln't write to output file: {}", o_file);
                        }
                    }
                } else {
                    println!("coudln't create output file: {}", o_file);
                }
            }
        }
    }
}

//-----------------------------------------------------------------------------
//	TESTS
//-----------------------------------------------------------------------------

#[cfg(test)]
mod test_incredible {

    #[cfg(test)]
    use super::*;

    #[test]
    fn test_absolute_path() {
        let a = path_absolute(Path::new("C:\test"));
        assert_eq!(Path::new("C:\test"), a);

        let a = path_absolute(Path::new("test"));
        assert_eq!(Path::new("test"), a);

        let a = path_absolute(Path::new("boo/bar"));
        assert_eq!(Path::new("boo/bar"), a);

        let a = path_absolute(Path::new(r#"go\fish"#));
        assert_eq!(Path::new(r#"go\fish"#), a);

        let a = path_absolute(Path::new(r#"first\second\.."#));
        assert_eq!(Path::new(r#"first"#), a);

        let a = path_absolute(Path::new(r#"first\second\..\third"#));
        assert_eq!(Path::new(r#"first\third"#), a);
    }
}
