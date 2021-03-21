# Exclude files generated for IDE support.
# gazelle:exclude cicdfilepb

load("@rules_proto//proto:defs.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

proto_library(
    name = "cicdfile_proto",
    srcs = [
        "cicdfile.proto",
    ],
    visibility = ["//visibility:private"],
    deps = [
        "//build/cicd/presubmit/protos:presubmit_proto",
    ],
)

go_proto_library(
    name = "cicdfile_go_proto",
    importpath = "sge-monorepo/build/cicd/cicdfile/protos/cicdfilepb",
    proto = ":cicdfile_proto",
    visibility = [
        "//build/cicd:__subpackages__",
    ],
    deps = [
        "//build/cicd/presubmit/protos:presubmit_go_proto",
    ],
)
