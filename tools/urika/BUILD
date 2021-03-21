load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "urika",
    embed = [":urika_lib"],
    gc_linkopts = [
        "-H=windowsgui",
    ],
    visibility = ["//visibility:public"],
)

go_library(
    name = "urika_lib",
    srcs = ["urika.go"],
    importpath = "sge-monorepo/tools/urika",
    visibility = ["//visibility:private"],
    deps = [
        "//libs/go/files",
        "//libs/go/urika",
        "//libs/go/windows_utils",
        "@com_github_golang_glog//:glog",
    ],
)
