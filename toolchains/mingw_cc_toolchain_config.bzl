# Copyright 2019 The Bazel Authors. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""A Starlark cc_toolchain configuration rule for Windows MinGW"""

load(
    "@rules_cc//cc:cc_toolchain_config_lib.bzl",
    "artifact_name_pattern",
    "env_entry",
    "env_set",
    "feature",
    "flag_group",
    "flag_set",
    "tool_path",
)

load("@rules_cc//cc:action_names.bzl", "ACTION_NAMES")

all_link_actions = [
    ACTION_NAMES.cpp_link_executable,
    ACTION_NAMES.cpp_link_dynamic_library,
    ACTION_NAMES.cpp_link_nodeps_dynamic_library,
]

def _impl(ctx):
    artifact_name_patterns = [
        artifact_name_pattern(
            category_name = "executable",
            prefix = "",
            extension = ".exe",
        ),
    ]

    action_configs = []

    targets_windows_feature = feature(
        name = "targets_windows",
        implies = ["copy_dynamic_libraries_to_binary"],
        enabled = True,
    )

    copy_dynamic_libraries_to_binary_feature = feature(name = "copy_dynamic_libraries_to_binary")

    gcc_env_feature = feature(
        name = "gcc_env",
        enabled = True,
        env_sets = [
            env_set(
                actions = [
                    ACTION_NAMES.c_compile,
                    ACTION_NAMES.cpp_compile,
                    ACTION_NAMES.cpp_module_compile,
                    ACTION_NAMES.cpp_module_codegen,
                    ACTION_NAMES.cpp_header_parsing,
                    ACTION_NAMES.assemble,
                    ACTION_NAMES.preprocess_assemble,
                    ACTION_NAMES.cpp_link_executable,
                    ACTION_NAMES.cpp_link_dynamic_library,
                    ACTION_NAMES.cpp_link_nodeps_dynamic_library,
                    ACTION_NAMES.cpp_link_static_library,
                ],
                env_entries = [
                    env_entry(key = "PATH", value = ctx.attr.tool_bin_path),
                ],
            ),
        ],
    )

    default_compile_flags_feature = feature(
        name = "default_compile_flags",
        enabled = True,
        flag_sets = [
            flag_set(
                actions = [
                    ACTION_NAMES.linkstamp_compile,
                    ACTION_NAMES.cpp_compile,
                    ACTION_NAMES.cpp_header_parsing,
                    ACTION_NAMES.cpp_module_compile,
                    ACTION_NAMES.cpp_module_codegen,
                    ACTION_NAMES.lto_backend,
                    ACTION_NAMES.clif_match,
                ],
                flag_groups = [flag_group(flags = ["-std=gnu++0x"])],
            ),
        ],
    )

    default_link_flags_feature = feature(
        name = "default_link_flags",
        enabled = True,
        flag_sets = [
            flag_set(
                actions = all_link_actions,
                flag_groups = [
                    flag_group(flags = [
                        "-lstdc++",
                        # This overrides -pie and prevents linking with the shared libraries.
                        "-static",
                        # Pass flag to linker to make it zero out the timestamp for determinism.
                        "-Wl,--no-insert-timestamp",
                    ]),
                ],
            ),
        ],
    )

    supports_dynamic_linker_feature = feature(
        name = "supports_dynamic_linker",
        enabled = True,
    )

    compiler_param_file_feature = feature(
        name = "compiler_param_file",
    )

    features = [
        targets_windows_feature,
        copy_dynamic_libraries_to_binary_feature,
        gcc_env_feature,
        default_compile_flags_feature,
        compiler_param_file_feature,
        default_link_flags_feature,
        supports_dynamic_linker_feature,
    ]

    tool_paths = [
        tool_path(name = name, path = path)
        for name, path in ctx.attr.tool_paths.items()
    ]

    return cc_common.create_cc_toolchain_config_info(
        ctx = ctx,
        features = features,
        action_configs = action_configs,
        artifact_name_patterns = artifact_name_patterns,
        cxx_builtin_include_directories = ctx.attr.cxx_builtin_include_directories,
        toolchain_identifier = ctx.attr.toolchain_identifier,
        host_system_name = ctx.attr.host_system_name,
        target_system_name = ctx.attr.target_system_name,
        target_cpu = ctx.attr.cpu,
        target_libc = ctx.attr.target_libc,
        compiler = ctx.attr.compiler,
        abi_version = ctx.attr.abi_version,
        abi_libc_version = ctx.attr.abi_libc_version,
        tool_paths = tool_paths,
    )

cc_toolchain_config = rule(
    implementation = _impl,
    attrs = {
        "cpu": attr.string(mandatory = True),
        "compiler": attr.string(),
        "toolchain_identifier": attr.string(),
        "host_system_name": attr.string(),
        "target_system_name": attr.string(),
        "target_libc": attr.string(),
        "abi_version": attr.string(),
        "abi_libc_version": attr.string(),
        "tool_paths": attr.string_dict(),
        "cxx_builtin_include_directories": attr.string_list(),
        "default_link_flags": attr.string_list(default = []),
        "msvc_env_tmp": attr.string(default = "msvc_not_found"),
        "msvc_env_vcinstalldir": attr.string(default = "msvc_not_found"),
        "msvc_env_path": attr.string(default = "msvc_not_found"),
        "msvc_env_include": attr.string(default = "msvc_not_found"),
        "msvc_env_lib": attr.string(default = "msvc_not_found"),
        "msvc_cl_path": attr.string(default = "vc_installation_error.bat"),
        "msvc_ml_path": attr.string(default = "vc_installation_error.bat"),
        "msvc_link_path": attr.string(default = "vc_installation_error.bat"),
        "msvc_lib_path": attr.string(default = "vc_installation_error.bat"),
        "dbg_mode_debug_flag": attr.string(),
        "fastbuild_mode_debug_flag": attr.string(),
        "tool_bin_path": attr.string(default = "not_found"),
    },
    provides = [CcToolchainConfigInfo],
)
