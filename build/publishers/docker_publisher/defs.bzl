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

load("@io_bazel_rules_docker//container:providers.bzl", "ImageInfo", "ImportInfo")

def _impl(ctx):
    image = None
    if ImageInfo in ctx.attr.image:
        image = ctx.attr.image[ImageInfo].container_parts
    elif ImportInfo in ctx.attr.image:
        image = ctx.attr.image[ImportInfo].container_parts
    else:
        # This can't happen because of the "providers" constraint on the attribute (see below).
        fail("could not find matching provider")

    layers = []

    compressed_layers = image.get("zipped_layer", [])
    uncompressed_layers = image.get("unzipped_layer", [])
    digest_files = image.get("blobsum", [])
    diff_id_files = image.get("diff_id", [])
    inputs = [image["config"]]
    inputs.extend(compressed_layers)
    inputs.extend(uncompressed_layers)
    inputs.extend(digest_files)
    inputs.extend(diff_id_files)
    for i, compressed_layer in enumerate(compressed_layers):
        inputs.append(compressed_layer)
        uncompressed_layer = uncompressed_layers[i]
        digest_file = digest_files[i]
        diff_id_file = diff_id_files[i]
        layers.append(
            "{},{},{},{}".format(
                compressed_layer.path,
                uncompressed_layer.path,
                digest_file.path,
                diff_id_file.path,
            ),
        )
    toolchain_info = ctx.toolchains["@io_bazel_rules_docker//toolchains/docker:toolchain_type"].info
    proto = struct(
        config = image["config"].path,
        client_config_dir = str(toolchain_info.client_config),
        manifest = image["manifest"].path,
        push_tool = ctx.executable._pusher.path,
        layers = layers,
    )
    output = ctx.actions.declare_file(ctx.label.name + ".pushconfig.textpb")
    ctx.actions.write(
        output = output,
        content = proto.to_proto(),
    )

    inputs.append(ctx.executable._pusher)

    return [
        DefaultInfo(files = depset([output] + inputs)),
    ]

# docker_push_config is a rule that transforms information from the docker rules
# into a format that is consumable by docker_publisher.
docker_push_config = rule(
    attrs = {
        "image": attr.label(
            doc = "the docker image, either a container_image or container_import target",
            mandatory = True,
            providers = [[ImageInfo], [ImportInfo]],
        ),
        "_pusher": attr.label(
            default = "@io_bazel_rules_docker//container/go/cmd/pusher",
            cfg = "host",
            executable = True,
            allow_files = True,
        ),
    },
    toolchains = ["@io_bazel_rules_docker//toolchains/docker:toolchain_type"],
    implementation = _impl,
)
