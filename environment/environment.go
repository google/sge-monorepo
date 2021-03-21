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

// Binary install-environment installs the required dependencies for running SG&E software
// into the system.
package main

import (
	"fmt"
	"log"

	"sge-monorepo/environment/envinstall"
	"sge-monorepo/libs/go/p4lib"
)

func main() {
	p4 := p4lib.New()
	m, err := envinstall.NewManager(p4)
	if err != nil {
		log.Fatal(err)
	}

	// Always update to the newest version.
	fmt.Println("Installing dependencies...")
	if err := m.SyncAndInstallDependencies(); err != nil {
		log.Fatal(err)
	}
}
