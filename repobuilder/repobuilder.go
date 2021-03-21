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

// Binary repobuilder is a bespoke tool to building the SG&E monorepo.
package main

import (
    "fmt"
    "os"
)

func internalMain(mode string, args ...string) error {
    cwd, err := os.Getwd()
    if err != nil {
        return fmt.Errorf("could not get cwd: %w", err)
    }
    root, err := lookUpwardsForFile(cwd, ".REPOBUILDER_MARKER")
    if err != nil {
        return fmt.Errorf("could not find repo root: %w", err)
    }
    switch mode {
    case "keywords":
        if err := searchForKeywords(root, args[0]); err != nil {
            return fmt.Errorf("could not search for keywords: %w", err)
        }
    case "protos":
        if err := buildProtos(root); err != nil {
            return fmt.Errorf("could not build protos: %w", err)
        }
    default:
        return fmt.Errorf("could not find mode %q", mode)
    }
    return nil
}

func printUsage() {
    fmt.Println(`Usage: repobuilder <MODE> <ARGS>

    Modes:
        keywords <INPUT_FILE>: Searches for keywords in all repo
        protos: Build all protos`)
}

func main() {
    if len(os.Args) < 2 {
        printUsage()
        os.Exit(1)
    }
    mode := os.Args[1]
    if err := internalMain(mode, os.Args[2:]...); err != nil {
        fmt.Println(err)
        os.Exit(1)
    }
}
