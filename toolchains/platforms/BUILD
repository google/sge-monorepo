# Platform definitions.

package(default_visibility = ["//visibility:public"])

platform(
    name = "windows",
    constraint_values = [
        "@platforms//os:windows",
        "@platforms//cpu:x86_64",
        "@toolchains//go:1.16.2",
        "//platforms/windows_abi:msvc",
    ],
)

platform(
    name = "windows-gnu",
    constraint_values = [
        "@platforms//os:windows",
        "@platforms//cpu:x86_64",
        "@toolchains//go:1.16.2",
        "//platforms/windows_abi:gnu",
    ],
)

platform(
    name = "ubuntu",
    constraint_values = [
        "@platforms//os:linux",
        "@platforms//cpu:x86_64",
        "@toolchains//go:1.14.3",
        "//platforms/linux_flavor:ubuntu",
    ],
)

platform(
    name = "wasm",
    constraint_values = [
        "@platforms//cpu:wasm32",
        "@toolchains//go:1.14.3",
        "@io_bazel_rules_go//go/toolchain:cgo_off",
    ],
)
