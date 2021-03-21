# Exclude files generated for IDE support.
# gazelle:exclude metadatapb

load("@rules_proto//proto:defs.bzl", "proto_library")
load("@io_bazel_rules_go//proto:def.bzl", "go_proto_library")

proto_library(
    name = "metadata_proto",
    srcs = [
        "metadata.proto",
    ],
    visibility = ["//visibility:private"],
)

proto_library(
    name = "license_proto",
    srcs = [
        "license.proto",
    ],
    visibility = ["//visibility:private"],
)

go_proto_library(
    name = "metadata_go_proto",
    importpath = "sge-monorepo/tools/vendor_bender/protos/metadatapb",
    proto = ":metadata_proto",
    visibility = [
        "//tools/vendor_bender:__subpackages__",
    ],
)

go_proto_library(
    name = "license_go_proto",
    importpath = "sge-monorepo/tools/vendor_bender/protos/licensepb",
    proto = ":license_proto",
    visibility = [
        "//tools/vendor_bender:__subpackages__",
    ],
)

proto_library(
    name = "manifest_proto",
    srcs = [
        "manifest.proto",
    ],
    visibility = ["//visibility:private"],
)

go_proto_library(
    name = "manifest_go_proto",
    importpath = "sge-monorepo/tools/vendor_bender/protos/manifestpb",
    proto = ":manifest_proto",
    visibility = [
        "//tools/vendor_bender:__subpackages__",
    ],
)
