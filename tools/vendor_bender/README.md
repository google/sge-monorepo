# vendor_bender

Package vendoring system

## OVERVIEW

vendor_bender helps users to vendor third party packages into our monorepo and integrate with our
bazel build system. You inform the tool about the type and version of the package you want to
vendor and vendor_bender will download the package, rename, and set up the appropriate bazel build
files for you.

## VERSIONING

vendor_bender is optimised for usage in a monorepo where everything is at head, and will strip
versioning numbers from folder names.

## LANGUAGE SUPPORT

vendor_bender has a native understanding of Go and Rust packages, and can create the relevants bazel
build files for them. Packages in other languages are supported as raw imports, and it is up to the
user to manually create the necessary BUILD files to get them compiling.

Go and Rust will vendor a single specified package. That is, vendoring module `foo` will not vendor
`foo`'s dependencies.

## USAGE

Run `vendor_bender` to see the up to date command line usage.

vendor_bender uses manifest files to list all the vendored dependencies. These files are called
`MANIFEST` (or `MANIFEST.textpb`). There can be multiple MANIFEST files within the monorepo, each listing the vendor packages in that directory.

Adding an entry will add the package, removing an entry will delete the corresponding package, and
updating it's properties will update the package.

The manifest format requires users to be precise when setting the version of the packages to be
vendored. In order to not burden the user with knowing the exact version corresponding to an alias
(eg. "latest"), vendor bender provides some helpers:
* `version` for Go packages can be set to `latest`, vender_bender is going to display the precise
version equivalent to be set in the file.
* `commit` in Git packages can be set to a tag or branch. In that case vendor_bender is going to
display the equivalent sha1 to be set in the file.

## INDEX

### Internals:

*   `github_pkg.go` : functions for vendoring raw github packages (mainly for C++ packages).
*   `vendor_bender.go` : binary entry point and main function.

*   `golang/gazelle.go` : gazelle functionality.
*   `golang/gazelle_analyze.go` : gazelle analyze functionality.
*   `golang/go_pkg.go` : functions for vendoring go packages.

*   `rust/rust_pkg.go` : functions for vendoring rust packages.

*   `bazel/bazel.go` : functions to manipulate WORKSPACEs.
