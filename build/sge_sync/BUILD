load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "sge_sync",
    embed = [":sge_sync_lib"],
    visibility = ["//visibility:public"],
)

build_test(
    name = "sge_sync_build_test",
    targets = [":sge_sync"],
)

go_library(
    name = "sge_sync_lib",
    srcs = ["sge_sync.go"],
    importpath = "build/sge_sync",
    visibility = ["//visibility:private"],
    deps = ["//libs/go/builddist"],
)
