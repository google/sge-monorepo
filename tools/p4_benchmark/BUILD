load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@rules_proto//proto:defs.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "p4_benchmark",
    embed = [":p4_benchmark_lib"],
    visibility = ["//visibility:public"],
)

build_test(
    name = "p4_benchmark_build_test",
    targets = [":p4_benchmark"],
)

go_library(
    name = "p4_benchmark_lib",
    srcs = [
        "csv_logger.go",
        "p4_benchmark.go",
    ],
    importpath = "sge-monorepo/tools/p4_benchmark",
    visibility = ["//visibility:private"],
    deps = [
        "//libs/go/zip_utils",
        "//tools/p4_benchmark/protos:benchmark_go_proto",
        "@com_github_golang_glog//:glog",
        "@com_github_golang_protobuf//proto:go_default_library",
        "@com_github_nu7hatch_gouuid//:gouuid",
    ],
)
