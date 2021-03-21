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

// Binary p4-split is a workaround for perforce's problematic behaviour with large changelists
// Perforce operations containing changelists over a certain size (25k+) can hang and never complete
// This tool allows you to chunk up these operations into a set of smaller operations under the
// threshold value
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

type node struct {
	folderName string
	fileCount  int
	children   []node
}

func directoryTreeBuild(root string) (node, error) {
	tree := node{
		folderName: filepath.Base(root),
	}
	path := root + "/*"
	entries, err := filepath.Glob(path)
	if err != nil {
		return tree, err
	}
	for _, entry := range entries {
		if fi, err := os.Stat(entry); err == nil {
			if fi.Mode().IsDir() {
				child, err := directoryTreeBuild(filepath.Join(root, fi.Name()))
				if err != nil {
					return tree, err
				}
				tree.fileCount += child.fileCount
				tree.children = append(tree.children, child)
			} else {
				tree.fileCount++
			}
		}
	}
	return tree, nil
}

func emitMoveCommands(tree node, src string, dst string, threshold int) {
	if tree.fileCount > threshold {
		for _, child := range tree.children {
			emitMoveCommands(child, fmt.Sprintf("%s/%s", src, child.folderName), fmt.Sprintf("%s/%s", dst, child.folderName), threshold)
		}
	}
	fmt.Printf("p4 edit %s/...\n", src)
	fmt.Printf("p4 move %s/... %s/...\n", src, dst)
}

func main() {
	threshold := flag.Int("threshold", 25000, "file threshold for subdividing work")
	flag.Parse()
	if len(flag.Args()) < 3 {
		fmt.Println("usage p4-split <local_src> <depot_src> <depot_dst>")
		os.Exit(1)
	}
	local := flag.Args()[0]
	src := flag.Args()[1]
	dst := flag.Args()[2]
	tree, err := directoryTreeBuild(local)
	if err != nil {
		log.Fatal(err)
	}
	emitMoveCommands(tree, src, dst, *threshold)
}
