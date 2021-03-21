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

extern crate getopts;
use chrono::prelude::*;
use chrono::Duration;
use getopts::Options;
use std::env;
use std::process::Command;

#[cfg(target_os = "windows")]
use clipboard_win::Clipboard;

fn print_help(program: &str, opts: Options) {
    let brief = format!("Usage: {} [options]", program);
    print!("{}", opts.usage(&brief));
}

fn build_p4_date(dt: DateTime<Local>) -> String {
    format!("@{}/{}/{}", dt.year(), dt.month(), dt.day())
}

fn main() {
    let args: Vec<String> = env::args().collect();
    let ref program = args[0];

    let mut opts = Options::new();
    opts.optopt(
        "r",
        "range",
        "specify an optional date or CL range (else past week)",
        "@2020/01/01,@now or @1,@37000",
    );
    opts.optflag("h", "help", "print this help menu");
    let matches = match opts.parse(&args[1..]) {
        Ok(m) => m,
        Err(f) => panic!(f.to_string()),
    };
    if matches.opt_present("h") {
        print_help(&program, opts);
        return;
    }

    let username = env::var("USERNAME").unwrap_or_default();

    let now = Local::now();
    let mut weekday_current = now.weekday().num_days_from_monday();
    if 0 == weekday_current {
        weekday_current = 7;
    }
    let monday = now - Duration::days(weekday_current.into());
    let sunday = monday + Duration::days(7);

    let range = match matches.opt_str("r") {
        Some(s) => s,
        None => format!("{},{}", build_p4_date(monday), build_p4_date(sunday)),
    };

    let output = Command::new("p4")
        .args(&[
            "-C",
            "utf8-bom",
            "changes",
            "-s",
            "submitted",
            "-u",
            &username,
            "-l",
            &range,
        ])
        .output()
        .expect("failed to execute process");

    let cmd_stdout = String::from_utf8_lossy(&output.stdout);
    let cmd_stderr = String::from_utf8_lossy(&output.stderr);

    let lines = cmd_stdout.split("\n");

    let mut details = Vec::new();

    details.push(format!("Perforce Changes:\n"));
    for line in lines {
        if line.starts_with("Change") {
            let words = line.split(" ").collect::<Vec<&str>>();
            if words.len() > 1 {
                details.push(format!(
                    "\n* [change {}]",
                    words[1], words[1]
                ));
            }
        } else {
            details.push(format!(" {}", line.trim()));
        }
    }

    for d in &details {
        print!("{}", d);
    }

    copy_to_clipboard(&details.into_iter().collect::<String>())
        .expect("couldn't copy to clipboard");

    println!("{}", cmd_stderr);
}

#[cfg(target_os = "windows")]
fn copy_to_clipboard(blob: &str) -> std::io::Result<()> {
    let c = Clipboard::new()?;
    c.set_string(blob)?;
    Ok(())
}

#[cfg(not(target_os = "windows"))]
fn copy_to_clipboard(blob: &str) -> std::io::Result<()> {
    Ok(())
}
