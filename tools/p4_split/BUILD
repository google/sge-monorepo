load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "p4_split",
    embed = [":p4_split_lib"],
    visibility = ["//visibility:public"],
)

build_test(
    name = "p4_split_build_test",
    targets = [":p4_split"],
)

go_library(
    name = "p4_split_lib",
    srcs = ["p4_split.go"],
    importpath = "sge-monorepo/tools/p4_split",
    visibility = ["//visibility:private"],
)
