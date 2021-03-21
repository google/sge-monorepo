load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "perculator",
    embed = [":perculator_lib"],
    gc_linkopts = select({
        "//libs/bzl/config_settings:opt": ["-H=windowsgui"],
        "//conditions:default": [],
    }),
    visibility = ["//visibility:public"],
)

go_library(
    name = "perculator_lib",
    srcs = [
        "gui.go",
        "perculator.go",
    ],
    importpath = "sge-monorepo/tools/perculator",
    visibility = ["//visibility:private"],
    deps = [
        "//libs/go/files",
        "//libs/go/p4lib",
        "//libs/go/swarm",
        "@com_github_allendang_giu//:giu",
        "@com_github_allendang_giu//imgui",
        "@com_github_atotto_clipboard//:clipboard",
        "@com_github_golang_glog//:glog",
        "@org_golang_x_text//language",
        "@org_golang_x_text//message",
    ],
)
