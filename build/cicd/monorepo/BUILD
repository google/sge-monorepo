load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "monorepo",
    srcs = ["monorepo.go"],
    importpath = "sge-monorepo/build/cicd/monorepo",
    visibility = [
        "//build/builders:__subpackages__",
        "//build/checks/checkbuildunit:__subpackages__",
        "//build/cicd:__subpackages__",
        "//build/owners:__subpackages__",
        "//build/packagemanifest:__subpackages__",
        "//tools/bazel2vs:__subpackages__",
        "//tools/gigantick:__subpackages__",
        "//tools/launcher:__subpackages__",
        "//tools/vendor_bender:__subpackages__",
    ],
    deps = ["//libs/go/trie"],
)

go_test(
    name = "monorepo_test",
    srcs = ["monorepo_test.go"],
    embed = [":monorepo"],
    deps = [
        "//libs/go/sgetest",
        "@com_github_google_go_cmp//cmp",
    ],
)
