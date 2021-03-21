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
use hassle_rs::utils::compile_hlsl;
use regex::Regex;
use std::fs::File;
use std::io::prelude::*;
use std::io::BufWriter;
use std::path::Path;

// use the target name of rust_library from bazel
// the rust_library will create a Crate with target name
use rust_shader_headers::render::shader::{
    get_root_as_shader_package, ShaderHeader, ShaderHeaderArgs, ShaderPackage, ShaderPackageArgs,
    ShaderType, ENUM_NAMES_SHADER_TYPE, ENUM_VALUES_SHADER_TYPE,
};

fn get_shader_target(st: ShaderType) -> &'static str {
    match st {
        ShaderType::Compute => "cs_6_0",
        ShaderType::Domain => "ds_6_0",
        ShaderType::Geometry => "gs_6_0",
        ShaderType::Hull => "hs_6_0",
        ShaderType::Pixel => "ps_6_0",
        ShaderType::Vertex => "vs_6_0",
    }
}

fn read_file(file_name: &str) -> std::io::Result<Vec<u8>> {
    let mut file = File::open(file_name)?;
    let info = file.metadata()?;
    let mut data = vec![0; info.len() as usize];
    file.read_exact(&mut data)?;
    Ok(data)
}

fn save_file(file_name: &str, data: &[u8]) -> std::io::Result<()> {
    let file = File::create(file_name)?;
    let mut buf_writer = BufWriter::new(file);
    buf_writer.write_all(data)?;
    Ok(())
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

// cannot use impl FromStr for ShaderType since ShaderType is from external Crate
fn shader_type_from_str(input: &str) -> SgeResult<ShaderType> {
    for (n, v) in ENUM_NAMES_SHADER_TYPE
        .iter()
        .zip(ENUM_VALUES_SHADER_TYPE.iter())
    {
        if *n == input {
            return Ok(*v);
        }
    }
    Err(SgeError::Literal("name not found"))
}

pub fn shader_compile<'a>(
    data: &[u8],
    name: &str,
) -> SgeResult<flatbuffers::FlatBufferBuilder<'a>> {
    let contents = std::str::from_utf8(&data).unwrap();

    let re = Regex::new(r#"\s*@shader\s*\(\s*(\S+)\s*,\s*(\S+)\s*\)"#).unwrap();

    let mut shader_text = String::with_capacity(contents.len());
    let mut variants = Vec::new();

    let mut builder = flatbuffers::FlatBufferBuilder::new();

    for line in contents.lines() {
        if let Some(groups) = regex_collector(&re, line) {
            let st = shader_type_from_str(groups[2])?;
            variants.push((st, groups[1].to_string()));
        } else {
            shader_text.push_str(line);
            shader_text.push('\n');
        }
    }

    let mut shaders = Vec::new();
    for s in variants.iter_mut() {
        let target_profile = get_shader_target(s.0);
        let entry_point = &s.1;
        let args = &["-spirv", "-fspv-reflect"];
        let defines = &[];
        let compiled = compile_hlsl(
            name,
            &shader_text,
            &entry_point,
            target_profile,
            args,
            defines,
        );

        let ep = builder.create_string(&entry_point);
        let sd = compiled.unwrap();
        println!("shader size: {}", sd.len());
        let shader_data = builder.create_vector(&sd);
        shaders.push(ShaderHeader::create(
            &mut builder,
            &ShaderHeaderArgs {
                entry_point: Some(ep),
                shader_type: s.0,
                data: Some(shader_data),
                ..Default::default()
            },
        ));
    }
    let sv = builder.create_vector(&shaders);
    let name_vec: Vec<&str> = name.split(".").collect();
    let package_name = builder.create_string(&name_vec[0]);
    let package = ShaderPackage::create(
        &mut builder,
        &ShaderPackageArgs {
            name: Some(package_name),
            shaders: Some(sv),
            ..Default::default()
        },
    );
    builder.finish(package, None);

    let pkg = get_root_as_shader_package(builder.finished_data());
    let sads = pkg.shaders().unwrap();
    for s in sads.iter() {
        println!("entry point: {}", s.entry_point().unwrap());
    }

    Ok(builder)
}

pub fn compile_and_save(intput: &str, output: &str) -> SgeResult<()> {
    let data = read_file(intput)?;
    let name = Path::new(intput).file_name().unwrap();
    let shaders = shader_compile(&data, name.to_str().unwrap())?;
    save_file(output, shaders.finished_data())?;
    Ok(())
}
