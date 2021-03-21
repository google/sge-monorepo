load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "urika",
    srcs = ["registry.go"],
    importpath = "sge-monorepo/libs/go/urika",
    visibility = ["//visibility:public"],
    deps = [
        "@org_golang_x_sys//windows/registry",
    ],
)

build_test(
    name = "urika_build_test",
    targets = [":urika"],
)
