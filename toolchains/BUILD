# Toolchain definitions.

load(":windows_cc_toolchain_config.bzl", "cc_toolchain_config")
load(":mingw_cc_toolchain_config.bzl", mingw_cc_toolchain_config = "cc_toolchain_config")
load(":linux_cc_toolchain_config.bzl", linux_cc_toolchain_config = "cc_toolchain_config")
load(":wasm_cc_toolchain_config.bzl", wasm_cc_toolchain_config = "cc_toolchain_config")
load("@rules_cc//cc:defs.bzl", "cc_import", "cc_toolchain", "cc_toolchain_suite")
load("@rules_python//python:defs.bzl", "py_runtime_pair")
load("@rules_winsdk//:winsdk_toolchain.bzl", "WINSDK_TOOLCHAIN_TYPE", "winsdk_toolchain")

package(default_visibility = ["//visibility:public"])

filegroup(
    name = "empty",
    srcs = [],
)

# For command line options we need to know where this repository
# will be placed.
PKG = "external/{repo}/".format(repo = repository_name()[1:])

#### WINDOWS TOOLCHAIN ####################################################

LLVM = "llvm/10.0.0"

CLANG = LLVM + "/lib/clang/10.0.0"

MSVC = "msvc/14.25.28610"

ATLMFC = MSVC + "/atlmfc"

WINSDK = "winsdk/10.0.18362.0"

WININC = WINSDK + "/Include/10.0.18362.0"

WINLIB = WINSDK + "/Lib/10.0.18362.0"

DIASDK = "diasdk/VS2019"

NETFX = "netfxsdk/4.6.2"

filegroup(
    name = "clangcl_compiler_files",
    srcs = [
        ":builtin_include_directory_paths_clangcl",
        LLVM + "/bin/clang-cl.exe",
    ] + glob([
        WININC + "/ucrt/**/*",
        MSVC + "/include/**/*",
        ATLMFC + "/include/**/*",
        WININC + "/um/**/*",
        WININC + "/shared/**/*",
        LLVM + "/include/**/*",
        CLANG + "/include/**/*",
        DIASDK + "/include/**/*",
    ]),
)

filegroup(
    name = "clangcl_linker_files",
    srcs = [
        LLVM + "/bin/lld-link.exe",
        LLVM + "/bin/llvm-lib.exe",
    ] + glob([
        LLVM + "/lib/*",
        CLANG + "/lib/windows/**/*",
        MSVC + "/lib/x64/**/*",
        ATLMFC + "/lib/x64/**/*",
        WINLIB + "/um/x64/**/*",
        WINLIB + "/ucrt/x64/**/*",
        DIASDK + "/lib/amd64/**/*",
    ]),
)

filegroup(
    name = "clangcl_all_files",
    srcs = [
        ":clangcl_compiler_files",
        ":clangcl_linker_files",
    ],
)

cc_toolchain(
    name = "cc_windows_toolchain_impl",
    all_files = ":clangcl_all_files",
    ar_files = ":clangcl_linker_files",
    as_files = ":empty",
    compiler_files = ":clangcl_compiler_files",
    dwp_files = ":empty",
    linker_files = ":clangcl_linker_files",
    objcopy_files = ":empty",
    strip_files = ":empty",
    supports_param_files = 1,
    toolchain_config = ":clang_cl_x64",
    toolchain_identifier = "clang_cl_x64",
)

