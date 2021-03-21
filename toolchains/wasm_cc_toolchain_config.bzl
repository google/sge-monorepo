load("@bazel_tools//tools/build_defs/cc:action_names.bzl", "ACTION_NAMES")
load(
    "@bazel_tools//tools/cpp:cc_toolchain_config_lib.bzl",
    "env_entry",
    "env_set",
    "feature",
    "flag_group",
    "flag_set",
    "tool_path",
    "with_feature_set",
)

def _impl(ctx):
    tool_paths = [
        tool_path(name = name, path = path)
        for name, path in ctx.attr.tool_paths.items()
    ]
    preprocessor_compile_actions = [
        ACTION_NAMES.c_compile,
        ACTION_NAMES.cpp_compile,
        ACTION_NAMES.linkstamp_compile,
        ACTION_NAMES.preprocess_assemble,
        ACTION_NAMES.cpp_header_parsing,
        ACTION_NAMES.cpp_module_compile,
        ACTION_NAMES.clif_match,
    ]

    all_link_actions = [
        ACTION_NAMES.cpp_link_executable,
        ACTION_NAMES.cpp_link_dynamic_library,
        ACTION_NAMES.cpp_link_nodeps_dynamic_library,
    ]

    all_compile_actions = [
        ACTION_NAMES.c_compile,
        ACTION_NAMES.cpp_compile,
        ACTION_NAMES.linkstamp_compile,
        ACTION_NAMES.assemble,
        ACTION_NAMES.preprocess_assemble,
        ACTION_NAMES.cpp_header_parsing,
        ACTION_NAMES.cpp_module_compile,
        ACTION_NAMES.cpp_module_codegen,
        ACTION_NAMES.clif_match,
        ACTION_NAMES.lto_backend,
    ]

    crosstool_default_flag_sets = [
        # Opt.
        flag_set(
            actions = preprocessor_compile_actions,
            flag_groups = [flag_group(flags = ["-DNDEBUG"])],
            with_features = [with_feature_set(features = ["opt"])],
        ),
        flag_set(
            actions = all_compile_actions + all_link_actions,
            flag_groups = [flag_group(flags = ["-g0", "-O3"])],
            with_features = [with_feature_set(features = ["opt"])],
        ),
        # Fastbuild.
        flag_set(
            actions = all_compile_actions + all_link_actions,
            flag_groups = [flag_group(flags = ["-O2"])],
            with_features = [with_feature_set(features = ["fastbuild"])],
        ),
        # Dbg.
        flag_set(
            actions = all_compile_actions + all_link_actions,
            flag_groups = [flag_group(flags = ["-g2", "-O0"])],
            with_features = [with_feature_set(features = ["dbg"])],
        ),
    ]

    linker_param_file_feature = feature(
        name = "linker_param_file",
        flag_sets = [
            flag_set(
                actions = all_link_actions +
                          [ACTION_NAMES.cpp_link_static_library],
                flag_groups = [
                    flag_group(
                        flags = ["@%{linker_param_file}"],
                        expand_if_available = "linker_param_file",
                    ),
                ],
            ),
        ],
    )

    python_env_feature = feature(
        name = "python_env",
        enabled = True,
        env_sets = [
            env_set(
                actions = all_link_actions + all_compile_actions,
                env_entries = [env_entry(key = "PATH", value = ctx.attr.python_env_path)],
            ),
        ],
    )

    features = [
        linker_param_file_feature,
        python_env_feature,
        # These 3 features will be automatically enabled by blaze in the
        # corresponding build mode.
        feature(
            name = "opt",
            provides = ["variant:crosstool_build_mode"],
        ),
        feature(
            name = "dbg",
            provides = ["variant:crosstool_build_mode"],
        ),
        feature(
            name = "fastbuild",
            provides = ["variant:crosstool_build_mode"],
        ),
        feature(
            name = "crosstool_default_flags",
            enabled = True,
            flag_sets = crosstool_default_flag_sets,
        ),
    ]

    return cc_common.create_cc_toolchain_config_info(
        ctx = ctx,
        toolchain_identifier = "wasm-toolchain",
        host_system_name = "windows",
        target_system_name = "wasm-unknown-emscripten",
        target_cpu = "wasm",
        target_libc = "unknown",
        compiler = "emscripten",
        abi_version = "unknown",
        abi_libc_version = "unknown",
        tool_paths = tool_paths,
        features = features,
    )

cc_toolchain_config = rule(
    implementation = _impl,
    attrs = {
        "tool_paths": attr.string_dict(),
        "python_env_path": attr.string(default = "python_not_found"),
    },
    provides = [CcToolchainConfigInfo],
)
