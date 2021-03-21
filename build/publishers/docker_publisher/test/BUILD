# TODO: Activate when there vendored in docker story is working again.
#load("//libs/bzl/build_test:build_test.bzl", "build_test")
#load("@io_bazel_rules_docker//container:container.bzl", "container_image")
#load("@io_bazel_rules_docker//container:import.bzl", "container_import")
#load("//build/publishers/docker_publisher:defs.bzl", "docker_push_config")
#
#container_import(
#    name = "ubuntu",
#    config = "testdata/ubuntu/config.json",
#    layers = glob(["testdata/ubuntu/*.tar.gz"]),
#)
#
#container_image(
#    name = "image",
#    base = ":ubuntu",
#    directory = "image_files",
#    files = ["testdata/sge.txt"],
#)
#
## Produces a docker push config suitable for docker_publisher.
#docker_push_config(
#    name = "push_config",
#    image = ":image",
#)
#
#build_test(
#    name = "push_config_build_test",
#    targets = [":push_config"],
#)
