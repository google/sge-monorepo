load("@io_bazel_rules_go//go:def.bzl", "go_library")

go_library(
    name = "auth",
    srcs = ["auth.go"],
    importpath = "sge-monorepo/libs/go/auth",
    visibility = ["//visibility:public"],
    deps = [
        "@org_golang_x_oauth2//:oauth2",
        "@org_golang_x_oauth2//google",
    ],
)
