# This is an example WORKSPACE file to shows a basic example of how to we managed our several
# languages and third party depdendencies as external local repositories.

# Note how we don't use the http_archive functions, but rather point to repositories made
# outside of Bazel. This is because we used our own vendoring mechanism and tool which handled
# several aspects:
# - Metadata tracking (vendor date, licenses, etc.).
# - Go to Bazel (eg. Gazelle) logic. This is so we can vendor normal Go packages and use them
#   directly from Bazel.
# - Overall gave more flexibility for more unusual dependencies.
#
# See //tools/vendor_bender for more information about our vendoring tool.

workspace(name = "sge")

# This is a made up root variable to be able to point to packages outside of the repo.
# This is quality of life feature for the open sourcing process.
ROOT = ""


# Have a local repo for all the toolchains. That way they can be refered to as @toolchains.
local_repository(
    name = "toolchains",
    path = ROOT + "toolchains",
)

# Bazel rules --------------------------------------------------------------------------------------

local_repository(
    name = "bazel_skylib",
    path = ROOT + "third_party/bzl/bazel_skylib",
)

local_repository(
    name = "rules_python",
    path = ROOT + "third_party/bzl/rules_python",
)

local_repository(
    name = "rules_winsdk",
    path = ROOT + "third_party/bzl/rules_winsdk",
)

local_repository(
    name = "io_bazel",
    path = ROOT + "third_party/io_bazel",
)


# Third Party --------------------------------------------------------------------------------------

# https://github.com/google/flatbuffers.git"
local_repository(
    name = "flatbuffers",
    path = ROOT + "third_party/flatbuffers",
)

# Go -----------------------------------------------------------------------------------------------
#
# For Go we ended defining our own toolchains using the official SDKs. This is because we developed
# several cross-compiling scenarios and having control over the toolchain was key.

# https://github.com/bazelbuild/rules_go
local_repository(
    name = "io_bazel_rules_go",
    path = ROOT + "third_party/bzl/io_bazel_rules_go",
)

load("@io_bazel_rules_go//extras:embed_data_deps.bzl", "go_embed_data_dependencies")
go_embed_data_dependencies()

load("//build/deps:go.bzl", "go_name_hack")
go_name_hack(name = "io_bazel_rules_go_name_hack",)

load("@io_bazel_rules_go//go/private:nogo.bzl", "DEFAULT_NOGO", "go_register_nogo")
go_register_nogo(
    name = "io_bazel_rules_nogo",
    nogo = DEFAULT_NOGO,
)

# windows: MSVC based toolchain.
# windows-gnu: MinGW based toolchain.
register_toolchains("@toolchains//go/1.16.2:go_windows_toolchain")
register_toolchains("@toolchains//go/1.16.2:go_windows_gnu_toolchain")
register_toolchains("@toolchains//go/1.14.3:go_windows_toolchain")
register_toolchains("@toolchains//go/1.14.3:go_windows_gnu_toolchain")


local_repository(
	name = "go_googleapis",
	path = ROOT + "third_party/go_googleapis",
)
load("@go_googleapis//:repository_rules.bzl", "switched_rules_by_language")

# Any new language that wants access to googleapis must add those
# languages to as keyword arguments to this repository rule.
switched_rules_by_language(
    name = "com_google_googleapis_imports",
    go = True,
)

# Go dependencies.
# These are not all the dependencies stored in the monorepo, but are all the ones needed to build
# the code that was released.

# gazelle:repository go_repository name=com_github_golang_glog importpath=github.com/golang/glog
local_repository(
	name = "com_github_golang_glog",
	path = ROOT + "third_party/go/com_github_golang_glog",
)

# gazelle:repository go_repository name=com_github_nu7hatch_gouuid importpath=github.com/nu7hatch/gouuid
local_repository(
    name = "com_github_nu7hatch_gouuid",
    path = ROOT + "third_party/go/com_github_nu7hatch_gouuid",
)

# gazelle:repository go_repository name=com_google_cloud_go importpath=cloud.google.com/go
local_repository(
    name = "com_google_cloud_go",
    path = ROOT + "third_party/go/com_google_cloud_go",
)

