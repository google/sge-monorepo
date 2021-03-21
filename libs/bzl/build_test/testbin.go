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

// Binary testbin provides a binary that either copies itself,
// or exits successfully without doing anything.
// Used to produce a dummy test for build_test.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
)

func internalMain() error {
	out := flag.String("out", "", "location to produce a copy of myself at")
	flag.Parse()
	if *out != "" {
		bin := os.Args[0]
		src, err := os.Open(bin)
		if err != nil {
			return err
		}
		defer src.Close()

		dest, err := os.Create(*out)
		if err != nil {
			return err
		}
		defer dest.Close()
		_, err = io.Copy(dest, src)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if err := internalMain(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
