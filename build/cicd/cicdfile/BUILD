load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "cicdfile",
    srcs = ["cicdfile.go"],
    importpath = "sge-monorepo/build/cicd/cicdfile",
    visibility = [
        "//build/cicd:__subpackages__",
        "//tools/gigantick:__subpackages__",
    ],
    deps = [
        "//build/cicd/cicdfile/protos:cicdfile_go_proto",
        "//build/cicd/monorepo",
        "@com_github_golang_protobuf//proto:go_default_library",
    ],
)

go_test(
    name = "cicdfile_test",
    srcs = ["cicdfile_test.go"],
    data = glob([
        "testdata/**",
    ]),
    embed = [":cicdfile"],
    deps = [
        "//build/cicd/cicdfile/protos:cicdfile_go_proto",
        "//build/cicd/monorepo",
    ],
)
