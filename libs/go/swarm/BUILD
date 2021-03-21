load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "swarm",
    srcs = ["swarm.go"],
    importpath = "sge-monorepo/libs/go/swarm",
    visibility = ["//visibility:public"],
    deps = ["//libs/go/log"],
)

build_test(
    name = "swarm_build_test",
    targets = [":swarm"],
)
