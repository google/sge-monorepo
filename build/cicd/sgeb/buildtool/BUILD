load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "buildtool",
    srcs = ["buildtool.go"],
    importpath = "sge-monorepo/build/cicd/sgeb/buildtool",
    visibility = [
        "//build:__subpackages__",
    ],
    deps = [
        "//build/cicd/monorepo",
        "//build/cicd/sgeb/protos:build_go_proto",
        "@com_github_golang_glog//:glog",
        "@com_github_golang_protobuf//proto:go_default_library",
    ],
)

go_test(
    name = "buildtool_test",
    srcs = ["buildtool_test.go"],
    embed = [":buildtool"],
    deps = [
        "//build/cicd/sgeb/build",
        "//build/cicd/sgeb/protos:build_go_proto",
        "@com_github_golang_protobuf//proto:go_default_library",
    ],
)
