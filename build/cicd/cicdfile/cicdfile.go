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

package cicdfile

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"sge-monorepo/build/cicd/cicdfile/protos/cicdfilepb"
	"sge-monorepo/build/cicd/monorepo"
	"github.com/golang/protobuf/proto"
)

// FileName is the name of the CICD file
const FileName = "CICD"

// OptionalExtension is usually used when a directory called "cicd" already exists
const OptionalExtension = ".textpb"

// Provider finds affected cicd files given a set of monorepo files.
type Provider interface {
	// Goes over all the locations given in |paths| and search for any existing CICD
	// files contained in them. This is done by a 2-step approach: first look for all possible
	// locations to search (and de-duplicate them) and them search for each place in a
	// multithreaded fashion.
	// |filename| represents the name of cicd file to search for. In production it would
	// be CICD.
	FindCicdFiles(mr monorepo.Monorepo, paths []monorepo.Path) ([]File, error)
}

// File holds all the information needed to deal with a CICD message found in
// the wild. It includes the message itself and associated data.
type File struct {
	Path  monorepo.Path
	Proto *cicdfilepb.CicdFile
}

// NewProvider returns a production provider.
func NewProvider() Provider {
	return NewProviderWithFileName(FileName, OptionalExtension)
}

// NewProviderWithFileName returns a cicd file provider looking for CICD files with a given name.
// Useful in tests where you can't check in testdata CICD files with the production name.
func NewProviderWithFileName(mdFileName string, mdOptionalExtension string) Provider {
	return &provider{mdFileName, mdOptionalExtension}
}

type provider struct {
	mdFileName          string
	mdOptionalExtension string
}

func (p *provider) FindCicdFiles(mr monorepo.Monorepo, paths []monorepo.Path) ([]File, error) {
	mdFiles := findCicdFiles(mr, paths, p.mdFileName, p.mdOptionalExtension)
	return loadCicdFiles(mr, mdFiles)
}

// fileSet is a thread safe monorepo path set.
type fileSet struct {
	files map[monorepo.Path]bool
	mutex sync.Mutex
}

func newFileSet() *fileSet {
	return &fileSet{
		files: make(map[monorepo.Path]bool),
	}
}

// add adds a path. Returns whether the path was not already in the set.
func (fs *fileSet) add(f monorepo.Path) bool {
	fs.mutex.Lock()
	defer fs.mutex.Unlock()
	if fs.files[f] {
		return false
	}
	fs.files[f] = true
	return true
}

// findCicdForInput recursively looks down from |filepath.Dir(inputPath) looking for CICD files.
// Stops when we hit the monorepo root.
func findCicdFilesForInput(mr monorepo.Monorepo, input monorepo.Path, searched *fileSet, found *fileSet, mdFileName string, mdOptionalExtension string) {
	dir := filepath.Dir(string(input))
	for {
		if dir == "." {
			dir = ""
		}
		if !searched.add(monorepo.NewPath(dir)) {
			return
		}
		// Search for a CICD or CICD.textpb file here.
		for _, fileName := range []string{mdFileName, mdFileName + mdOptionalExtension} {
			mdPath := monorepo.NewPath(path.Join(dir, fileName))
			f := mr.ResolvePath(mdPath)
			if stat, err := os.Stat(f); err == nil {
				if !stat.Mode().IsDir() {
					found.add(mdPath)
					break
				}
			}
		}

		if dir == "" {
			// Monorepo root, break.
			return
		}
		dir = filepath.Dir(dir)
	}
}

func findCicdFiles(mr monorepo.Monorepo, paths []monorepo.Path, mdFileName string, mdOptionalExtension string) *fileSet {
	searched := newFileSet()
	found := newFileSet()
	var wg sync.WaitGroup
	wg.Add(len(paths))
	for _, p := range paths {
		go func(input monorepo.Path) {
			defer wg.Done()
			findCicdFilesForInput(mr, input, searched, found, mdFileName, mdOptionalExtension)
		}(p)
	}
	wg.Wait()
	return found
}

func loadCicdFiles(mr monorepo.Monorepo, mdFiles *fileSet) ([]File, error) {
	var wg sync.WaitGroup
	wg.Add(len(mdFiles.files))
	var mutex sync.RWMutex
	var cicdFiles []File
	var errors []error
	for mdFile := range mdFiles.files {
		go func(md monorepo.Path) {
			defer wg.Done()
			p := mr.ResolvePath(md)
			cicdFile, err := readCicdFileProto(p)
			mutex.Lock()
			defer mutex.Unlock()
			if err != nil {
				errors = append(errors, fmt.Errorf("%s: %v", p, err))
			} else {
				cicdFiles = append(cicdFiles, File{
					Path:  md,
					Proto: cicdFile,
				})
			}
		}(mdFile)
	}
	wg.Wait()
	if len(errors) > 0 {
		var msgs []string
		for _, err := range errors {
			msgs = append(msgs, err.Error())
		}
		return nil, fmt.Errorf("could not load CICD files\n%s", strings.Join(msgs, "\n"))
	}
	sort.Slice(cicdFiles, func(i, j int) bool { return cicdFiles[i].Path < cicdFiles[j].Path })
	return cicdFiles, nil
}

func readCicdFileProto(path string) (*cicdfilepb.CicdFile, error) {
	in, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cicdFile := &cicdfilepb.CicdFile{}
	if err := proto.UnmarshalText(string(in), cicdFile); err != nil {
		return nil, err
	}
	return cicdFile, nil
}
