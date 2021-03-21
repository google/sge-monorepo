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
    "fmt"
    "log"
    "os"
    "path/filepath"
    "strings"
    "sort"
    "sync"
)

var dirsToSkip = []string{
    ".git",
    ".github",
    "build/publishers/docker_publisher/test/testdata",
    "proto-gen",
    "third_party",
    "tools/ebert/p4_linux",
}

var extToSkip = []string{
    ".exe",
    ".chman",
    ".sum",
    ".ttf",
}

func extractKeywords(keywordsFilePath string) ([]string, error) {
    file, err := os.Open(keywordsFilePath)
    if err != nil {
        return nil, fmt.Errorf("could not open file %q: %w", keywordsFilePath, err)
    }
    defer file.Close()
    // Get all the keywords from the file.
    var keywords []string
    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        keyword := strings.Trim(scanner.Text(), " \r\n")
        if keyword == "" {
            continue
        }
        lower := strings.ToLower(keyword)
        keywords = append(keywords, lower)
    }
    if err := scanner.Err(); err != nil {
        return nil, fmt.Errorf("could not scan file %q: %w", keywordsFilePath, err)
    }
    return keywords, nil
}

type notice struct {
    path string
    lineno int
    line string
    keyword string
}

func searchFileForKeywords(filesToNotice chan<- notice, root, path string, keywords []string) error {
    rel, err := filepath.Rel(root, path)
    if err != nil {
        return fmt.Errorf("could not get rel path %q and %q: %w", root, path, err)
    }
    file, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("could not open %q: %w", path, err)
    }
    defer file.Close()
    // List line by line.
    scanner := bufio.NewScanner(file)
    for i := 1; scanner.Scan(); i++ {
        line := strings.Trim(scanner.Text(), " \r\n")
        lower := strings.ToLower(line)
        for _, keyword := range keywords {
            if !strings.Contains(lower, keyword) {
                continue
            }
            filesToNotice <- notice{
                path: rel,
                lineno: i,
                line: line,
                keyword: keyword,
            }
        }
    }
    return nil
}

var workerCount = 10
func searchForKeywords(root, keywordsFilePath string) error {
    keywords, err := extractKeywords(keywordsFilePath)
    if err != nil {
        return fmt.Errorf("could not list keywords: %w", err)
    }
    // Spawn a set of workers to look for files.
    var wg sync.WaitGroup
    wg.Add(workerCount)
    filesToProcess := make(chan string, workerCount)
    filesToNotice := make(chan notice, workerCount)
    for i := 0; i < workerCount; i++ {
        go func() {
            defer wg.Done()
            for path := range filesToProcess {
                if err := searchFileForKeywords(filesToNotice, root, path, keywords); err != nil {
                    log.Fatalf("could not search for file %q: %v", path, err)
                }
            }
        }()
    }
    // Insert all the files to notice into an array.
    var sinkwg sync.WaitGroup
    sinkwg.Add(1)
    var notices []notice
    go func() {
        defer sinkwg.Done()
        for n := range filesToNotice {
            notices = append(notices, n)
        }
    }()
    // Go over all files and pipe them for processing.
    err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }
        if info.IsDir() {
            rel, err := filepath.Rel(root, path)
            if err != nil {
                return fmt.Errorf("could not obtain relative path for %q: %w", path, err)
            }
            for _, skip := range dirsToSkip {
                // Fix separator.
                r := strings.ReplaceAll(rel, `\`, `/`)
                if r == skip {
                    fmt.Println("SKIPPING:", path)
                    return filepath.SkipDir
                }
            }
            return nil
        }
        // Avoid any extensions we don't want.
        ext := filepath.Ext(path)
        for _, skip := range extToSkip {
            if ext == skip {
                fmt.Println("SKIPPING BY EXTENSION:", path)
                return nil
            }
        }
        filesToProcess <- path
        return nil
    })
    close(filesToProcess)
    wg.Wait()
    close(filesToNotice)
    sinkwg.Wait()
    sort.Slice(notices, func(i, j int) bool {
        if notices[i].path != notices[j].path {
            return notices[i].path < notices[j].path
        }
        return notices[i].lineno < notices[j].lineno
    })
    for _, n := range notices {
        fmt.Printf("%s:%d: Contains %q -> %s\n", n.path, n.lineno, n.keyword, n.line)
    }
    if err != nil {
        return fmt.Errorf("could not walk dir %q: %w", root, err)
    }
    return nil
}
