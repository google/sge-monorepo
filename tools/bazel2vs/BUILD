load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "bazel2vs",
    embed = [":bazel2vs_lib"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "bazel2vs_lib",
    srcs = ["bazel2vs.go"],
    importpath = "sge-monorepo/tools/bazel2vs",
    visibility = ["//visibility:private"],
    deps = ["//tools/bazel2vs/b2vs"],
)
