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
    "path/filepath"
    "strings"
)

func dirExists(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

func fileExists(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.Mode().IsRegular()
}

func findByExtension(dir, extension string) ([]string, error) {
    var files []string
    err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if info.IsDir() {
            return nil
        }
        if !strings.HasSuffix(path, extension) {
            return nil
        }
        files = append(files, path)
        return nil
    })
    if err != nil {
        return nil, fmt.Errorf("could not find all files with extension %q: %w", extension, err)
    }
    return files, nil
}

// lookUpwardsForFile looks for |filename| in the current directory or any parents until the root.
// Returns the directory it was found or an error.
func lookUpwardsForFile(dir, filename string) (string, error) {
	for {
		// We iterate until we find the same dir or a separator
		if len(dir) == 1 && (dir[0] == '.' || dir[0] == os.PathSeparator) {
			break
		}
		if len(dir) == 0 || dir[len(dir)-1] == os.PathSeparator {
			break
		}
		mr := filepath.Join(dir, filename)
		if stat, err := os.Stat(mr); err == nil && stat.Mode().IsRegular() {
            return dir, nil
		}
		dir = filepath.Dir(dir)
	}
    return "", fmt.Errorf("could not find %q starting from %q", filename, dir)
}
