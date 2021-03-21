load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "remove_empty_directories",
    embed = [":remove_empty_directories_lib"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "remove_empty_directories_lib",
    srcs = ["remove_empty_directories.go"],
    importpath = "sge-monorepo/tools/remove_empty_directories",
    visibility = ["//visibility:private"],
)
