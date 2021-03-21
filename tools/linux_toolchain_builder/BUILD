load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

filegroup(
    name = "docker_files",
    srcs = [
        "docker/Dockerfile",
        "docker/build.sh",
        "docker/crosstool.config",
    ],
)

go_binary(
    name = "linux_toolchain_builder",
    data = [
        "remote-commands.sh",
        ":docker_files",
    ],
    embed = [":linux_toolchain_builder_lib"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "linux_toolchain_builder_lib",
    srcs = ["main.go"],
    importpath = "sge-monorepo/tools/linux_toolchain_builder",
    visibility = ["//visibility:private"],
    deps = [
        "@org_golang_google_api//compute/v1:compute",
        "@org_golang_x_oauth2//:oauth2",
        "@org_golang_x_oauth2//google",
    ],
)