cc_toolchain_config(
    name = "clang_cl_x64",
    abi_libc_version = "local",
    abi_version = "local",
    compiler = "clang-cl",
    cpu = "x64_windows",
    cxx_builtin_include_directories = [
        PKG + MSVC + "/include",
        PKG + ATLMFC + "/include",
        PKG + NETFX + "/include/um",
        PKG + WININC + "/ucrt",
        PKG + WININC + "/shared",
        PKG + WININC + "/um",
        PKG + WININC + "/winrt",
        PKG + WININC + "/cppwinrt",
        PKG + LLVM + "/include/clang-c",
        PKG + LLVM + "/include/llvm-c",
        PKG + CLANG + "/include",
        PKG + DIASDK + "/include",
    ],
    dbg_mode_debug_flag = "/DEBUG",
    default_link_flags = [
        "/MACHINE:X64",
        "/DEFAULTLIB:clang_rt.builtins-x86_64.lib",
        # Deterministic builds
        "/timestamp:0",
    ],
    fastbuild_mode_debug_flag = "/DEBUG",
    host_system_name = "local",
    msvc_cl_path = LLVM + "/bin/clang-cl.exe",
    msvc_env_include = ";".join([
        PKG + LLVM + "/include/clang-c",
        PKG + LLVM + "/include/llvm-c",
        PKG + CLANG + "/include",
        PKG + WININC + "/ucrt",
        PKG + MSVC + "/include",
        PKG + ATLMFC + "/include",
        PKG + WININC + "/um",
        PKG + WININC + "/shared",
        PKG + DIASDK + "/include",
    ]),
    msvc_env_lib = ";".join([
        PKG + LLVM + "/lib",
        PKG + CLANG + "/lib/windows",
        PKG + MSVC + "/lib/x64",
        PKG + ATLMFC + "/lib/x64",
        PKG + WINLIB + "/um/x64",
        PKG + WINLIB + "/ucrt/x64",
        PKG + DIASDK + "/lib/amd64",
    ]),

    # These contain way too much. Should probably be brought in as needed.
    #msvc_env_path = "autosdk\\HostWin64\\Win64\\VS2019\\14.25.28610\\bin\\HostX64\\x64;C:\\Program Files (x86)\\Microsoft SDKs\\Windows\\v10.0A\\bin\\NETFX 4.6.1 Tools\\x64\\;C:\\Program Files (x86)\\Windows Kits\\10\\bin\\10.0.18362.0\\x64;C:\\Program Files (x86)\\Windows Kits\\10\\bin\\x64;C:\\Program Files (x86)\\Microsoft Visual Studio\\2017\\Professional\\\\MSBuild\\15.0\\bin;C:\\Windows\\Microsoft.NET\\Framework64\\v4.0.30319;C:\\Program Files (x86)\\Microsoft Visual Studio\\2017\\Professional\\Common7\\IDE\\;C:\\Program Files (x86)\\Microsoft Visual Studio\\2017\\Professional\\Common7\\Tools\\;;C:\\Windows\\system32;C:\\Program Files (x86)\\Microsoft Visual Studio\\2017\\Professional\\Common7\\IDE\\CommonExtensions\\Microsoft\\CMake\\CMake\\bin;C:\\Program Files (x86)\\Microsoft Visual Studio\\2017\\Professional\\Common7\\IDE\\CommonExtensions\\Microsoft\\CMake\\Ninja",
    #msvc_env_include = "autosdk\\HostWin64\\Win64\\VS2019\\14.25.28610\\ATLMFC\\include;autosdk\\HostWin64\\Win64\\VS2019\\14.25.28610\\include;C:\\Program Files (x86)\\Windows Kits\\NETFXSDK\\4.6.1\\include\\um;C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.18362.0\\ucrt;C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.18362.0\\shared;C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.18362.0\\um;C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.18362.0\\winrt;C:\\Program Files (x86)\\Windows Kits\\10\\include\\10.0.18362.0\\cppwinrt;D:\\sge\\external\\sge_test_cases\\sgegame\\build\\autosdk\\HostWin64\\Win64\\LLVM\\10.0.0\\lib\\clang\\10.0.0\\include",
    #msvc_env_lib = "autosdk\\HostWin64\\Win64\\VS2019\\14.25.28610\\ATLMFC\\lib\\x64;autosdk\\HostWin64\\Win64\\VS2019\\14.25.28610\\lib\\x64;C:\\Program Files (x86)\\Windows Kits\\NETFXSDK\\4.6.1\\lib\\um\\x64;C:\\Program Files (x86)\\Windows Kits\\10\\lib\\10.0.18362.0\\ucrt\\x64;C:\\Program Files (x86)\\Windows Kits\\10\\lib\\10.0.18362.0\\um\\x64;;D:\\sge\\external\\sge_test_cases\\sgegame\\build\\autosdk\\HostWin64\\Win64\\LLVM\\10.0.0\\lib\\clang\\10.0.0\\lib\\windows",
    msvc_env_path = MSVC + "/bin/HostX64/x64",
    msvc_lib_path = LLVM + "/bin/llvm-lib.exe",
    msvc_link_path = LLVM + "/bin/lld-link.exe",
    msvc_ml_path = MSVC + "/bin/HostX64/x64/ml64.exe",
    opt_mode_debug_flag = "/DEBUG",
    target_libc = "msvcrt",
    target_system_name = "local",
    tool_paths = {
        "ar": LLVM + "/bin/llvm-lib.exe",
        "ml": MSVC + "/bin/HostX64/x64/ml64.exe",
        "cpp": LLVM + "/bin/clang-cl.exe",
        "gcc": LLVM + "/bin/clang-cl.exe",
        "gcov": "wrapper/bin/msvc_nop.bat",
        "ld": LLVM + "/bin/lld-link.exe",
        "nm": "wrapper/bin/msvc_nop.bat",
        "objcopy": "wrapper/bin/msvc_nop.bat",
        "objdump": "wrapper/bin/msvc_nop.bat",
        "strip": "wrapper/bin/msvc_nop.bat",
    },
    toolchain_identifier = "clang_cl_x64",
)

