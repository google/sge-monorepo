load("@io_bazel_rules_go//go:def.bzl", "go_embed_data", "go_library", "go_test")

#go_embed_data(
#    name = "templates",
#    srcs = [
#        "templates/solution.sln",
#        "templates/vcxproj.filters.xml",
#        "templates/vcxproj.user.xml",
#        "templates/vcxproj.xml",
#    ],
#    flatten = True,
#    package = "b2vs",
#    string = True,
#    var = "templates",
#)

go_library(
    name = "b2vs",
    srcs = [
        "b2vs.go",
        "project.go",
        "solution.go",
    ],
    embedsrcs = [
       "templates/solution.sln",
       "templates/vcxproj.filters.xml",
       "templates/vcxproj.user.xml",
       "templates/vcxproj.xml",
    ],
    importpath = "sge-monorepo/tools/bazel2vs/b2vs",
    visibility = ["//tools/bazel2vs:__subpackages__"],
    deps = [
        "//build/cicd/bep",
        "//build/cicd/monorepo",
        "//tools/bazel2vs/protos:msbuild_go_proto",
        "@com_github_golang_protobuf//proto:go_default_library",
        "@com_github_nu7hatch_gouuid//:gouuid",
        "@io_bazel//src/main/java/com/google/devtools/build/lib/buildeventstream/proto:build_event_stream_go_proto",
    ],
)

# TODO: Reactivate once the test data is re-generated.
#go_test(
#    name = "b2vs_test",
#    srcs = [
#        "b2vs_test.go",
#        "project_test.go",
#        "solution_test.go",
#    ],
#    data = glob(["testdata/**"]),
#    embed = [":b2vs"],
#    deps = [
#        "//build/cicd/bep",
#        "//build/cicd/monorepo",
#        "//libs/go/sgetest",
#        "//tools/bazel2vs/protos:msbuild_go_proto",
#        "@com_github_golang_protobuf//proto:go_default_library",
#        "@com_github_google_go_cmp//cmp",
#    ],
#)
