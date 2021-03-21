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

// Binary gazelle_wrapper wraps the real gazelle.exe (checked into the
// depot as "gazelle_actual.exe"). It sets the GOROOT to our vendored
// Go SDK before forwarding to the real gazelle.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"sge-monorepo/libs/go/p4lib"
)

func internalMain() error {
	p4 := p4lib.New()
	gazelle, err := p4.Where("//sge/bin/windows/gazelle_actual.exe")
	if err != nil {
		return fmt.Errorf("cannot find actual gazelle: %v", err)
	}
	goRootFile, err := p4.Where("//sge/third_party/toolchains/go/1.14.3/ROOT")
	if err != nil {
		return fmt.Errorf("cannot find goroot marker file: %v", err)
	}
	goRoot := filepath.Dir(goRootFile)

	args := os.Args[1:]
	cmd := exec.Command(gazelle, args...)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOROOT=%s", goRoot))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	if err := internalMain(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
