# TODO: Activate when rust is activated back.
#load("@io_bazel_rules_rust//rust:rust.bzl", "rust_binary", "rust_library", "rust_test")
#load(":shader_header_lib.bzl", "shader_header_library")
#load("@bazel_skylib//rules:copy_file.bzl", "copy_file")
#
#copy_file(
#    name = "dxcompiler",
#    src = "@vulkan//:dxc_shared_library",
#    out = "dxcompiler.dll",
#)
#
#copy_file(
#    name = "spriv",
#    src = "@vulkan//:spriv_shared_library",
#    out = "SPIRV-Tools-shared.dll",
#)
#
#shader_header_library(
#    name = "shader_headers",
#    srcs = ["//tools/shaderpacker_rust/headers:shader_header_src"],
#    visibility = ["//visibility:public"],
#)
#
#rust_library(
#    name = "shaderpacker_rust_lib",
#    srcs = [
#        "src/lib.rs",
#    ],
#    data = [
#        ":dxcompiler",
#        ":spriv",
#    ],
#    deps = [
#        ":rust_shader_headers",
#        "//libs/rust/error_lib",
#        "@hassle_rs",
#        "@regex",
#        "@rust_flatbuffers//:flatbuffers",
#    ],
#)
#
#rust_binary(
#    name = "shaderpacker_rust",
#    srcs = [
#        "src/main.rs",
#    ],
#    data = [
#        ":dxcompiler",
#        ":spriv",
#    ],
#    visibility = ["//visibility:public"],
#    deps = [
#        ":shaderpacker_rust_lib",
#    ],
#)
#
#rust_test(
#    name = "shaderpacker_rust_tests",
#    srcs = [
#        "tests/tests.rs",
#    ],
#    deps = [
#        ":shaderpacker_rust_lib",
#    ],
#)
