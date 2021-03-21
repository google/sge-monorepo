# Exclude files generated for IDE support.
# gazelle:exclude presubmitpb

load("@rules_proto//proto:defs.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

proto_library(
    name = "presubmit_proto",
    srcs = [
        "presubmit.proto",
    ],
    visibility = [
        "//build/cicd:__subpackages__",
        "//tools/gigantick:__subpackages__",
    ],
    deps = [
        "//build/cicd/presubmit/check/protos:check_proto",
        "//build/cicd/sgeb/protos:build_proto",
    ],
)

go_proto_library(
    name = "presubmit_go_proto",
    importpath = "sge-monorepo/build/cicd/presubmit/protos/presubmitpb",
    proto = ":presubmit_proto",
    visibility = [
        "//build/cicd:__subpackages__",
        "//tools/gigantick:__subpackages__",
    ],
    deps = [
        "//build/cicd/presubmit/check/protos:check_go_proto",
        "//build/cicd/sgeb/protos:build_go_proto",
    ],
)
