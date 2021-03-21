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

def _impl(ctx):
    inputs = depset(transitive = [t.files for t in ctx.attr.targets])
    out = ctx.actions.declare_file(ctx.attr.name + "_testbin.exe")

    # This runs the dummy testbin app which copies itself
    # into the 'out' location. Next, that copy is run
    # without the -out argument when the test executes,
    # and simply returns 0 immediately.
    # Note that we have build_test's targets as inputs
    # to the action even though they are not used, which
    # causes the test to fail at the build stage if any
    # of those dependencies are broken.
    args = ctx.actions.args()
    args.add("-out", out)
    ctx.actions.run(
        executable = ctx.executable._bin,
        arguments = [args],
        inputs = inputs,
        outputs = [out],
    )
    return [DefaultInfo(
        files = depset([out]),
        executable = out,
    )]

build_test = rule(
    implementation = _impl,
    test = True,
    attrs = {
        "targets": attr.label_list(),
        "_bin": attr.label(
            default = "//libs/bzl/build_test:testbin",
            allow_single_file = True,
            executable = True,
            cfg = "host",
        ),
    },
)
