load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_embed_data", "go_library")

go_embed_data(
    name = "fonts",
    srcs = [
        "GoogleSans-Regular.ttf",
    ],
    flatten = True,
    package = "main",
    var = "FontData",
)

go_binary(
    name = "poogle",
    embed = [":poogle_lib"],
    gc_linkopts = select({
        "//libs/bzl/config_settings:opt": ["-H=windowsgui"],
        "//conditions:default": [],
    }),
    visibility = ["//visibility:public"],
)

go_library(
    name = "poogle_lib",
    srcs = [
        "poogle.go",
        ":fonts",  # keep
    ],
    importpath = "sge-monorepo/tools/poogle",
    visibility = ["//visibility:private"],
    deps = [
        "//libs/go/files",
        "//libs/go/imguix",
        "//libs/go/p4lib",
        "@com_github_allendang_giu//:giu",
        "@com_github_allendang_giu//imgui",
        "@com_github_atotto_clipboard//:clipboard",
    ],
)
