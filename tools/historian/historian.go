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

// Binary historian gets the revision history from Perforce.

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"sge-monorepo/libs/go/p4lib"
)

func internalMain() error {
	flags := struct {
		count  int
		out    string
		depots string
	}{}
	flag.IntVar(&flags.count, "count", 0, "Count of CLs to obtain. 0 for everything.")
	flag.StringVar(&flags.out, "out", "", "File where the CSV will be extracted.")
	flag.StringVar(&flags.depots, "paths", "", "Perforce paths to export. Empty means everything.")
	flag.Parse()
	if flags.out == "" {
		flag.PrintDefaults()
		return errors.New(`"out" flag required`)
	}
	p4 := p4lib.New()
	var changesArgs []string
	changesArgs = append(changesArgs, "-s", "submitted", "-l")
	if flags.count > 0 {
		changesArgs = append(changesArgs, "-m", strconv.Itoa(flags.count))
	}
	depots := strings.Split(flags.depots, " ")
	depotCount := 0
	for _, depot := range depots {
		if depot == "" {
			continue
		}
		depotCount++
		changesArgs = append(changesArgs, depot)
	}
	// If no depot was set, we want everything.
	if depotCount == 0 {
		changesArgs = append(changesArgs, "//...")
	}
	fmt.Println("Running: ", changesArgs)
	changes, err := p4.Changes(changesArgs...)
	if err != nil {
		return fmt.Errorf("could not obtain changes: %v", err)
	}
	fmt.Printf("Got %d changes.\n", len(changes))
	fmt.Printf("Attempting to write CSV to %q.\n", flags.out)
	path := flags.out
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("Could not create csv file at %q: %v", path, err)
	}
	defer file.Close()
	// We only want one line for the description.
	replacer := strings.NewReplacer("\n", "", "\r", "", ";", "", "\"", "", "'", "")
	file.WriteString("change;user;timestamp;description;target\n")
	for _, cl := range changes {
		description := replacer.Replace(cl.Description)
		if cl.User != "jenkins" {
			continue
		}
		if !strings.HasPrefix(description, "Publish") {
			continue
		}
		split := strings.Split(description, " ")
		file.WriteString(fmt.Sprintf("%d;%s;%s;%s;%s\n", cl.Cl, cl.User, cl.Date, description, split[1]))
	}
	return nil
}

func main() {
	if err := internalMain(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
