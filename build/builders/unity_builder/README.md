# unity_builder

unity_builder is a build tool meant for building unity games via SG&E build system.

This means that is can be used a driver (bin argument) of a `build_unit`.

## Usage

Building a unity game automatically requires two parts: calling the CLI tool (Unity.exe) and some
driving code within the Unity project that can be called from the outside. The way unity builder
works is that it has defined an CLI interface between unity_builder and a standard file called
[BuildConfig.cs](//sge/build/builders/unity_builder/BuildConfig.cs).

This file receives command flags from unity builder and drives the build in a "generic way", as it
does not know the specifics of the project build. For that, project owners will have to define a
class that implements the `IBuildConfig` interface, providing the specifics for their project. The
BuildConfig.cs file will then find that class via reflection and invoke the interface accordingly.

The interface is based via "profiles", which is a string ID that identifies which build you want to
make (eg. "console-X", "windows-dev", etc.). This IDs are opaque to the build tool, hence the need for
the project owners to provide the specifics about how to build these via the interface.

### Providing a new project

1. Copy BuildConfig.cs into the project into `<ASSETS>/SGE/BuildUtils/Editor/BuildConfig.cs`.
   The easiest way to achieve this is dragging into Unity, so that it takes care of all the
   "meta" things.
2. Add a .cs file with a class that implements the `IBuildConfig` interface.
3. Define build_unit that provide targets for the defined profiles in (2).
4. Call `sgeb build <TARGET>` to build.

### Flags

The Unity Builder build_unit has the following arguments:

* -profile: (REQUIRED) Which build profile to execute.

* -editor: (REQUIRED) Monodepot path to the Unity Editor that will be used to build.
           Eg: //engines/unity/editor/2019.3.11f1

* -project: (OPTIONAL) Monodepot path to the project to be built. This path should point to the
            Unity project directory (eg. Where the Assets folder is).
            If not provided, it assumes that the path of the containing BUILDUNIT is also the
            project path.

* -output-name: (OPTIONAL) Output name for the executable. If ommited, it will use the basename
                (ie. filepath.Base) of the -project value (eg. If project is //sge/1p/foo,
                then the output-name will be foo).