toolchain(
    name = "cc_windows_toolchain",
    exec_compatible_with = [
        "@platforms//cpu:x86_64",
        "@platforms//os:windows",
    ],
    target_compatible_with = [
        "@platforms//cpu:x86_64",
        "@platforms//os:windows",
        "//platforms/windows_abi:msvc",
    ],
    toolchain = ":cc_windows_toolchain_impl",
    toolchain_type = "@rules_cc//cc:toolchain_type",
)

#### WIN GNU ABI TOOLCHAIN ###############################################

MINGW_ROOT = "mingw/8.1.0"

MINGW_GCC_PATH = MINGW_ROOT + "/lib/gcc/x86_64-w64-mingw32/8.1.0"

mingw_cc_toolchain_config(
    name = "windows_gnu_toolchain_config",
    abi_libc_version = "local",
    abi_version = "local",
    compiler = "mingw-gcc",
    cpu = "x64_windows",
    cxx_builtin_include_directories = [
        MINGW_GCC_PATH + "/include/c++",
        MINGW_ROOT + "/x86_64-w64-mingw32/include",
        MINGW_ROOT + "/usr/include",
        MINGW_GCC_PATH + "/include",
        MINGW_GCC_PATH + "/include-fixed",
    ],
    host_system_name = "local",
    target_libc = "mingw",
    target_system_name = "local",
    tool_bin_path = PKG + MINGW_ROOT + "/bin",
    tool_paths = {
        "gcc": MINGW_ROOT + "/bin/gcc.exe",
        "ar": MINGW_ROOT + "/bin/ar.exe",
        "ld": MINGW_ROOT + "/bin/ld.exe",
        "cpp": MINGW_ROOT + "/bin/c++.exe",
        "gcov": MINGW_ROOT + "/bin/gcov.exe",
        "nm": MINGW_ROOT + "/bin/nm.exe",
        "objcopy": MINGW_ROOT + "/bin/objcopy.exe",
        "objdump": MINGW_ROOT + "/bin/objdump.exe",
        "strip": MINGW_ROOT + "/bin/strip.exe",
    },
    toolchain_identifier = "mingw_toolchain",
)

cc_toolchain(
    name = "cc_windows_gnu_toolchain_impl",
    all_files = ":empty",
    compiler_files = ":empty",
    dwp_files = ":empty",
    features = None,
    linker_files = ":empty",
    objcopy_files = ":empty",
    strip_files = ":empty",
    supports_param_files = False,
    toolchain_config = ":windows_gnu_toolchain_config",
    toolchain_identifier = "windows-gnu-toolchain",
)

toolchain(
    name = "cc_windows_gnu_toolchain",
    exec_compatible_with = [
        "@platforms//cpu:x86_64",
        "@platforms//os:windows",
    ],
    target_compatible_with = [
        "@platforms//cpu:x86_64",
        "@platforms//os:windows",
        "//platforms/windows_abi:gnu",
    ],
    toolchain = ":cc_windows_gnu_toolchain_impl",
    toolchain_type = "@rules_cc//cc:toolchain_type",
)

#### LINUX TOOLCHAIN ####################################################

LINUX_ROOT = "linux/ubuntu-18.04"

