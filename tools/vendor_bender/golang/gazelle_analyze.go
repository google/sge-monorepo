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

package golang

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var goLibraryRe = regexp.MustCompile(`^go_library\(`)
var nameRe = regexp.MustCompile(`name = "([^"]+)"`)

type gzResult struct {
	// repo is the name of the analyzed repo
	repo string
	// isGoRepo returns true if there are any Go rules
	isGoRepo bool
	// isGazelleGenerated is set when the analyzer thinks all BUILD files were generated by Gazelle.
	isGazelleGenerated bool
	// gazelleProblems is a set of targets that the analyzer thinks isn't generated by Gazelle.
	gazelleProblems []string
}

func hasGoLibrary(buildFile string) bool {
	for _, l := range strings.Split(buildFile, "\n") {
		if goLibraryRe.MatchString(l) {
			return true
		}
	}
	return false
}

func analyzePkg(mrRoot string, repo string) (gzResult, error) {
	var isGoRepo bool
	var problems []string
	repoRoot := path.Join(mrRoot, "third_party", repo)
	err := filepath.Walk(repoRoot, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		p = strings.ReplaceAll(p, "\\", "/")
		// Skip anything that isn't a BUILD file.
		if !strings.HasPrefix(path.Base(p), "BUILD") {
			return nil
		}
		bytes, err := ioutil.ReadFile(p)
		if err != nil {
			return err
		}
		str := string(bytes)
		isGoRepo = isGoRepo || hasGoLibrary(str)
		pkg := path.Base(path.Dir(p))

		// Attempt to find any target that appears not to be generated by Gazelle.
		// This is a heuristic, it is possible to get false positives.
		for _, matches := range nameRe.FindAllStringSubmatch(str, -1) {
			m := matches[1]
			switch m {
			case "go_default_library", "go_default_test", "go_tool_library":
				// Gazelle targets with go_naming_convention=go_default_library.
			case "all_files":
				// Gazelle filegroup targets generated with its test support.
				// This should be turned off (we don't want to generate these),
				// but legacy gazelle runs will have these.
			case pkg:
				// go_binary or go_library with the same name as the pkg,
				// Gazelle always gives go_binary targets this name.
				// Gazelle generates go_libraries with these names when go_naming_convention=import.
			case pkg + "_lib":
				// Gazelle generated go_library in a binary package with go_naming_convention=import.
			case pkg + "_test":
				// Gazelle generated go_test with go_naming_convention=import.
			default:
				if strings.HasSuffix(m, "_proto") {
					// (Potentially) Gazelle-generated proto_library or go_proto_library.
					continue
				}
				problem := fmt.Sprintf("%s: contains %s", p, m)
				problems = append(problems, problem)
			}
		}
		return nil
	})
	if err != nil {
		return gzResult{}, err
	}
	result := gzResult{
		repo:     repo,
		isGoRepo: isGoRepo,
	}
	if isGoRepo {
		result.isGazelleGenerated = len(problems) == 0
		result.gazelleProblems = problems
	}
	return result, nil
}

func exclude(repo string) bool {
	switch repo {
	case "unreal":
		return true
	case "go":
		return true
	}
	return false
}

func analyzeThirdParty(mrRoot string) ([]gzResult, error) {
	var res []gzResult
	{
		r, err := analyzeDir(mrRoot, "")
		if err != nil {
			return nil, err
		}
		res = append(res, r...)
	}

	{
		r, err := analyzeDir(mrRoot, "go")
		if err != nil {
			return nil, err
		}
		res = append(res, r...)
	}

	return res, nil
}

func analyzeDir(root, rel string) ([]gzResult, error) {
	dir := path.Join(root, "third_party", rel)
	dirs, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var results []gzResult
	for _, d := range dirs {
		if !d.IsDir() || exclude(d.Name()) {
			continue
		}
		r, err := analyzePkg(root, path.Join(rel, d.Name()))
		if err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, nil
}

var gazelleAnalyzeFlags = struct {
	verbose   bool
	omitNonGo bool
}{}

func PrintGazelleAnalyzeUsage() {
	fmt.Println("  gazelle-analyze [-omit-non-go=1] [-verbose=0] [pkg]")
	fmt.Println("    Analyzes the Gazelle state of third_party libraries.")
}

// gazelleAnalyze looks at all third_party packages.
// It outputs a list of all Go packages and whether they are gazelle generated.
// If they aren't, it explains why it thinks so if -verbose.
func GazelleAnalyze(mrRoot string, verbose, omitNonGo bool) error {
	flags := flag.NewFlagSet("gazelle-analyze", flag.ExitOnError)
	gazelleAnalyzeFlags.verbose = verbose
	gazelleAnalyzeFlags.omitNonGo = omitNonGo

	var results []gzResult
	if flags.NArg() > 0 {
		r, err := analyzePkg(mrRoot, flags.Arg(0))
		if err != nil {
			return err
		}
		results = append(results, r)
	} else {
		var err error
		results, err = analyzeThirdParty(mrRoot)
		if err != nil {
			return err
		}
	}
	printResults("Gazelle generated", results, func(res gzResult) bool {
		return res.isGoRepo && res.isGazelleGenerated
	})
	printResults("Not generated by Gazelle", results, func(res gzResult) bool {
		return res.isGoRepo && !res.isGazelleGenerated
	})
	if !gazelleAnalyzeFlags.omitNonGo {
		printResults("Non-Go repos", results, func(res gzResult) bool {
			return !res.isGoRepo
		})
	}
	return nil
}

func printResults(header string, results []gzResult, filter func(res gzResult) bool) {
	var toPrint []gzResult
	for _, r := range results {
		if filter(r) {
			toPrint = append(toPrint, r)
		}
	}
	if len(toPrint) == 0 {
		return
	}
	sort.Slice(toPrint, func(i, j int) bool { return results[i].repo < results[j].repo })
	fmt.Println(header)
	for _, r := range toPrint {
		fmt.Println("  " + r.repo)
		if r.isGoRepo && !r.isGazelleGenerated && gazelleAnalyzeFlags.verbose {
			fmt.Println("    Non-Gazelle generated targets:")
			for _, problem := range r.gazelleProblems {
				fmt.Printf("    %s\n", problem)
			}
		}
	}
}
