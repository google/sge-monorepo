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

// Binary basel2vs is a command line application used to generate a visual studio solution from bazel.
// Editing, debugging and building can be done from the generated visual studio solution.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"sge-monorepo/tools/bazel2vs/b2vs"
)

func printUsage() {
	fmt.Println(`Usage:
bazel2vs [options] [targets...]`)
	fmt.Println("options:")
	fmt.Println("\t -output_dir\t\t specify the output directory. Default: msbuild")
	fmt.Println("\t -solution_name\t\t specify the solution name. Default: sge")
	fmt.Println("\t [targets...] targets expressions")
}

func main() {
	var outputDir string
	var solutionName string

	flag.StringVar(&outputDir, "output_dir", "msbuild", "Output directory")
	flag.StringVar(&solutionName, "solution_name", "", "Solution name [default: sge]")
	flag.Parse()

	if flag.NArg() == 0 {
		printUsage()
		os.Exit(1)
	}

	cfg := b2vs.Config{
		OutputDir:      outputDir,
		SolutionName:   solutionName,
		Targets:        flag.Args(),
	}

	if solutionPath, err := b2vs.GenerateVSSolution(&cfg); err == nil {
		fmt.Printf("bazel2vs succeeded to create a VS solution at: %s\n", solutionPath)
	} else {
		fmt.Println(err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Println(string(exitErr.Stderr))
		}
		os.Exit(2)
	}
}
