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

load("@aspect_msbuild//:msbuild.bzl", "MSBuildProvider", "msbuild_aspect")

def _impl(ctx):
    outputs = []
    msbuild_inputs = []
    args = ctx.actions.args()

    # 1 project file and 1 filter file will be generte per dependency.
    for dep in ctx.attr.deps:
        msbuilt_input = dep[MSBuildProvider].msbuild_output
        msbuild_inputs.append(msbuilt_input)
        args.add("-input_msbuild", msbuilt_input)
        outputs.append(ctx.actions.declare_file(dep.label.name + ".vcxproj"))
        outputs.append(ctx.actions.declare_file(dep.label.name + ".vcxproj.filters"))

    out_solution = ctx.actions.declare_file(ctx.label.name + ".sln")
    args.add("-workspace_dir", ctx.attr.workspace_dir)
    args.add("-output_dir", out_solution.dirname)
    args.add("-solution_name", ctx.label.name)
    outputs.append(out_solution)

    ctx.actions.run(
        outputs = outputs,
        inputs = msbuild_inputs,
        arguments = [args],
        executable = ctx.executable._bazel2vs_tool,
    )

    return [DefaultInfo(files = depset(outputs))]

msbuild = rule(
    implementation = _impl,
    attrs = {
        "deps": attr.label_list(aspects = [msbuild_aspect]),
        "workspace_dir": attr.string(doc = "Directory of the root of the bazel workspace. Necessary to build from VS.", mandatory = True),
        "_bazel2vs_tool": attr.label(
            executable = True,
            cfg = "host",
            default = Label("//tools/bazel2vs:bazel2vs"),
        ),
    },
)
