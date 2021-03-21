load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_embed_data")

# gazelle:ignore

go_embed_data(
    name = "fonts",
    srcs = [
        "GoogleSans-Regular.ttf",
        "consolas-l1x.ttf",
    ],
    flatten = True,
    package = "main",
    var = "FontData",
)

go_binary(
    name = "gigantick",
    srcs = [
        "change.go",
        "diff.go",
        "gigantick.go",
        "diff_gui.go",
        "main_gui.go",
        "tests_gui.go",
        "install.go",
        "routine.go",
        "server.go",
        "view.go",
        ":fonts",
    ],
    gc_linkopts = select({
        "//libs/bzl/config_settings:opt": ["-H=windowsgui"],
        "//conditions:default": [],
    }),
    deps = [
        "//build/cicd/cicdfile",
        "//build/cicd/monorepo",
        "//build/cicd/monorepo/universe",
        "//build/cicd/presubmit",
        "//build/cicd/presubmit/protos:presubmit_go_proto",
        "//build/cicd/sgeb/protos:build_go_proto",
        "//libs/go/files",
        "//libs/go/imguix",
        "//libs/go/p4lib",
        "//libs/go/swarm",
        "@com_github_allendang_giu//:go_default_library",
        "@com_github_allendang_giu//imgui:go_default_library",
        "@com_github_atotto_clipboard//:go_default_library",
        "@com_github_golang_glog//:glog",
        "@org_golang_x_sys//windows/registry:go_default_library",
    ],
)