# gazelle:repository go_repository name=com_github_julvo_htmlgo importpath=github.com/julvo/htmlgo
local_repository(
    name = "com_github_julvo_htmlgo",
    path = ROOT + "third_party/go/com_github_julvo_htmlgo",
)

# gazelle:repository go_repository name=com_google_cloud_go_logging importpath=cloud.google.com/go/logging
local_repository(
	name = "com_google_cloud_go_logging",
	path = ROOT + "third_party/go/com_google_cloud_go_logging",
)

# gazelle:repository go_repository name=com_google_cloud_go_spanner importpath=cloud.google.com/go/spanner
local_repository(
    name = "com_google_cloud_go_spanner",
    path = ROOT + "third_party/go/com_google_cloud_go_spanner",
)

# gazelle:repository go_repository name=com_google_cloud_go_datastore importpath=cloud.google.com/go/datastore
local_repository(
    name = "com_google_cloud_go_datastore",
    path = ROOT + "third_party/go/com_google_cloud_go_datastore",
)

# gazelle:repository go_repository name=org_golang_x_sys importpath=golang.org/x/sys
local_repository(
    name = "org_golang_x_sys",
    path = ROOT + "third_party/go/org_golang_x_sys",
)

# gazelle:repository go_repository name=org_golang_google_protobuf importpath=google.golang.org/protobuf
local_repository(
    name = "org_golang_google_protobuf",
    path = ROOT + "third_party/go/org_golang_google_protobuf",
)

# gazelle:repository go_repository name=bazel_gazelle importpath=github.com/bazelbuild/bazel-gazelle
local_repository(
    name = "bazel_gazelle",
    path = ROOT + "third_party/go/bazel_gazelle",
)

# gazelle:repository go_repository name=org_golang_google_api importpath=google.golang.org/api
local_repository(
    name = "org_golang_google_api",
    path = ROOT + "third_party/go/org_golang_google_api",
)

# gazelle:repository go_repository name=org_golang_x_xerrors importpath=golang.org/x/xerrors
local_repository(
    name = "org_golang_x_xerrors",
    path = ROOT + "third_party/go/org_golang_x_xerrors",
)

# gazelle:repository go_repository name=com_github_google_go_containerregistry importpath=github.com/google/go-containerregistry
local_repository(
    name = "com_github_google_go_containerregistry",
    path = ROOT + "third_party/go/com_github_google_go_containerregistry",
)

# gazelle:repository go_repository name=com_github_pkg_errors importpath=github.com/pkg/errors
local_repository(
    name = "com_github_pkg_errors",
    path = ROOT + "third_party/go/com_github_pkg_errors",
)

# gazelle:repository go_repository name=com_github_docker_cli importpath=github.com/docker/cli
local_repository(
    name = "com_github_docker_cli",
    path = ROOT + "third_party/go/com_github_docker_cli",
)

# gazelle:repository go_repository name=com_github_docker_docker importpath=github.com/docker/docker
local_repository(
    name = "com_github_docker_docker",
    path = ROOT + "third_party/go/com_github_docker_docker",
)

# gazelle:repository go_repository name=com_github_opencontainers_runc importpath=github.com/opencontainers/run
local_repository(
    name = "com_github_opencontainers_runc",
    path = ROOT + "third_party/go/com_github_opencontainers_runc",
)

# gazelle:repository go_repository name=com_github_docker_docker_credential_helpers importpath=github.com/docker/docker-credential-helpers
local_repository(
    name = "com_github_docker_docker_credential_helpers",
    path = ROOT + "third_party/go/com_github_docker_docker_credential_helpers",
)

# gazelle:repository go_repository name=org_golang_x_oauth2 importpath=golang.org/x/oauth2
local_repository(
    name = "org_golang_x_oauth2",
    path = ROOT + "third_party/go/org_golang_x_oauth2",
)

# gazelle:repository go_repository name=com_google_cloud_go_storage importpath=cloud.google.com/go/storage
local_repository(
    name = "com_google_cloud_go_storage",
    path = ROOT + "third_party/go/com_google_cloud_go_storage",
)

