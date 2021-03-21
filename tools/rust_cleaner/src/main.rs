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

// binary rust_cleaner cleans up intermediate rust artefacts from all rust directories
// RLS creates a directory titled "target" that accumulates gigabytes of intermediate data across
// our repo

use error_lib::SgeResult;

use std::env;
use std::fs;
use std::path::PathBuf;
use std::process::Command;

fn get_monorepo_base_path() -> SgeResult<PathBuf> {
    let mut dir = env::current_dir()?;
    loop {
        let mr = dir.join("MONOREPO");
        if mr.exists() {
            return Ok(dir);
        }
        if !dir.pop() {
            return Err("monorepo not found.\nPlease run in sub directory of monorepo".into());
        }
    }
}

fn cargo_clean(path: &PathBuf) -> SgeResult<()> {
    println!("cargo clean: {:#?}", path);
    let status = Command::new("cargo")
        .args(&["clean"])
        .current_dir(path)
        .status()?;
    if !status.success() {
        println!("  FAILED");
    }
    Ok(())
}

fn toml_process(base_dir: PathBuf) -> SgeResult<()> {
    let toml = base_dir.join("Cargo.toml");
    if toml.exists() {
        cargo_clean(&base_dir)?;
    }
    let entries = fs::read_dir(base_dir)?;
    for entry in entries {
        let entry = entry?;
        if entry.path().is_dir() {
            if let Err(e) = toml_process(entry.path()) {
                println!("directory process error: {:#?}", e)
            }
        }
    }
    Ok(())
}

fn paths_process() -> SgeResult<()> {
    // we only want to crawl a subset of the monorepo
    let rust_paths = &["build", "libs", "third_party/rust", "tools"];
    let base = get_monorepo_base_path()?;
    for r in rust_paths {
        let sub_dir = base.join(r);
        if let Err(e) = toml_process(sub_dir) {
            println!("error processing sub directory: {}", e)
        }
    }
    Ok(())
}

fn main() {
    if let Err(e) = paths_process() {
        println!("error: {}", e);
    }
}
