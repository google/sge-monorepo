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
	"io/ioutil"
	"log"
	"os"
	"path"
)

func removeEmptyDirectories(dirpath string) (err error) {
	children, err := ioutil.ReadDir(dirpath)
	if err != nil {
		return
	}

	if len(children) != 0 {
		for _, child := range children {
			if child.IsDir() {
				err = removeEmptyDirectories(path.Join(dirpath, child.Name()))
				if err != nil {
					return
				}
			}
		}

		// We refresh the children list to see if we're not an empty directory.
		children, err = ioutil.ReadDir(dirpath)
		if err != nil {
			return
		}
	}

	if len(children) == 0 {
		fmt.Println("Removing:", dirpath)
		err = os.Remove(dirpath)
		if err != nil {
			return
		}
	}
	return
}

func main() {
	for _, arg := range os.Args[1:] {
		err := removeEmptyDirectories(arg)
		if err != nil {
			log.Println(err)
		}
	}
}
