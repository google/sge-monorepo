load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

go_library(
    name = "checkfmt_lib",
    srcs = ["checkfmt.go"],
    importpath = "build/checks/checkfmt",
    visibility = ["//visibility:private"],
    deps = [
        "//build/cicd/presubmit/check",
        "//build/cicd/presubmit/check/protos:check_go_proto",
        "//build/cicd/sgeb/protos:build_go_proto",
        "//libs/go/sgeflag",
        "@com_github_google_go_cmp//cmp",
    ],
)

go_binary(
    name = "checkfmt",
    embed = [":checkfmt_lib"],
    visibility = ["//visibility:public"],
)

go_test(
    name = "checkfmt_test",
    srcs = ["checkfmt_test.go"],
    data = [
        "testdata/bad.go.testinput",
        "testdata/badsyntax.go.testinput",
        "testdata/good.go.testinput",
        # TODO: Enable this when there is a way to refer to a checked-in gofmt.
        #"//bin/windows:gofmt",
    ],
    embed = [":checkfmt_lib"],
    deps = [
        "//build/cicd/presubmit/check/checkmock",
        "//build/cicd/presubmit/check/protos:check_go_proto",
        "//libs/go/sgetest",
    ],
)
