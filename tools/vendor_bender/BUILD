# gazelle:exclude vendor

load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library", "go_test")

go_binary(
    name = "vendor_bender",
    embed = [":vendor_bender_lib"],
    visibility = ["//visibility:public"],
)

go_library(
    name = "vendor_bender_lib",
    srcs = [
        "actions.go",
        "validations.go",
        "vendor_bender.go",
    ],
    data = glob([
        "testdata/**",
    ]),
    importpath = "sge-monorepo/tools/vendor_bender",
    visibility = ["//visibility:private"],
    deps = [
        "//build/cicd/monorepo",
        "//libs/go/files",
        "//libs/go/p4lib",
        "//tools/vendor_bender/bazel",
        "//tools/vendor_bender/git",
        "//tools/vendor_bender/golang",
        "//tools/vendor_bender/protos:license_go_proto",
        "//tools/vendor_bender/protos:manifest_go_proto",
        "//tools/vendor_bender/protos:metadata_go_proto",
        "//tools/vendor_bender/rust",
        "//tools/vendor_bender/zip",
        "@com_github_golang_protobuf//proto:go_default_library",
        "@org_golang_google_protobuf//encoding/prototext",
    ],
)

go_test(
    name = "vendor_bender_test",
    size = "small",
    srcs = ["vendor_bender_test.go"],
    data = glob(["testdata/**"]),
    embed = [":vendor_bender_lib"],
)

build_test(
    name = "vendor_bender_build_test",
    targets = [":vendor_bender"],
)
