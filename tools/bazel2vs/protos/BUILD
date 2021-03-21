load("@rules_proto//proto:defs.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

proto_library(
    name = "msbuild_proto",
    srcs = ["msbuild.proto"],
    visibility = ["//tools/bazel2vs:__subpackages__"],
)

go_proto_library(
    name = "msbuild_go_proto",
    importpath = "sge-monorepo/tools/bazel2vs/protos/msbuildpb",
    proto = ":msbuild_proto",
    visibility = ["//tools/bazel2vs:__subpackages__"],
)
