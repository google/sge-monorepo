# sgeb

`sgeb` (SGE-Build) is SGE's primary build tool. It provides a unified entry point for building and
testing Bazel and non-Bazel code, and is used under the hood by the presubmit system.

## Build Units

The subject of a `sgeb build` operation is a build unit. These are defined in `BUILDUNIT` files,
somewhat similar to `BUILD` files. The format is a text proto whose format is defined in
[`sgeb.proto`](//build/cicd/sgeb/protos/sgeb.proto).

There are two main types of build units. The first type invokes Bazel with a target and some
user-defined arguments:

```
build_unit {
  name: "foo_binary"
  target: "//foo:foo"
  args: "--config=windows"
}
```

The second type invokes custom build tools defined by the bin field and some user-defined arguments:

```
build_unit {
  name: "editor"
  # bin can be a checked-in binary or another sgeb build unit
  bin: "//build/unreal-builder"
  args: "--platform=Win64"
  args: "build"
  args: "editor"
}
```

To build either kind of build unit, invoke `sgeb build`:

```
sgeb build //label/of/build:unit
```

The resulting build artifacts are output to the console.

TIP: Same as Bazel, `sgeb build //my/app` is shorthand for `sgeb build //my/app:app`.

### Writing a custom build tool

When you invoke `sgeb build` `sgeb` will:

1.  Build all the dependencies of the build unit indicated by `deps`.
1.  Pass the result to the binary indicated by `bin` (which can itself be a build unit).

The inputs are passed to the tool via an invocation proto defined in
[`build.proto`](//build/cicd/sgeb/protos/build.proto). The proto is passed to the tool via the
`--tool-invocation` argument. You can use the [`buildtool`](//build/cicd/sgeb/buildtool/buildtool.go)
library to easily load the invocation proto.

The result must be written is a `BuildinvocationResult` proto also defined in
[`build.proto`](//build/cicd/sgeb/protos/build.proto). The tool must write the result to the
location indicated by `--tool-invocation-result`. Again, the `buildtool` library provides
convencience functions for this.

The exit code from the binary is interpreted by `sgeb` as success/failure.

## Test Units

The subject of a `sgeb test` operation is a test unit. These are also defined in `BUILDUNIT` files.

```
test_unit {
  name: "foo_tests"
  target: "//foo/..."
  args: "--config=windows"
}
```

To run a test:

```
sgeb test //label/of/test:unit
```

TIP: `sgeb test //my/app` is shorthand for `sgeb test //my/app:test`.

TIP: You can use `...` to run all test units recursively, much like Bazel.

NOTE: Be warned that invoking `sgeb test //...` will take a while by virtue of `sgeb` needing to
scan the entire directory structure.

### Test suites

It can be convenient express "run these tests as one unit":

```
test_suite{
  name: "foo_tests"
  test_unit: ":foo_test_one"
  test_unit: ":foo_test_two"
  ...
}
```

Or, you may want to run all tests recursively under some directory. This can be particularly useful
when combined with `sgep` to say "when these files change, run all tests in this other directory".

```
test_suite{
  name: "tests"
  test_unit: "..."
}
```

To run a test suite, invoke `sgeb test`:

```
sgeb test //foo:tests
```

## `sgeb` run

`sgeb run` builds a build unit with a single executable output and runs it. The working directory is
the same one that you invoke `sgeb` from. Any arguments after the build unit are passed to the
executable.

```
sgeb run //my/build/unit --some_option
```

## Publish Units

A publish unit is the combination of a `sgeb` build unit with a user-supplied binary that knows how
to publish the build result. Publish units too are contained in `BUILDUNIT` files.

Note that publish units do not generate any artifacts themselves. Their role is to generate changes
to the target being published on (eg. submit a Perforce CL, store at a GCS bucket).

In the following example we add a unit to publish `sgeb` itself:

```
publish_unit {
  name: "publish"
  build_unit: "//build/cicd/sgeb"
  bin: "//build/cicd/cl_publisher"
  args: "-name=sgeb"
  args: "-out_dir=bin/windows"
}
```

`//build/cicd/cl_publisher` is a publisher that knows how to create a CL. The publish unit above
would create a CL with the result of the `//build/cicd/sgeb:sgeb` build unit and store it at
//bin/windows.

In this example, to publish `sgeb` we invoke:

```
sgeb publish //build/cicd/sgeb:publish
```

This will create a CL with `sgeb` published to `bin/windows`.

TIP: `sgeb publish //my/app` is shorthand for `sgeb publish //my/app:publish`.

Any arguments after the publish unit are passed along to the publishing binary:

```
sgeb publish //build/cicd/sgeb:publish -submit_cl
```

Each publisher defines its own set of flags and arguments. In this example, `-submit_cl` means that
the invocation will not only create the CL, but also submit it.

### Auto-publish

A CICD machine continously syncs the depot to HEAD, discovers all the `auto_publish`-enabled publish
units, and runs `sgeb publish` on them.

To enable automatic publication of your publish units, add an `auto_publish` section:

```
publish_unit {
  name: "publish"
  build_unit: "//build/cicd/sgeb"
  bin: "//build/cicd/cl_publisher"
  args: "-name=sgeb"
  args: "-out_dir=bin/windows"

  # Turn on automatic publication
  auto_publish {
    # Extra argument to pass to the publishing binary.
    # In this case we want to submit the CL, not just create it.
    args: "-submit_cl"

    # Notification recipients when/if the build becomes unhealthy.
    notify {
      email: "group+notifications@someemail.com"
    }
  }
}
```

#### Health notifications

When a publish unit becomes unhealthy all email recipients are notified via email. If the build
isn't fixed, the system keeps sending rate limited notifications until it is restored to health.

### Writing a publishing binary

When you invoke `sgeb publish` `sgeb` will:

1.  Build the build unit(s) indicated by `build_unit`.
1.  Pass the result to the publish tool indicated by `bin` (which can itself be a build unit).

The build result is passed to the tool via an invocation proto defined in
[`build.proto`](//build/cicd/sgeb/protos/build.proto). The proto is passed to the tool via the
`--tool-invocation` argument. You can use the [`buildtool`](//build/cicd/sgeb/buildtool/buildtool.go)
library to easily load the invocation proto.

See [`cl_publisher`](//build/cicd/cl_publisher/cl_publisher.go) for an example.

`sgeb` will print the output from the publish binary to the console. The exit code from the binary
is interpreted by `sgeb` as success/failure.

NOTE: It is the responsibility of the publishing binary to perform no action when nothing has
changed. This is required.

### Postsubmit

A CICD machine continously syncs the depot to HEAD, discovers all the `post_submit`-enabled test and
publish units, and runs `sgeb test/publish` on them if the postsubmit triggers are met.

To enable postsubmit for your test/publish units, add an `post_submit` section.

```
publish_unit {
  name: "publish"
  build_unit: "//game:publish"
  bin: "//build/publishers:game_publisher"
  ...

  # Turn on post submit for daily builds
  post_submit {
    # Notification recipients For build results.
    notify {
      email: "group+notifications@someemail.com"
    }
    frequency {
      daily: true
    }
  }
}
```

Postsubmits have *triggers* that have to be met for the post submit action to happen. Triggers can
be combined where appropriate.

There are currently two types of triggers:

#### `frequency`

The postsubmit is run at some particular frequency. Currently these frequencies are supported:

**Daily**: Postsubmits are triggered once per day at the specified UTC time using 24 hour time
`HH:MM`. The minute part is merely for readability and is ignored.

**Example:**

```
post_submit {
  frequency {
    daily_at_utc: "05:00"
  }
}
```

#### `trigger_paths`

The postsubmit is run whenever a CL is submitted that matches one of any number of paths.

**Example:**

```
post_submit {
  trigger_paths {
    path: "//mygame/..."
    path: "-//mygame/donottrigger/..."
  }
}
```

### Cron Units

Cron units are jobs that run at a specified frequency with certain arguments.

They are specified in `BUILDUNIT` files using [`cron_unit`](//build/cicd/sgeb/protos/sgeb.proto?CL=135242&L=172).

**Example:**

```
cron_unit {
  name: "test_sge_cleanup"
  bin: "//build/publishers/cleanup"
  args: "-some_sdk=//third_party/toolchains/some_sdk/1.12"
  args: "-archive_older_than_days=30"
  config {
    frequency_minutes: 60
    notify {
      email: "sge-failure-test-sge-cleanup@someemail.com"
    }
  }
}
```

Any cron unit can be run manually using `sgeb cron`:

```
sgeb cron foo/bar:test_sge_cleanup
```

If you also add the `config` section, it will be automatically picked up by our cron job runner and
executed at the specified frequency.

If a cron unit fails to run, an email will be sent to all entries in its corresponding `notify`
section.

#### Writing a cron job

You specify the binary to run using the `bin` argument. This must be a `sgeb` `build_unit`, which
can be a regular Bazel target.

An invocation proto is passed to the cron binary via the `--tool-invocation` argument. The protocol
buffer is defined in [build.proto](//build/cicd/sgeb/protos/build.proto?CL=136376&L=11A.).
You can use the [`buildtool`](//build/cicd/sgeb/buildtool/buildtool.go?CL=136376&L=1) library to
easily load proto for use by the binary.

The invocation proto can be used to resolve monorepo paths (such as the `some_sdk` above). If your
cron job does not need use of the invocation proto it does not need to use the `buildtool` helper.
