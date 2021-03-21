load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "p4mock",
    testonly = True,
    srcs = ["p4_mock.go"],
    importpath = "sge-monorepo/libs/go/p4lib/p4mock",
    visibility = ["//visibility:public"],
    deps = [
        "//libs/go/p4lib",
    ],
)

go_test(
    name = "p4mock_test",
    size = "small",
    srcs = ["p4_mock_test.go"],
    embed = [":p4mock"],
    deps = [
        "//libs/go/p4lib",
        "@com_github_google_go_cmp//cmp",
    ],
)
