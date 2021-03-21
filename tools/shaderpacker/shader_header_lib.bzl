# Copyright 2021 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

load("@rules_cc//cc:defs.bzl", "cc_library")

def _shader_header_library_impl(ctx):
    outputs = []
    for f in ctx.files.srcs:
        out = ctx.actions.declare_file(f.basename.split(".")[0] + "_generated.h")
        args = ctx.actions.args()
        args.add("-o", out.dirname)
        args.add("--cpp", f)
        ctx.actions.run(
            executable = ctx.executable._flatc,
            inputs = [f],
            outputs = [out],
            arguments = [args],
        )
        outputs.append(out)
    return [DefaultInfo(files = depset(outputs))]

_shader_header_library = rule(
    implementation = _shader_header_library_impl,
    attrs = {
        "srcs": attr.label_list(
            mandatory = True,
            allow_files = True,
        ),
        "_flatc": attr.label(
            default = "@flatbuffers//:flatc",
            executable = True,
            allow_single_file = True,
            cfg = "exec",
        ),
    },
)

def shader_header_library(name, srcs, visibility, **kwargs):
    _shader_header_library(
        name = "_" + name,
        srcs = srcs,
        visibility = visibility,
    )

    cc_library(
        name = name,
        hdrs = [
            "_" + name,
        ],
        deps = [
            "@flatbuffers//:runtime_cc",
        ],
        visibility = visibility,
    )
