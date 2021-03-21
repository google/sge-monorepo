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

def pages(name, srcs, deps, visibility = None):
    outs = []
    for src in srcs:
        native.genrule(
            name = "%s_gen" % src,
            srcs = [src] + deps,
            outs = ["tmpl-%s" % src],
            cmd_bat = "$(location //tools/ebert/weaver) --input=$(location %s) --output=$@" % src,
            tools = ["//tools/ebert/weaver"],
        )
        outs = outs + ["tmpl-%s" % src]

    native.filegroup(
        name = name,
        srcs = outs,
        visibility = visibility,
    )
