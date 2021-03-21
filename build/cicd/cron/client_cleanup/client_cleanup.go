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

	"sge-monorepo/libs/go/p4lib"
)

func internalMain() int {
	p4 := p4lib.New()
	clients, err := p4.Clients()
	if err != nil {
		fmt.Printf("could not obtain clients: %v\n", err)
		return 1
	}
	for _, client := range clients {
		fmt.Println("Client:", client)
	}
	return 0
}

func main() {
	os.Exit(internalMain())
}