linux_cc_toolchain_config(
    name = "linux_toolchain_config",
    abi_libc_version = "glibc_2.17",
    abi_version = "clang",
    compile_flags = [
        "-Wall",
        "-fno-omit-frame-pointer",
        "--target=x86_64-unknown-linux-gnu",
    ],
    compiler = "llvm",
    coverage_compile_flags = [
        '"-fprofile-instr-generate", "-fcoverage-mapping"',
    ],
    coverage_link_flags = [
        '"-fprofile-instr-generate"',
    ],
    cpu = "local",
    cxx_builtin_include_directories = [
        LINUX_ROOT + "/include/c++/8.3.0",
        LINUX_ROOT + "/usr/include",
        CLANG + "/include",
    ],
    cxx_flags = [],
    dbg_compile_flags = ["-g"],
    host_system_name = "windows",
    link_flags = [
        # Using the x86_64 linux tripplet that gets passed on to LLVM to target
        # Linux
        "--target=x86_64-unknown-linux-gnu",
        # By default clang on Windows defaults to ld-link.exe, this force the
        # use of ld-lld
        "-fuse-ld=lld",
        # The -m64 option sets int to 32 bits and long and pointer types
        # to 64 bits, and generates code for the x86-64 architecture.
        # normally auto determined by the compiler, set here explicitely
        # in cases where the linker is invoked in other contexts like
        # Rust
        "-m64",
        # Always enable NX protection, makes stack pages not executable,
        # doesn't cost extra and supported on x86 in hardware
        "-Wl,-znoexecstack",
    ],
    link_libs = [
        # Force static linking of libstdc++.  Otherwise we can get dynamic
        # linking errors when deploying to VMs that don't have the same
        # version of libstdc++.so installed (for example our Rampart VMs
        # use Ubuntu 16.04 which uses libstdc++.so.6.0.21 by default, but
        # our toolchain is libstdc++.so.6.0.25).
        "-l:libstdc++.a",
        "-ldl",
        "-lpthread",
        "-lm",
    ],
    opt_compile_flags = [
        "-g0",
        "-O2",
        "-D_FORTIFY_SOURCE=1",
        "-DNDEBUG",
        "-ffunction-sections",
        "-fdata-sections",
    ],
    opt_link_flags = [
        "-Wl,--gc-sections",
    ],
    supports_start_end_lib = False,
    sysroot = PKG + LINUX_ROOT,
    target_libc = "linux",
    target_system_name = "linux",
    tool_paths = {
        "gcc": LLVM + "/bin/clang.exe",
        "ar": LLVM + "/bin/llvm-ar.exe",
        "ld": LLVM + "/bin/ld.lld.exe",
        "cpp": LLVM + "/bin/clang++.exe",
        "gcov": LLVM + "/bin/llvm-cov.exe",
        "nm": LLVM + "/bin/llvm-nm.exe",
        "objcopy": LLVM + "/bin/llvm-objcopy.exe",
        "objdump": LLVM + "/bin/llvm-objdump.exe",
        "strip": LLVM + "/bin/llvm-strip.exe",
    },
    toolchain_identifier = "linux_toolchain",
    unfiltered_compile_flags = [
        "-Wno-builtin-macro-redefined",
        "-D__DATE__=\"redacted\"",
        "-D__TIMESTAMP__=\"redacted\"",
        "-D__TIME__=\"redacted\"",
    ],
)

cc_toolchain(
    name = "cc_ubuntu_toolchain_impl",
    all_files = ":empty",
    compiler_files = ":empty",
    dwp_files = ":empty",
    features = None,
    linker_files = ":empty",
    objcopy_files = ":empty",
    strip_files = ":empty",
    supports_param_files = True,
    toolchain_config = ":linux_toolchain_config",
    toolchain_identifier = "linux-toolchain",
)

toolchain(
    name = "cc_ubuntu_toolchain",
    exec_compatible_with = [
        "@platforms//cpu:x86_64",
        "@platforms//os:windows",
    ],
    target_compatible_with = [
        "@platforms//cpu:x86_64",
        "@platforms//os:linux",
        "//platforms/linux_flavor:ubuntu",
    ],
    toolchain = ":cc_ubuntu_toolchain_impl",
    toolchain_type = "@rules_cc//cc:toolchain_type",
)

#### WASM TOOLCHAIN ####################################################

EMSDK_ROOT = "emsdk/1.39.20"

