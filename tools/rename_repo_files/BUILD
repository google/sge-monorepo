load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

go_binary(
    name = "rename_repo_files",
    embed = [":rename_repo_files_lib"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "rename_repo_files_lib",
    srcs = ["rename_repo_files.go"],
    importpath = "sge-monorepo/tools/rename_repo_files",
    visibility = ["//visibility:private"],
    deps = [
        "//libs/go/p4lib",
    ],
)

go_test(
    name = "rename_repo_files_test",
    srcs = ["rename_repo_files_test.go"],
    data = glob([
        "testdata/**",
    ]),
    embed = [":rename_repo_files_lib"],
)
