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

// Binary rename_repo_files helps all files in the repo of a given name, it is used to help
// transition names
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"sge-monorepo/libs/go/p4lib"
)

type mapping struct {
	fromPath string
	toPath   string
}

func collectMappings(dirpath string, from string, to string, mappings *[]mapping) (err error) {
	children, err := ioutil.ReadDir(dirpath)
	if err != nil {
		return err
	}

	for _, child := range children {
		childName := path.Join(dirpath, child.Name())
		if child.IsDir() {
			if err = collectMappings(childName, from, to, mappings); err != nil {
				return err
			}
		} else {
			fromPath := path.Join(dirpath, from)
			if strings.EqualFold(childName, fromPath) {
				toPath := path.Join(dirpath, to)
				if _, err := os.Stat(toPath); err == nil {
					fmt.Printf("- Skipping %s because %s exists\n", fromPath, toPath)
				} else if os.IsNotExist(err) {
					fmt.Printf("+ Renaming %s to %s\n", fromPath, toPath)
					*mappings = append(*mappings, mapping{fromPath, toPath})
				}
			}
		}
	}

	return
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage:")
		fmt.Println("  rename_repo_files from_file_name to_file_name dir1 [dir2 dir3 ...]")
		fmt.Println("Example:")
		fmt.Println("  rename_repo_files METADATA.textpb METADATA .")
		os.Exit(1)
	}

	from := os.Args[1]
	to := os.Args[2]
	mappings := []mapping{}
	for _, arg := range os.Args[3:] {
		if err := collectMappings(arg, from, to, &mappings); err != nil {
			fmt.Printf("Encountered error: %s. Stopping processing\n", err)
		}
	}

	if len(mappings) == 0 {
		os.Exit(0)
	}

	p4 := p4lib.New()
	cl, err := p4.Change(fmt.Sprintf("Rename %s to %s", from, to))
	if err != nil {
		fmt.Printf("Failed creating changelist %s\n", err)
		os.Exit(1)
	}

	for _, mapping := range mappings {
		p4.Edit([]string{mapping.fromPath}, cl)
		_, err = p4.Move(cl, mapping.fromPath, mapping.toPath)
		if err != nil {
			fmt.Printf("Failed to move file %s to %s\n", mapping.fromPath, mapping.toPath)
		}
	}
}
