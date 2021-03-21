# Exclude files generated for IDE support.
# gazelle:exclude checkpb

load("@rules_proto//proto:defs.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

proto_library(
    name = "check_proto",
    srcs = [
        "check.proto",
    ],
    visibility = [
        "//build/cicd:__subpackages__",
    ],
    deps = [
        "//build/cicd/sgeb/protos:build_proto",
    ],
)

go_proto_library(
    name = "check_go_proto",
    importpath = "sge-monorepo/build/cicd/presubmit/check/protos/checkpb",
    proto = ":check_proto",
    visibility = [
        "//build/checks:__subpackages__",
        "//build/cicd:__subpackages__",
    ],
    deps = [
        "//build/cicd/sgeb/protos:build_go_proto",
    ],
)