filegroup(
    name = "emsdl_all",
    srcs = glob(["emsdk/1.39.20/**"]),
    visibility = ["//visibility:public"],
)

wasm_cc_toolchain_config(
    name = "wasm_toolchain_config",
    python_env_path = PKG + EMSDK_ROOT + "/python/3.7.4-pywin32_64bit",
    tool_paths = {
        "gcc": EMSDK_ROOT + "/upstream/emscripten/emcc.bat",
        "ar": EMSDK_ROOT + "/upstream/emscripten/emar.bat",
        "ld": EMSDK_ROOT + "/upstream/emscripten/emcc.bat",
        "cpp": EMSDK_ROOT + "/upstream/emscripten/emcc.bat",
        "gcov": "NOT_USED",
        "nm": "NOT_USED",
        "objcopy": "NOT_USED",
        "objdump": "NOT_USED",
        "strip": "NOT_USED",
    },
)

cc_toolchain(
    name = "cc_wasm_toolchain_impl",
    all_files = ":emsdl_all",
    compiler_files = ":emsdl_all",
    dwp_files = ":empty",
    linker_files = ":emsdl_all",
    objcopy_files = ":empty",
    strip_files = ":empty",
    supports_param_files = 1,
    toolchain_config = ":wasm_toolchain_config",
    toolchain_identifier = "wasm-toolchain",
)

toolchain(
    name = "cc_wasm_toolchain",
    exec_compatible_with = [
        "@platforms//cpu:x86_64",
        "@platforms//os:windows",
    ],
    target_compatible_with = [
        "@platforms//cpu:wasm32",
    ],
    toolchain = ":cc_wasm_toolchain_impl",
    toolchain_type = "@rules_cc//cc:toolchain_type",
)

# Here to maintain compatibility with legacy flags
# We will decomission it when necessary possible
cc_toolchain_suite(
    name = "toolchains",
    toolchains = {
        "x64_windows|clang-cl": ":cc_windows_toolchain_impl",
        "x64_linux|llvm": ":cc_ubuntu_toolchain_impl",
    },
)

# Used by bindgen to generate rust FFI binding
sh_binary(
    name = "clang.exe",
    srcs = [
        LLVM + "/bin/clang.exe",
    ],
    data = glob([LLVM + "/bin/*.dll"]),
)

cc_import(
    name = "libclang.dll",
    shared_library = LLVM + "/bin/libclang.dll",
)

#### WINDOWS PYTHON TOOLCHAIN ##################################################
filegroup(
    name = "py3_windows_interpreter_files",
    srcs = glob(["python/3.8.5-embed-amd64/*"]),
)

py_runtime(
    name = "py3_windows_runtime",
    files = [":py3_windows_interpreter_files"],
    interpreter = "python/3.8.5-embed-amd64/python.exe",
    python_version = "PY3",
    visibility = ["//visibility:public"],
)

py_runtime_pair(
    name = "py_windows_runtime_pair",
    py2_runtime = None,
    py3_runtime = ":py3_windows_runtime",
)

toolchain(
    name = "py_windows_toolchain",
    toolchain = ":py_windows_runtime_pair",
    toolchain_type = "@rules_python//python:toolchain_type",
)

#### WINDOWS SDK TOOLCHAIN ##################################################
WINBIN = WINSDK + "/bin/10.0.18362.0/x64"

filegroup(
    name = "winsdk_dlls",
    srcs = glob(
        [
            WINBIN + "/*.dll",
        ],
    ),
)

winsdk_toolchain(
    name = "winsdk_toolchain_impl",
    env_include = ";".join([
        PKG + WININC + "/um",
        PKG + WININC + "/shared",
    ]),
    extra_files = ":winsdk_dlls",
    mc = WINBIN + "/mc.exe",
    rc = WINBIN + "/rc.exe",
)

toolchain(
    name = "winsdk_toolchain",
    exec_compatible_with = [
        "@platforms//os:windows",
        "@platforms//cpu:x86_64",
    ],
    target_compatible_with = [
        "@platforms//os:windows",
        "@platforms//cpu:x86_64",
    ],
    toolchain = ":winsdk_toolchain_impl",
    toolchain_type = WINSDK_TOOLCHAIN_TYPE,
    visibility = ["//visibility:public"],
)
