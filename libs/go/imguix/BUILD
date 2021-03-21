load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "imguix",
    srcs = ["imguix.go"],
    importpath = "sge-monorepo/libs/go/imguix",
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_allendang_giu//imgui",
    ],
)

build_test(
    name = "imguix_build_test",
    targets = [":imguix"],
)
