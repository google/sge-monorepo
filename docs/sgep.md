# sgep

`sgep` (SGE-Presubmit) runs presubmit checks to validate your code. It is invoked by the CI system,
but you can also run it locally.

## Usage

To run a local presubmit, invoke `sgep` without any arguments. This command inspects your current P4
state and runs all triggered presubmit checks from all your CLs.

To restrict the presubmit run to a single CL, pass `-c`:

```
sgep -c 1234
```

This will only trigger the checks relevant to that CL. The presubmit is still run locally so other
CLs could still affect the result.

### What do I do when a presubmit fails?

The check should print actionable information. For instance, if `gofmt` fails, a command will be
printed that will format the files.

In the event of a compilation error you can invoke `sgeb build/test` (see `README.sgeb`) on the
failing unit. Of course, if the failure is a broken Bazel target you may manually issue a `bazel`
command to help you iterate on fixing the problem.

### `sgep fix`

`sgep fix` runs all fixable checks and applies the resulting fixes.

At time of writing fixable checks includes the formatters (`buildifier`, `gofmt`, and `rustfmt`).

## Adding a presubmit check

There are three components to adding a check, with an additional step if you are adding a new type
of check. The components are `CICD` files, *presubmits*, and *presubmit checks*.

### Anatomy of a presubmit run

When you invoke `sgep`:

*   `sgep` analyzes your current P4 state using `p4 opened` to discover all modified files in _all_
    local CLs.

IMPORTANT: There isn't currently a way to run presubmit on only one local CL. When you send a CL for
review the presubmit is isolated to that CL.

*   The modified files incur a set of `CICD` files to be collected.

*   Presubmits inside the `CICD` files are matched against the CL. If a presubmit matches the CL its
    checks are collected.

*   The collected presubmit checks are executed.

### CICD files

`CICD` files contain presubmits and presubmit checks. The format is a text proto defined in
[`cicd.proto`](//build/cicd/cicdfile/protos/cicdfile.proto).

To add new presubmit checks, place a `CICD` file in the suitable directory. Any files in a CL whose
paths overlap that directory will cause `sgep` to examine that `CICD` file for matching presubmits.

### Presubmits

Presubmits are a combination of a match condition and one or more presubmit checks. The format is a
text proto added to the `CICD` file, defined in [`presubmit.proto`](//build/cicd/presubmit/protos/presubmit.proto).

For example, this `CICD` file runs `gofmt` on all `.go` files in a CL except in `third_party`:

```
presubmit {
  include: "....go"
  exclude: "third_party/..."
  check {
    action: "gofmt"
  }
}
```

And this `CICD` file tests a `sgeb` test unit whenever any file below the `CICD` file is in a CL:

```
presubmit {
  # Omitting 'include' is equivalent to 'match all files', or 'include: "..."'
  check_test {
    test_unit: "//foo:tests"
  }
}
```

### Presubmit checks

A presubmit check is one of `check`, `check_build`, or `check_test`.

#### `check_build`

`check_build` runs `sgeb build` on a [build unit](sgeb.md#build units) and verifies that it builds:

```
check_build {
  # Check the "lib" build unit from "foo/BUILDUNIT".
  build_unit: "//foo:lib"
}
```

For the most part, prefer converting a `check_build` into a test by wrapping your Bazel library in a
[`build_test`](//libs/bzl/build_test/build_test.bzl) rule invoked via a `check_test`.

#### `check_test`

`check_test` runs `sgeb test` on a [test unit](sgeb.md#test_units).

```
check_test {
  # Test the "tests" test unit from "foo/BUILDUNIT".
  test_unit: //foo:tests""
}
```

#### `check`

`check` invokes a checker tool defined by its `action`.

```
check {
  action: "gofmt"
}
```

#### Writing your own check.

Check actions are registered in [`tools.textpb`](//build/cicd/presubmit/checks/tools.textpb).

The registration maps the action name to a check tool, which can be either a prebuilt binary or a
build unit label, along with some arguments.

When `sgep` issues a check it calls the check tool binary with a `--checker-invocation` argument
pointing to an invocation proto with information about what the check needs to do. The invocation
proto is defined in [`check.proto`](//build/cicd/presubmit/check/protos/check.proto).

The check tool is expected to examine the CL (as defined by the invocation proto) and output one or
more check results (defined in the same proto). We encourage you to use the [`check`](//build/cicd/presubmit/check/check.go)
library to load the invocation proto and write the results.

An example check can be found at [`checkfmt`](//build/cicd/presubmit/checks/checkfmt/checkfmt.go).
This particular check runs a formatting tool on each matching file and outputs a check result per file.
