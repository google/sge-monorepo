load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "testbin",
    embed = [":build_test_lib"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "build_test_lib",
    srcs = ["testbin.go"],
    importpath = "libs/bzl/build_test",
    visibility = ["//visibility:private"],
)
