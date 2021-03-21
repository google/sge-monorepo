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
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"sge-monorepo/libs/go/builddist"
)

func help() {
	const helpString = `
Usage:
    sge-sync.exe [package-config.json]
`
	fmt.Println(helpString)
}

// getPerforceCurrentChangelist returns the current perforce changelist.
//   it needs to run p4.exe and the calling user should be logged in
func getPerforceCurrentChangelist() int {
	cmd := exec.Command("p4", "changes", "-m1", "//...#have")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal("error getting current perforce changelist: ", err)
	}
	tokens := strings.Split(string(output), " ")
	changelistStr := tokens[1]
	changelist, err := strconv.Atoi(changelistStr)
	if err != nil {
		log.Fatal("changelist not a number: ", err)
	}
	return changelist
}

func max(slice []int) int {
	if len(slice) < 1 {
		log.Fatal("max of empty slice")
	}
	maxValue := slice[0]
	for _, v := range slice {
		if v > maxValue {
			maxValue = v
		}
	}
	return maxValue
}

// runCommandWithOutput executes cmd and prints it's stdout and stderr.
func runCommandWithOutput(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func feedback(msg string) {
	fmt.Println(msg)
}

func doSync() error {
	ctx := context.Background()
	configFile := os.Args[1]
	packageConfig, err := builddist.ReadPackageConfig(configFile)
	if err != nil {
		return err
	}
	authOption := builddist.MakeDefaultAuthClientOption(ctx, *packageConfig)
	fmt.Println("fetching the list of pre-built packages")
	lp, err := builddist.ListCompletePackages(ctx, *packageConfig, authOption)
	if err != nil {
		return err
	}
	var completePackages []int
	for _, pkg := range lp {
		changelist, err := strconv.Atoi(pkg.Version)
		if err != nil {
			log.Fatal("changelist not a number : ", err)
		}
		completePackages = append(completePackages, changelist)
	}
	fmt.Println("fetching current perforce changelist")
	fmt.Println("current changelist: ", getPerforceCurrentChangelist())
	latestPackageChangelist := max(completePackages)
	fmt.Println("latest package changelist: ", latestPackageChangelist)
	fmt.Printf("sync to changelist %v? [y/n]", latestPackageChangelist)
	reader := bufio.NewReader(os.Stdin)
	char, _, err := reader.ReadRune()
	if err != nil {
		return fmt.Errorf("error reading user input: %v", err)
	}
	if char != 'Y' && char != 'y' {
		return fmt.Errorf("sync abandoned")
	}
	fmt.Println("executing p4 sync")
	cmd := exec.Command("p4", "sync", fmt.Sprintf("//...@%v", latestPackageChangelist))
	if err := runCommandWithOutput(cmd); err != nil {
		return fmt.Errorf("error running p4 sync: %v", err)
	}
	fmt.Println("downloading pre-built binairies")
	return builddist.InstallPackage(ctx, *packageConfig, fmt.Sprintf("%v", latestPackageChangelist), authOption, feedback)
}

func main() {
	if len(os.Args) <= 1 {
		help()
		os.Exit(1)
	}
	if err := doSync(); err != nil {
		fmt.Println(err)
		os.Exit(2)
	}
}
