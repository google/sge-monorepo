# Windows ABIs definitions.

package(default_visibility = ["//visibility:public"])

constraint_setting(name = "windows_abi")

# MSVC abi is the default Windows ABI output by msvc and
# when using llvm with -target=x86_64-pc-windows-msvc
constraint_value(
    name = "msvc",
    constraint_setting = ":windows_abi",
)

# The GNU abi is targetted by MinGW64 and by the llvm
# toolchain when using -target=x86_64-pc-windows-gnu
constraint_value(
    name = "gnu",
    constraint_setting = ":windows_abi",
)
