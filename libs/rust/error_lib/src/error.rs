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

// Library error_lib creates a generic error type for rust
// This allows simplified chaining of error callbacks using the ? operator

#[derive(Debug)]
pub enum SgeError {
    IO(std::io::Error),
    StdErr(Box<dyn std::error::Error>),
    Literal(&'static str),
    Message(String),
}

pub type SgeResult<T> = Result<T, SgeError>;

impl From<std::io::Error> for SgeError {
    fn from(e: std::io::Error) -> Self {
        SgeError::IO(e)
    }
}

impl From<Box<dyn std::error::Error>> for SgeError {
    fn from(e: Box<dyn std::error::Error>) -> Self {
        SgeError::StdErr(e)
    }
}

impl From<&'static str> for SgeError {
    fn from(e: &'static str) -> Self {
        SgeError::Literal(e)
    }
}

impl From<String> for SgeError {
    fn from(e: String) -> Self {
        SgeError::Message(e)
    }
}

impl From<std::fmt::Error> for SgeError {
    fn from(e: std::fmt::Error) -> Self {
        SgeError::Message(format!("{:?}", e))
    }
}

impl From<std::ffi::NulError> for SgeError {
    fn from(_: std::ffi::NulError) -> Self {
        SgeError::Literal("Null error")
    }
}

impl From<()> for SgeError {
    fn from(_: ()) -> Self {
        SgeError::Literal("")
    }
}

impl Into<&'static str> for SgeError {
    fn into(self) -> &'static str {
        match self {
            SgeError::IO(_) => "io error",
            SgeError::StdErr(_) => "std err",
            SgeError::Literal(_) => "literal",
            SgeError::Message(_) => "message",
        }
    }
}

impl std::error::Error for SgeError {
    fn source(&self) -> Option<&(dyn std::error::Error + 'static)> {
        match *self {
            SgeError::IO(ref e) => Some(e),
            SgeError::StdErr(_) => None,
            SgeError::Literal(_) => None,
            SgeError::Message(_) => None,
        }
    }
}

impl std::fmt::Display for SgeError {
    fn fmt(&self, f: &mut std::fmt::Formatter) -> std::fmt::Result {
        match *self {
            SgeError::IO(ref e) => e.fmt(f),
            SgeError::StdErr(_) => write!(f, "std err"),
            SgeError::Literal(ref lit) => write!(f, "{}", lit),
            SgeError::Message(ref msg) => write!(f, "{}", msg),
        }
    }
}

impl Clone for SgeError {
    fn clone(&self) -> Self {
        match self {
            SgeError::IO(m) => SgeError::Message(format!("{}", m)),
            SgeError::StdErr(m) => SgeError::Message(format!("{}", m)),
            SgeError::Literal(m) => SgeError::Literal(*m),
            SgeError::Message(m) => SgeError::Message(m.into()),
        }
    }
}

impl PartialEq for SgeError {
    fn eq(&self, other: &Self) -> bool {
        match (self, other) {
            (SgeError::IO(_), SgeError::IO(_)) => true,
            (SgeError::StdErr(_), SgeError::StdErr(_)) => true,
            (SgeError::Literal(a), SgeError::Literal(b)) => a == b,
            (SgeError::Message(a), SgeError::Message(b)) => a == b,
            (_, _) => false,
        }
    }
}

pub fn err_logged<T>(msg: &'static str) -> Result<T, &'static str> {
    println!("{}", msg);
    Err(msg)
}
