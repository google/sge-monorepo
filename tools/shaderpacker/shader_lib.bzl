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

def _shader_library_impl(ctx):
    outputs = []
    for f in ctx.files.srcs:
        dst_file = ctx.label.name + "/" + f.basename.replace("." + f.extension, ".shader_pkg")
        out = ctx.actions.declare_file(dst_file)
        args = ctx.actions.args()
        args.add("-T", out)
        args.add(f)
        ctx.actions.run(
            executable = ctx.executable._compiler,
            inputs = [f],
            outputs = [out],
            arguments = [args],
        )
        outputs.append(out)
    return [DefaultInfo(files = depset(outputs))]

shader_library = rule(
    implementation = _shader_library_impl,
    attrs = {
        "srcs": attr.label_list(
            mandatory = True,
            allow_files = True,
        ),
        "_compiler": attr.label(
            default = "//tools/shaderpacker",
            executable = True,
            allow_single_file = True,
            cfg = "exec",
        ),
    },
)
