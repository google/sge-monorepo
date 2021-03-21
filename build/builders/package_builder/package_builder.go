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
	"flag"
	"fmt"
	"os"
	"path"
	"strings"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/monorepo/p4path"
	"sge-monorepo/build/cicd/sgeb/buildtool"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
	"sge-monorepo/build/packagemanifest"
	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/sgeflag"
)

var flags = struct {
	inputs sgeflag.StringList
	srcs   sgeflag.StringList
}{}

type pathExprReplacer struct {
	pathSet        p4path.ExprSet
	replacements   []string
	lhsWildCardIdx []int
	rhsWildCardIdx []int
}

func splitPathExprLine(line string) (string, string) {
	bits := strings.SplitN(line, " ", 2)
	expr := bits[0]
	var repl string
	if len(bits) > 1 {
		repl = bits[1]
	}
	return expr, repl
}

func validatePathExprLine(line string) error {
	expr, repl := splitPathExprLine(line)
	// Ignores should not have a replacement mapping.
	if repl != "" && strings.HasPrefix(expr, "-") {
		return fmt.Errorf("mappings with an ignore should not have a right-hand-side: %q", line)
	}
	if strings.Index(line, "*") != -1 {
		return fmt.Errorf("* wildcards are currently not supported: %q", line)
	}
	lhsWildCardCount := strings.Count(expr, "...")
	rhsWildCardCount := strings.Count(repl, "...")
	if lhsWildCardCount != rhsWildCardCount || lhsWildCardCount > 1 {
		return fmt.Errorf("mappings must have either one or zero wildcards on both sides: %q", line)
	}
	rhsWildCardIdx := strings.Index(repl, "...")
	if rhsWildCardIdx >= 0 && rhsWildCardIdx != len(repl)-3 {
		return fmt.Errorf("wildcard must be at the end of the right-hand side mapping: %q", line)
	}
	return nil
}

func makePathExprReplacer(mr monorepo.Monorepo, relTo monorepo.Path, lines []string) (*pathExprReplacer, error) {
	ret := &pathExprReplacer{}
	var exprs []string
	for _, line := range lines {
		if err := validatePathExprLine(line); err != nil {
			return nil, err
		}
		expr, repl := splitPathExprLine(line)
		exprs = append(exprs, expr)
		ret.replacements = append(ret.replacements, repl)
		ret.rhsWildCardIdx = append(ret.rhsWildCardIdx, strings.Index(repl, "..."))
	}
	var err error
	ret.pathSet, err = p4path.NewExprSet(mr, relTo, exprs)
	if err != nil {
		return nil, fmt.Errorf("invalid path expression: %v", err)
	}
	for i := 0; i < len(ret.pathSet); i++ {
		expr, _ := ret.pathSet.ExprAt(i)
		ret.lhsWildCardIdx = append(ret.lhsWildCardIdx, strings.Index(string(expr), "..."))
	}
	return ret, nil
}

func (per *pathExprReplacer) packagePathForInput(p monorepo.Path) (string, bool, error) {
	m, idx, err := per.pathSet.FindMatch(p)
	if err != nil {
		return "", false, err
	} else if !m {
		return "", false, nil
	}
	r := per.replacements[idx]
	lwci := per.lhsWildCardIdx[idx]
	rwci := per.rhsWildCardIdx[idx]
	// Non-wildcard match? Return rhs exactly has is then
	if lwci == -1 {
		return r, true, nil
	}
	pkgPath := r[0:rwci] + string(p)[lwci:]
	return pkgPath, true, nil
}

func internalMain() error {
	helper := buildtool.MustLoad()
	mr, _, err := monorepo.NewFromPwd()
	if err != nil {
		return err
	}
	relTo, err := mr.NewPath("", helper.Invocation().BuildUnitDir)
	if err != nil {
		return err
	}
	// Process inputs.
	inputMap, err := makePathExprReplacer(mr, relTo, flags.inputs)
	if err != nil {
		return err
	}
	pkg := packagemanifest.NewBuilder()
	for _, artifactSet := range helper.Invocation().Inputs {
		for _, artifact := range artifactSet.Artifacts {
			if artifact.StablePath == "" {
				continue
			}
			srcPath, isFile := buildtool.ResolveArtifact(artifact)
			if !isFile {
				continue
			}
			p, err := mr.NewPath("", artifact.StablePath)
			if err != nil {
				return err
			}
			if pkgPath, ok, err := inputMap.packagePathForInput(p); err != nil {
				return err
			} else if ok {
				if err := pkg.Add(pkgPath, srcPath); err != nil {
					return err
				}
			}
		}
	}
	// Process srcs
	srcMap, err := makePathExprReplacer(mr, relTo, flags.srcs)
	if err != nil {
		return err
	}
	// First find any file touched by any include in the set.
	potentialFiles := map[monorepo.Path]bool{}
	for i := 0; i < len(srcMap.pathSet); i++ {
		expr, isInclude := srcMap.pathSet.ExprAt(i)
		if !isInclude {
			continue
		}
		fs, err := expr.FindFiles(mr)
		if err != nil {
			return err
		}
		for _, f := range fs {
			potentialFiles[f] = true
		}
	}
	// Next filter the files by the expr set and use any matching expr to get a pkg path.
	for f := range potentialFiles {
		if pkgPath, ok, err := srcMap.packagePathForInput(f); err != nil {
			return err
		} else if ok {
			srcPath := mr.ResolvePath(f)
			if err := pkg.Add(pkgPath, srcPath); err != nil {
				return err
			}
		}
	}
	output, err := pkg.WritePkgManifestArtifact(path.Join(helper.Invocation().BuildInvocation.OutputDir, "manifest.textpb"))
	if err != nil {
		return err
	}
	helper.MustWriteBuildResult(&buildpb.BuildInvocationResult{
		Result: &buildpb.Result{Success: true},
		ArtifactSet: &buildpb.ArtifactSet{
			Artifacts: []*buildpb.Artifact{output},
		},
	})
	return nil
}

func main() {
	flag.Var(&flags.inputs, "input", "a list of p4 expression pairs that maps build unit dependency inputs to package paths")
	flag.Var(&flags.srcs, "src", "a list of p4 expression pairs that maps depot source files to package paths")
	flag.Parse()
	log.AddSink(log.NewGlog())
	defer log.Shutdown()
	if err := internalMain(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
