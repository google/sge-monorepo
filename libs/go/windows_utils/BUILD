load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_test", "go_library")

go_library(
    name = "windows_utils",
    srcs = [
        "process_utils.cc",
        "windows_utils.go",
        "windows_utils.h",
    ],
    cgo = True,
    importpath = "sge-monorepo/libs/go/windows_utils",
    visibility = ["//visibility:public"],
)

go_test(
    name = "windows_utils_test",
    srcs = ["windows_utils_test.go"],
    embed = [
        ":windows_utils",
    ],
)

build_test(
    name = "windows_utils_build_test",
    targets = [":windows_utils"],
)
