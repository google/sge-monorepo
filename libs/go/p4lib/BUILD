load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

go_library(
    name = "p4lib",
    srcs = [
        "p4.go",
        "p4_cgo_api.go",
        "p4_cgo_bridge.cc",
        "p4_cgo_bridge.h",
        "p4_cgo_strview.go",
        "p4_changes.go",
        "p4_describe.go",
        "p4_fstat.go",
        "p4_impl.go",
        "p4_impl_default.go",
        "p4_impl_windows.go",
        "p4_keys.go",
        "p4_login.go",
        "p4_print.go",
        "p4_where.go",
    ],
    cdeps = [
        "@p4api//:p4api",
    ],
    cgo = True,
    importpath = "sge-monorepo/libs/go/p4lib",
    visibility = ["//visibility:public"],
    deps = [
        "@com_github_golang_glog//:glog",
    ],
)

go_test(
    name = "p4lib_test",
    size = "small",
    srcs = ["p4_test.go"],
    data = glob(["testdata/**"]),
    embed = [":p4lib"],
    deps = [
        "//libs/go/sgetest",
        "@com_github_google_go_cmp//cmp",
    ],
)
