load("//libs/bzl/build_test:build_test.bzl", "build_test")
load("@io_bazel_rules_go//go:def.bzl", "go_binary", "go_library")

go_binary(
    name = "con_job",
    embed = [":con_job_lib"],
    visibility = ["//visibility:public"],
)

build_test(
    name = "con_job_build_test",
    targets = [":con_job"],
)

go_library(
    name = "con_job_lib",
    srcs = ["con_job.go"],
    importpath = "build/con_job",
    visibility = ["//visibility:private"],
    deps = ["@go_googleapis//google/datastore/v1:datastore_go_proto"],
)
