# gazelle:ignore
load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "builddist",
    srcs = [
        "auth.go",
        "common.go",
        "config.go",
        "delete_package.go",
        "install_package.go",
        "list_packages.go",
        "make_package.go",
        "register_product.go",
    ],
    importpath = "sge-monorepo/libs/go/builddist",
    visibility = [
        "//build/build_dist_cli:__subpackages__",
        "//build/sge_sync:__subpackages__",
        "//tools/kitchen:__subpackages__",
    ],
    deps = [
        "//libs/go/auth",
        "@com_google_cloud_go_datastore//:go_default_library",
        "@com_google_cloud_go_storage//:go_default_library",
        "@org_golang_google_api//iterator",
        "@org_golang_google_api//option",
        "@org_golang_x_oauth2//:oauth2",
    ],
)

build_test(
    name = "builddist_build_test",
    targets = [":builddist"],
)
