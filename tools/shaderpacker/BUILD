load(":shader_header_lib.bzl", "shader_header_library")

shader_header_library(
    name = "shader_headers",
    srcs = ["//tools/shaderpacker/headers:shader_header_src"],
    visibility = ["//visibility:public"],
)

cc_binary(
    name = "shaderpacker",
    srcs = glob([
        "*.cpp",
        "*.h",
    ]),
    visibility = ["//visibility:public"],
    deps = [
        ":shader_headers",
        "@vulkan//:dxc",
        "@vulkan//:spirv_import",
    ],
)