# gazelle:repository go_repository name=org_golang_x_tools importpath=golang.org/x/tools
local_repository(
    name = "org_golang_x_tools",
    path = ROOT + "third_party/go/org_golang_x_tools",
)

# gazelle:repository go_repository name=com_github_google_go_cmp importpath=github.com/google/go-cmp
local_repository(
    name = "com_github_google_go_cmp",
    path = ROOT + "third_party/go/com_github_google_go_cmp",
)

# gazelle:repository go_repository name=org_golang_x_net importpath=golang.org/x/net
local_repository(
    name = "org_golang_x_net",
    path = ROOT + "third_party/go/org_golang_x_net",
)

# gazelle:repository go_repository name=com_github_allendang_giu importpath=github.com/AllenDang/giu
local_repository(
    name = "com_github_allendang_giu",
    path = ROOT + "third_party/go/com_github_allendang_giu",
)

# gazelle:repository go_repository name=com_github_golang_protobuf importpath=github.com/golang/protobuf
local_repository(
    name = "com_github_golang_protobuf",
    path = ROOT + "third_party/go/com_github_golang_protobuf",
)

# gazelle:repository go_repository name=com_github_go_gl_gl importpath=github.com/go-gl/gl
local_repository(
    name = "com_github_go_gl_gl",
    path = ROOT + "third_party/go/com_github_go_gl_gl",
)

# gazelle:repository go_repository name=org_golang_google_grpc importpath=google.golang.org/grpc
local_repository(
    name = "org_golang_google_grpc",
    path = ROOT + "third_party/go/org_golang_google_grpc",
)

# gazelle:repository go_repository name=com_github_go_gl_glfw_v3_3_glfw importpath=github.com/go-gl/glfw/v3.3/glfw
local_repository(
    name = "com_github_go_gl_glfw_v3_3_glfw",
    path = ROOT + "third_party/go/com_github_go_gl_glfw_v3_3_glfw",
)

# gazelle:repository go_repository name=org_golang_x_sync importpath=golang.org/x/sync
local_repository(
    name = "org_golang_x_sync",
    path = ROOT + "third_party/go/org_golang_x_sync",
)

# gazelle:repository go_repository name=org_golang_x_text importpath=golang.org/x/text
local_repository(
    name = "org_golang_x_text",
    path = ROOT + "third_party/go/org_golang_x_text",
)

# gazelle:repository go_repository name=com_github_golang_groupcache importpath=github.com/golang/groupcache
local_repository(
    name = "com_github_golang_groupcache",
    path = ROOT + "third_party/go/com_github_golang_groupcache",
)

# gazelle:repository go_repository name=com_github_googleapis_gax_go_v2 importpath=github.com/googleapis/gax-go/v2
local_repository(
    name = "com_github_googleapis_gax_go_v2",
    path = ROOT + "third_party/go/com_github_googleapis_gax_go_v2",
)

# gazelle:repository go_repository name=io_opencensus_go importpath=go.opencensus.io
local_repository(
    name = "io_opencensus_go",
    path = ROOT + "third_party/go/io_opencensus_go",
)

# gazelle:repository go_repository name=com_github_hashicorp_go_version importpath=github.com/hashicorp/go-version
local_repository(
    name = "com_github_hashicorp_go_version",
    path = ROOT + "third_party/go/com_github_hashicorp_go_version",
)

# gazelle:repository go_repository name=com_github_mb0_glob importpath=github.com/mb0/glob
local_repository(
    name = "com_github_mb0_glob",
    path = ROOT + "third_party/go/com_github_mb0_glob",
)

# gazelle:repository go_repository name=com_github_atotto_clipboard importpath=github.com/atotto/clipboard
local_repository(
    name = "com_github_atotto_clipboard",
    path = ROOT + "third_party/go/com_github_atotto_clipboard",
)

# gazelle:repository go_repository name=com_github_burntsushi_toml importpath=github.com/BurntSushi/toml
local_repository(
    name = "com_github_burntsushi_toml",
    path = ROOT + "third_party/go/com_github_burntsushi_toml",
)

