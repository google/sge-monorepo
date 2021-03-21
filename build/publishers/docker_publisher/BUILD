load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")
load("@//libs/bzl/build_test:build_test.bzl", "build_test")

go_library(
    name = "docker_publisher_lib",
    srcs = ["docker_publisher.go"],
    importpath = "sge-monorepo/build/publishers/docker_publisher",
    visibility = ["//visibility:private"],
    deps = [
        "//build/cicd/sgeb/buildtool",
        "//build/cicd/sgeb/protos:build_go_proto",
        "//build/publishers/docker_publisher/protos:dockerpushconfigpb_go_proto",
        "@com_github_golang_glog//:glog",
        "@com_github_golang_protobuf//proto:go_default_library",
    ],
)

go_binary(
    name = "docker_publisher",
    embed = [":docker_publisher_lib"],
    visibility = ["//visibility:public"],
)
