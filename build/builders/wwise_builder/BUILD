load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "wwise_builder",
    embed = [":wwise_builder_lib"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "wwise_builder_lib",
    srcs = ["wwise_builder.go"],
    importpath = "build/builders/wwise_builder",
    visibility = ["//visibility:private"],
    deps = [
        "//build/builders/wwise_builder/protos:soundbank_go_proto",
        "//build/cicd/sgeb/buildtool",
        "//build/cicd/sgeb/protos:build_go_proto",
        "//libs/go/files",
        "//libs/go/log",
        "@com_github_golang_protobuf//proto:go_default_library",
    ],
)
