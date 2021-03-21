load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "build_dist_cli",
    embed = [":build_dist_cli_lib"],
    visibility = ["//visibility:public"],
)

build_test(
    name = "build_dist_cli_build_test",
    targets = [":build_dist_cli"],
)

go_library(
    name = "build_dist_cli_lib",
    srcs = ["build_dist.go"],
    importpath = "build/build_dist_cli",
    visibility = ["//visibility:private"],
    deps = [
        "//libs/go/builddist",
        "@org_golang_google_api//option",
    ],
)
