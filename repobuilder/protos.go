// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
)

var protoGenDirName = "proto-gen"

func buildProto(cwd, proto string, includes []string, out string, options ...string) error {
    args := []string{"protoc"}
    for _, i := range includes {
        args = append(args, "-I", i)
    }
    args = append(args, "--go_out", out)
    for _, o := range options {
        args = append(args, "--go_opt", o)
    }
    args = append(args, proto)
    fmt.Println("Executing:", args)
    cmd := exec.Command(args[0], args[1:]...)
    cmd.Dir = cwd
    bout, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("could not execute %v: %w\n%s", args, err, string(bout))
    }
    return nil
}

// verifyGeneratedModule ensures a go.mod file exists in each generated proto so that we can
// maintain our directory structure.
func verifyGeneratedModule(gen string) error {
    dir := filepath.Dir(gen)
    mod := filepath.Join(dir, "go.mod")
    f, err := os.Create(mod)
    if err != nil {
        return fmt.Errorf("could not create %q: %w", mod, err)
    }
    defer f.Close()
    // We name the module the name of the package (the base of the path to the generated file).
    _, err = f.WriteString(fmt.Sprintf("module %s\n\ngo 1.14\n", filepath.Base(dir)))
    if err != nil {
        return fmt.Errorf("could not write into %q: %w", mod, err)
    }
    return nil
}

func buildSGEProtos(root string) error {
    // Make sure the out directory exists.
    outDir := filepath.Join(root, protoGenDirName)
    if !dirExists(outDir) {
        if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
            return fmt.Errorf("could not create generated directory %q: %w", outDir, err)
        }
    }
    // Find all the protos to build.
    var protos []string
    subdirs := []string{
        "libs",
        "build",
        "tools",
    }
    for _, subdir := range subdirs {
        dir := filepath.Join(root, subdir)
        if !dirExists(dir) {
            continue
        }
        dirProtos, err := findByExtension(dir, ".proto")
        if err != nil {
            return fmt.Errorf("could not find protos for %q: %w", dir, err)
        }
        protos = append(protos, dirProtos...)
    }
    // Create all the protos.
    includes := []string{
        root,
        filepath.Join(root, "third_party", "com_google_protobuf", "src"),
    }
    for _, proto := range protos {
        if err := buildProto(root, proto, includes, outDir); err != nil {
            return fmt.Errorf("could not build proto %q: %w", proto, err)
        }
    }
    // Find all the generated files.
    gens, err := findByExtension(outDir, ".pb.go")
    if err != nil {
        return fmt.Errorf("could not list generated proto files: %w", err)
    }
    for _, gen := range gens {
        if err := verifyGeneratedModule(gen); err != nil {
            return fmt.Errorf("could not generated module for %q: %w", gen, err)
        }
        fmt.Println("Generated module for:", gen)
    }
    return nil
}

func buildThirdPartyProtos(root string) error {
    // Bazel.
    bazelDir := filepath.Join(root, "third_party", "bazel.io")
    protos, err := findByExtension(bazelDir, ".proto")
    if err != nil {
        return fmt.Errorf("could not find protos in %q: %w", bazelDir, err)
    }
    includes := []string {
        bazelDir,
        filepath.Join(root, "third_party", "com_google_protobuf", "src"),
    }
    option := "paths=source_relative"
    for _, proto := range protos {
        if err := buildProto(bazelDir, proto, includes, bazelDir, option); err != nil {
            return fmt.Errorf("could not build proto %q: %w", proto, err)
        }
    }
    return nil
}

func buildProtos(root string) error {
    if err := buildSGEProtos(root); err != nil {
        return fmt.Errorf("could not build SGE protos: %w", err)
    }
    if err := buildThirdPartyProtos(root); err != nil {
        return fmt.Errorf("could not build third party protos: %w", err)
    }
    return nil
}


