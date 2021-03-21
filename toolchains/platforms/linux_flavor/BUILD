# Platform definitions.

package(default_visibility = ["//visibility:public"])

# This constraint is used to differentiate between diferent flavors of linux
# It is important that @platforms//os:linux is defined in the case of
constraint_setting(name = "linux_flavor")

# Using ubuntu as a distribution currently
constraint_value(
    name = "ubuntu",
    constraint_setting = ":linux_flavor",
)
