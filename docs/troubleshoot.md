# Troubleshooting

## Bazel can't find / Gazelle won't import @com_google_protobuf

If you bring in a package that depends on @com_google_protobuf, Gazelle will not be able to process
the transitive dependencies correctly. The result is you'll get an error like this when you
`bazel [run|build]`:

```
ERROR: <PATH>/io_bazel_rules_go/proto/<TARGET>/BUILD.bazel: no such package '@com_google_protobuf//': The repository '@com_google_protobuf' could not be resolved and referenced by '@io_bazel_rules_go//proto/wkt:duration_go_proto'
```

This is [apparently working as intended](https://github.com/bazelbuild/bazel-gazelle/issues/591),
though it's not documented in gazelle, bazel, or the Google Cloud API docs.

TL;DR: Including the Protobufs package directly made it difficult for people to specify their
desired versions, so they pulled it out into its own repo, which you need to manually add.

The solution is to add this to your WORKSPACE:

```
load("@bazel_tools//tools/build_defs/repo:git.bzl", "git_repository")

git_repository(
   name = "com_google_protobuf",
   commit = "09745575a923640154bcf307fba8aedff47f240a",
   remote = "https://github.com/protocolbuffers/protobuf",
   shallow_since = "1558721209 -0700",
)

load("@com_google_protobuf//:protobuf_deps.bzl", "protobuf_deps")

protobuf_deps()
```

Then run `bazel run //:gazelle -- update-repos -from-file <yourGoModFileHere>` to pick up the changes.
