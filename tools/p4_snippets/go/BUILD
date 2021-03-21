load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "p4_snippets",
    embed = [":go_lib"],
    visibility = ["//visibility:public"],
)

build_test(
    name = "p4_snippets_build_test",
    targets = [":p4_snippets"],
)

go_library(
    name = "go_lib",
    srcs = ["p4_snippets.go"],
    importpath = "sge-monorepo/tools/p4_snippets/go",
    visibility = ["//visibility:private"],
    deps = ["//libs/go/p4lib"],
)
