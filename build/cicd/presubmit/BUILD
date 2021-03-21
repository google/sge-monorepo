load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "presubmit",
    srcs = ["presubmit.go"],
    importpath = "sge-monorepo/build/cicd/presubmit",
    visibility = [
        "//build/cicd:__subpackages__",
        "//tools/gigantick:__subpackages__",
    ],
    deps = [
        "//build/cicd/cicdfile",
        "//build/cicd/monorepo",
        "//build/cicd/monorepo/p4path",
        "//build/cicd/monorepo/universe",
        "//build/cicd/presubmit/check/protos:check_go_proto",
        "//build/cicd/presubmit/protos:presubmit_go_proto",
        "//build/cicd/sgeb/build",
        "//build/cicd/sgeb/protos:build_go_proto",
        "//libs/go/p4lib",
        "@com_github_golang_protobuf//proto:go_default_library",
        "@com_github_nu7hatch_gouuid//:gouuid",
    ],
)

go_test(
    name = "presubmit_test",
    srcs = ["presubmit_test.go"],
    data = glob([
        "testdata/**/*",
    ]),
    embed = [":presubmit"],
    deps = [
        "//build/cicd/cicdfile",
        "//build/cicd/monorepo",
        "//build/cicd/monorepo/universe",
        "//build/cicd/presubmit/protos:presubmit_go_proto",
        "//libs/go/p4lib",
        "//libs/go/p4lib/p4mock",
        "//libs/go/sgetest",
        "@com_github_google_go_cmp//cmp",
    ],
)
