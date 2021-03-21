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

def _go_name_hack_impl(ctx):
    ctx.file("BUILD.bazel")
    ctx.file(
        "def.bzl", 
        content = "IS_RULES_GO = False",
    )

go_name_hack = repository_rule(
    implementation = _go_name_hack_impl,
    doc = """go_name_hack records whether the main workspace is rules_go.""",
)
