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

// p4cmd implements something like the p4 command line utility via p4lib.
// Meant as a test for the library, not for actual use.
package main

import (
	"flag"
	"fmt"
	"strconv"

	"sge-monorepo/libs/go/p4lib"
)

func main() {
	flag.Parse()
	args := flag.Args()

	p4 := p4lib.New()
	switch args[0] {
	case "testuser":
		// This is a pseudo-command meant to verify changing user/passwd.
		// It should be called with 2 arguments, representing the user and
		// passwd of a user with different permissions than the default user
		// (really the permissions should be different with respect to repo
		// visibility).  I use 'testuser' who has very restricted permissions.
		if len(args) < 3 {
			fmt.Println("testuser takes two arguments")
			return
		}
		r, err := p4.Dirs("//*")
		if err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Printf("dirs: %v\n", r)
		}
		testp4 := p4lib.NewForUser(args[1], args[2])
		r, err = testp4.Dirs("//*")
		if err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Printf("dirs: %v\n", r)
		}
		r, err = p4.Dirs("//*")
		if err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Printf("dirs: %v\n", r)
		}
	case "testindex":
		// This is a psuedo-command ment to exercise p4 index.
		if len(args) < 5 {
			fmt.Println("testindex takes at least 4 arguments")
			return
		}
		attr, err := strconv.Atoi(args[2])
		if err != nil {
			fmt.Printf("second parameter must be an int: %v\n", err)
			return
		}
		err = p4.Index(args[1], attr, args[3:]...)
		if err != nil {
			fmt.Printf("p4 index failed: %v", err)
			return
		}
		err = p4.IndexDelete(args[1], attr, args[3:]...)
		if err != nil {
			fmt.Printf("p4 index delete failed: %v", err)
			return
		}
		fmt.Println("p4 index appears successful")
	case "changes":
		r, err := p4.Changes(args[1:]...)
		if err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Printf("changes: %v", r)
		}
	case "fstat":
		r, err := p4.Fstat(args[1:]...)
		if err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Printf("fstat: %v", r)
		}
	case "keys":
		if len(args) < 2 {
			fmt.Println("keys takes one argument")
			return
		}
		r, err := p4.Keys(args[1])
		if err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Printf("keys: %v", r)
		}
	case "print":
		details, err := p4.PrintEx(args[1:]...)
		if err != nil {
			fmt.Printf("error: %v\n", err)
		}
		for _, detail := range details {
			fmt.Printf("file %s\n%s\n", detail.DepotFile, detail.Content)
		}
	default:
		r, err := p4.ExecCmd(args...)
		if err != nil {
			fmt.Printf("error: %v\n", err)
		} else {
			fmt.Print(string(r))
		}
	}
}
