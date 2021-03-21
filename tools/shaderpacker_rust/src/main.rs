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

use shaderpacker_rust_lib::*;

fn main() {
    if std::env::args().len() != 4 {
        // using "-T" to be consistent with dxc cmd
        println!("usage: shaderpacker_rust -T <output_compiled_shader_file> <input_hlsl_file>");
        std::process::exit(1);
    }

    let input = std::env::args().nth(3).unwrap();
    let output = std::env::args().nth(2).unwrap();
    if let Err(e) = compile_and_save(&input, &output) {
        println!("error: {}", e);
    }
}
