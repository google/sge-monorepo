# Conditions used in select statements

package(default_visibility = ["//visibility:public"])

# Usage looks like this:
# select({
#     "@toolchains/conditions:windows": "UseThis",
#     "@toolchains/conditions:linux": "UseThat",
# })

config_setting(
    name = "windows",
    constraint_values = [
        "@platforms//os:windows",
        "@platforms//cpu:x86_64",
    ],
)

config_setting(
    name = "windows-msvc",
    constraint_values = [
        "@platforms//os:windows",
        "@platforms//cpu:x86_64",
        "//platforms/windows_abi:msvc",
    ],
)

config_setting(
    name = "windows-gnu",
    constraint_values = [
        "@platforms//os:windows",
        "@platforms//cpu:x86_64",
        "//platforms/windows_abi:gnu",
    ],
)

config_setting(
    name = "linux",
    constraint_values = [
        "@platforms//os:linux",
        "@platforms//cpu:x86_64",
    ],
)

config_setting(
    name = "ubuntu",
    constraint_values = [
        "@platforms//os:linux",
        "@platforms//cpu:x86_64",
        "//platforms/linux_flavor:ubuntu",
    ],
)

config_setting(
    name = "wasm",
    constraint_values = [
        "@platforms//cpu:wasm32",
    ],
)

config_setting(
    name = "dbg",
    values = {
        "compilation_mode": "dbg",
    },
)

config_setting(
    name = "fastbuild",
    values = {
        "compilation_mode": "fastbuild",
    },
)

config_setting(
    name = "opt",
    values = {
        "compilation_mode": "opt",
    },
)
