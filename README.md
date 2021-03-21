# SG&E Monorepo

This repository contains the open sourcing of the infrastructure developed by Stadia Games &
Entertainment (SG&E) to run its operations. This entails part of the build system setup, the CICD
system and a number of tools developed for internal use, some experimental in nature, some saw more
widespread use. No game projects or game-related technologies are present in this repository.

This system is not being worked on anymore, so it will not have any support. We are open sourcing
for contribution purposes mostly. Note that the system also has limited documentation.

This is not an officially supported Google product.

NOTE: This is not a working system as it is published here. Several key setup pieces, like the Bazel
setup, the toolchains, the vendored dependencies are not present. It is likely to be a non-trivial
amount of work to get it up and running again.

## Requirements

SG&E was running on a custom environment that was different from normal Google operations. The
reasons for these were various, but a big driver was to have the ability to tailor the infra to the
specific needs of making video games.

Overall we strived to maintain the feel and good practices of Google's own tooling, which informed
the kind of tooling and design paradigms we chose. In that vein, we determined the following
requirements for our infrastructure:

- Windows based: game developers, especially non-programmers, heavily rely on windows based tooling,
  so it makes sense to natively support that platform.

- Monorepo: We determined that the benefits in maintenance and verifyability outweighed the costs of
  a monorepo, so we decided to have all of our code and assets in one single repository.

- Hermetic: *All* dependencies must be checked in into de monorepo. This heavily decreases the
  maintenance burden, as builds (locally or on CI) do not depend on the machine's environment to
  work. This comes with the burden to have to vendor (check-in) all the third party dependendies
  into the monorepo. (NOTE: these dependencies *are not* present in this github repository, they
  would have to be re-vendored as needed).

## Build System: Bazel & sgeb

With the requirements in mind, we decided to base the build system for SG&E on [Bazel](https://Bazel.build).
This is because it is a polyglot (multi-language) build system designed to work on monorepos:
Google's internal version of Bazel powers the largest repository of the world.

While Bazel is very extensible and supports many targets, there are certain projects that it is not
reasonable or feasable to build with Bazel. The clearest example of this are the game engines, which
normally have their own build orchestrator: Unreal has UnrealBuildTool and Unity drives it's own
build internally as a black box.

Since we wanted to support one single build system regardless of the target and support *all* the
possible targets, we decided to create a layer on top of Bazel that would cover all the cases: SG&E
Build, or sgeb. The code for sgeb can be found in `build/cicd/sgeb`.

sgeb is a Bazel-like system in terms of its interface (BUILDUNIT files vs BUILD files that Bazel
uses) that can delegates the build of a sgeb target to an underlying tool that knows how to do it.
If it's a normal Bazel target (like a Go program), sgeb will delegate to Bazel. But if it is a more
uncommon target, programmers are able to write custom programs that know how to build that target.
sgeb will then build and invoke this builder for them. Builders are meant to build targets that
already have their special way of building that it is not reasonable to port to Bazel. For all other
cases Bazel should be used. In the game engine examples, there would be an `unreal_builder` that
drives the Unreal build and an `unity_builder` that drives the Unity builds. These builders are sgeb
targets themselves, meaning that can be written in any language that sgeb supports. In practice,
they are all Go programs. Builders can be found in `build/builders`.

NOTE: This open source version was modified to build with the normal Go flow (go build), with some
      caveats. See the build scripts and repobuilder for more details. While the tooling _builds_,
      most of the functionality will not work as it expects a valid Bazel WORKSPACE and several
      other setups (eg. MONOREPO). It also has heavy assumptions of running in a Perforce depot.

You can see more documentation on this on `docs/sgeb.md`.

## CICD

Most of the infrastructure was written in Go, using protobuf for configuration. Our strategy for
CICD was to have a single binary that had a simple plugin architecture to drive common use cases
(presubmit, building, etc.). The goal was to maintain as much logic as possible within the monorepo
and not rely in external CICD platforms for configuration.

The code for the cicd code can be found in `build/cicd`. The program that was run on CI machines is
found in `build/cicd/cirunner`.

You can see more documentation on this on `docs/sgep.md`.

# How to run

It is important to note that the way the project builds in this github repository is not the same
that was used in SG&E. This is because Bazel is not used for driving the build in this case, in
order to simplify distribution. Instead we modifying the source to be able to be built with the
normal Go toolchain (eg. go build).

Keep in mind that there are some caveats, that Bazel and our vendored monorepo took care for use:

- Some targets (like the p4lib) use cgo to link against C++ libraries. You wil need to compile and
  provide those libraries yourself, as they are not included in this repository. You can check on
  the source of each Go package what libraries they are.
  IMPORTANT: Compile these dependencies with a GNU toolchain (MinGW), as that is the
  toolchain that Go uses. There seems to be ABI incompatibilities with the MSVC toolchain.

- Go has no concept of generating protobuf stubs, so these need to be generated before doing a
  normal build. This will require you to install the protoc compiler. We added a simple script to
  help with building the stubs, but it will require some PATH modification to work.
  This file can be found in `build_protos.bat`.

- Our setup uses some marker files to find the monorepo. In particular Bazel uses its WORKSPACE file,
  which should have the correct mapping for all the dependencies (either vendored or otherwise). The
  CICD system uses an empty MONOREPO file to mark the monorepo. The WORKSPACE and the MONOREPO file
  should be side to side. This separation came because there are multiple WORKSPACES due to the way
  we vendored.

## Third party

As the last section showed, some third party code and libraries would be needed to build. You can
see in each individual package or code where the code is expected to be but overall they conform to
the following:

```
third_party/
  lib/
    ... all pre-compiled libraries ...
  ...
  <expected third party, which would have their own structure>
  ...
```

As an example, the [p4api](https://www.perforce.com/downloads/helix-core-c/c-api) would
be installed into `third_party/p4api`.