# gazelle:repository go_repository name=io_opencensus_go_contrib_exporter_stackdriver importpath=contrib.go.opencensus.io/exporter/stackdriver
local_repository(
    name = "io_opencensus_go_contrib_exporter_stackdriver",
    path = ROOT + "third_party/go/io_opencensus_go_contrib_exporter_stackdriver",
)

# gazelle:repository go_repository name=com_github_census_instrumentation_opencensus_proto importpath=github.com/census-instrumentation/opencensus-proto
local_repository(
    name = "com_github_census_instrumentation_opencensus_proto",
    path = ROOT + "third_party/go/com_github_census_instrumentation_opencensus_proto",
)

# gazelle:repository go_repository name=com_github_aws_aws_sdk_go importpath=github.com/aws/aws-sdk-go
local_repository(
    name = "com_github_aws_aws_sdk_go",
    path = ROOT + "third_party/go/com_github_aws_aws_sdk_go",
)

# gazelle:repository go_repository name=com_github_jmespath_go_jmespath importpath=github.com/jmespath/go-jmespath
local_repository(
    name = "com_github_jmespath_go_jmespath",
    path = ROOT + "third_party/go/com_github_jmespath_go_jmespath",
)

# gazelle:repository go_repository name=com_github_go_resty_resty_v2 importpath=github.com/go-resty/resty/v2
local_repository(
    name = "com_github_go_resty_resty_v2",
    path = ROOT + "third_party/go/com_github_go_resty_resty_v2",
)

# gazelle:repository go_repository name=org_golang_google_genproto importpath=google.golang.org/genproto
local_repository(
    name = "org_golang_google_genproto",
    path = ROOT + "third_party/go/org_golang_google_genproto",
)

# CC -----------------------------------------------------------------------------------------------

register_toolchains("@toolchains//:cc_windows_toolchain")
register_toolchains("@toolchains//:cc_windows_gnu_toolchain")

# https://github.com/google/boringssl
local_repository(
    name = "boringssl",
    path = ROOT + "third_party/cc/boringssl",
)

# https://github.com/google/protobuf
local_repository(
    name = "com_google_protobuf",
    path = ROOT + "third_party/cc/com_google_protobuf",
)

# https://github.com/microsoft/DirectXShaderCompiler/releases/download/v1.5.2003/dxc_2020_03-25.zip
local_repository(
    name = "dxc",
    path = ROOT + "third_party/cc/dxc",
)

# https://swarm.workshop.perforce.com/projects/perforce_software-p4/archives/2018-2.zip
local_repository(
    name = "p4api",
    path = ROOT + "third_party/cc/p4api",
)

local_repository(
    name = "vulkan",
    path = ROOT + "third_party/cc/vulkan",
)

# Rust ---------------------------------------------------------------------------------------------

local_repository(
    name = "io_bazel_rules_rust",
    path = ROOT + "third_party/bzl/io_bazel_rules_rust",
)

load("@io_bazel_rules_rust//:workspace.bzl", "bazel_version")
bazel_version(name = "bazel_version",)

register_toolchains("@toolchains//rust/1.48.0:rust_windows_toolchain")

# Docker -------------------------------------------------------------------------------------------

local_repository(
    name = "io_bazel_rules_docker",
    path = ROOT + "third_party/bzl/io_bazel_rules_docker",
)

register_toolchains(
    "@io_bazel_rules_docker//toolchains/docker:default_windows_toolchain",
)

load(
    "@io_bazel_rules_docker//toolchains/docker:toolchain.bzl",
    _docker_toolchain_configure = "toolchain_configure",
)

_docker_toolchain_configure(
    name = "docker_config",
    # %BUILD_WORKSPACE_DIRECTORY% returns the absolute path to the root.
    # It is required because the client_config path must be absolute:
    # https://github.com/bazelbuild/rules_docker/blob/master/README.md
    # The value is passed to the pusher binary.
    client_config = "%BUILD_WORKSPACE_DIRECTORY%/environment/docker",
)
