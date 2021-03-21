# Shader Packer Rust

The shader packer rust project uses [hassle_rs](https://docs.rs/hassle-rs/0.4.0/hassle_rs/) which
internally links with [dxc shader compiler](https://github.com/microsoft/DirectXShaderCompiler) to
compile .hlsl files into .shader_pkg files which contains spirv binary and some meta data.

## Syntax

.hlsl file could have multiple shaders, all the entry points of the shaders need to be declared with
following syntax:
@shader([EntryPoint], [ShaderType])

Valide ShaderType options:

* Vertex,
* Pixel,
* Geometry,
* Hull,
* Domain,
* Compute,

## Header Generation

[FlatBuffers](https://google.github.io/flatbuffers/) is used to serialize the output data.
Headers (.fbs) are used to generate the headers (.rs and .h) for the packer and runtime project.

## Build Command

Bazel command:

`bazel build shaderpacker_rust`

## Run Command

`shaderpacker_rust -T <output_shader_pkg_name> <input_hlsl_file>`

Example: `"shaderpacker_rust -T lighting.shader_pkg lighting.hlsl"`

