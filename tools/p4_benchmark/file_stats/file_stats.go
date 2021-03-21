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
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Prints the usage.
func printUsage() {
	fmt.Println(`Usage:
file_stats <path>`)
	fmt.Println("  prints number of files and total size in bytes")
}

func runStats() error {
	flag.Parse()

	if flag.NArg() < 1 {
		printUsage()
		return fmt.Errorf("insufficient number or arguments specified")
	}

	path := flag.Arg(0)
	var totalSize int64 = 0
	count := 0
	err := filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			totalSize += info.Size()
			count += 1
			return nil
		})

	if err != nil {
		return fmt.Errorf("Failed to walk the directory tree at %s: %v\n", path, err)
	}

	fmt.Printf("count: %d\n", count)
	fmt.Printf("size: %d\n", totalSize)

	return nil
}

func main() {
	if err := runStats(); err != nil {
		fmt.Printf("Execution failed: %v\n", err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			fmt.Printf("exitErr.Stderr: %v\n", exitErr.Stderr)
		}
		os.Exit(1)
	}
}
