load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "check",
    srcs = ["check.go"],
    importpath = "sge-monorepo/build/cicd/presubmit/check",
    visibility = [
        "//build/checks:__subpackages__",
        "//build/cicd:__subpackages__",
    ],
    deps = [
        "//build/cicd/monorepo",
        "//build/cicd/presubmit/check/protos:check_go_proto",
        "//build/cicd/sgeb/protos:build_go_proto",
        "@com_github_golang_glog//:glog",
        "@com_github_golang_protobuf//proto:go_default_library",
    ],
)

go_test(
    name = "check_test",
    srcs = ["check_test.go"],
    embed = [":check"],
    deps = [
        "//build/cicd/presubmit/check/protos:check_go_proto",
        "//build/cicd/sgeb/protos:build_go_proto",
        "@com_github_golang_protobuf//proto:go_default_library",
    ],
)
