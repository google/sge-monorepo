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

// Package p4path contains p4 path expression types.
package p4path

import (
	"math"
	"os"
	"path/filepath"
	"strings"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/libs/go/files"

	"github.com/mb0/glob"
)

// Expr is a superset of |Path|, in the sense that it is a monorepo relative path that
// also supports matching wildcards.
//
// Patterns:
//  Patterns can either be repo-absolute (start with "//") or start without "/", in which they
//  are repo-relative patterns. They can also refer to a subrepo if they start with the name
//  (eg. @repo). The explanation on how each behaves is as follows:
//
//  If they start with "//", they are assumed to be based upon |repoRoot|, or in other words,
//  it is understood to be a "repository absolute" pattern (eg. //1p/...).
//
//  If they don't start with "//", they are assumed to be a relative path, and they will
//  be appended to |mdDir|. These are understood to be a "relative pattern"
//  (eg. "dir/*.rs", "...", etc.).
//
//  If they start with a subrepo (eg. @repo//path/file.txt), they will get resolved to what
//  that subrepo is and then be treated as a normal resolved absolute path above.
//
// Globing:
//  Path Expressions support globing that match between a directory (or separator) boundary.
//  That is, the expression "foo/*/baz" will match any expression between foo and bar that
//  doesn't have a separator (/) on it. The expressions "foo/*/baz" matches any file names baz
//  that is under _any_ directory under foo. That is "foo/bar/baz" will match but "foo/a/bar/baz"
//  will not.
//
//  You can also have many globs: "foo/*/*/baz", which looks for any file named baz two-levels
//  deep under foo.
//
//  Finally you can have globs that span multiple directory boundaries, with **. For example,
//  the expression foo/**/baz will match any file named baz that is _at least_ one directory deep,
//  as ** will match one of more directories. So "foo/a/baz", "foo/a/b/baz" and "foo/a/b/c/baz"
//  will match. Note that "foo/baz" will not match, as there has not intermediate directory to
//  match with the glob.
//
// Wildcard:
//  The last part of the path must be one of the following:
//
//  pattern (Based on https://golang.org/pkg/path/filepath/#Match):
//     { term }
//  term:
//     '*'         matches any sequence of non-Separator characters
//     '?'         matches any single non-Separator character
//     '[' [ '^' ] { character-range } ']'
//                 character class (must be non-empty)
//     c           matches character c (c != '*', '?', '\\', '[')
//     '\\' c      matches character c
//
//  or the following special perforce patterns
//     ...     -> All files under that directory.
//     ....h   -> All files under that directory that end in .h
//
// Examples:
//  //game/...            -> Matches all file under //game.
//  //some/path/....cc    -> All .cc files under //some/path.
//  @repo//some/path/*  -> All files within //some/path in the "repo" subrepo.
//  [a-z]*.go             -> All lowercased go files.
//  ....rs                -> All .rs files under this presubmit path..
//  //game/*/*.txt        -> All txt files one level deep under //game.
//  //game/**/*.txt       -> All txt files one or more levels deep under //game.
type Expr string

// newExpr returns a Expr monorepo path. Will verify that the matching expressions are
// correct.
func newExpr(s string) (Expr, error) {
	// Use path to normalize the path.
	p := string(monorepo.NewPath(s))

	// If the base starts with "...", it means that it is a Perforce-style matching, so we don't
	// check against a glob.
	base := filepath.Base(filepath.Clean(p))
	if strings.HasPrefix(base, "...") {
		return Expr(p), nil
	}

	// Otherwise, it's a glob match. A simple dummy match will run the verification that
	// the expression is correct.
	_, err := glob.Match(p, s)
	return Expr(p), err
}

func toExpr(p monorepo.Path) (Expr, error) {
	return newExpr(string(p))
}

// NewExpr creates a Expr relative to |relTo|. Verifies that the matching expressions
// are correct.
func NewExpr(mr monorepo.Monorepo, relTo monorepo.Path, s string) (Expr, error) {
	// Under the hood, PathExprs are Paths with different wildcard endings, so they can be
	// treated as Paths.
	path, err := mr.NewPath(relTo, s)
	if err != nil {
		return "", err
	}
	return toExpr(path)
}

func (e Expr) Matches(path monorepo.Path) (bool, error) {
	// Verify that |path| is contained by this expression. We search a base that we can use to
	// compare as a parent of |path|.
	base := monorepo.NewPath(string(e)).Dir()
	for base != "" && base[len(base)-1:] == "*" {
		base = base.Dir()
	}

	// Unless we're left with a root, check if the base is correct.
	if base != "" && !base.IsParentOf(path) {
		return false, nil
	}

	// We now determine what kind of wildcard it has.
	// We clean so that we ensure the separator is correct.
	wildcard := filepath.Base(filepath.Clean(string(e)))
	if strings.HasPrefix(wildcard, "...") {
		return perforceWildcardMatch(path, wildcard), nil
	}

	// We know that we do not have a perforce wildcard, so we can simply glob.
	return glob.Match(string(e), string(path))
}

func perforceWildcardMatch(path monorepo.Path, wildcard string) bool {
	// We know that wildcard begins with "...", so len == 3 means that everything matches.
	if len(wildcard) == 3 {
		return true
	}

	// Since this is a ... match we simply compare extension (what Perforce does).
	ext := wildcard[3:]
	base := filepath.Base(filepath.Clean(string(path)))
	if len(base) < len(ext) {
		return false
	}

	return base[len(base)-len(ext):] == ext
}

// ExprSet is an ordered list of path expressions, with both includes and excludes.
type ExprSet []exprAndIncludeExclude

type exprAndIncludeExclude struct {
	expr    Expr
	include bool
}

// ExprAt returns the expression at position i as well as whether it's an include or exclude.
func (set ExprSet) ExprAt(i int) (Expr, bool) {
	return set[i].expr, set[i].include
}

// Matches returns true if the path expression matches the path.
// Each expression is evaluated in order, and the last matching
// include/exclude wins.
func (set ExprSet) Matches(p monorepo.Path) (bool, error) {
	m, _, err := set.FindMatch(p)
	return m, err
}

// FindMatch returns true if the path expression matches the path.
// The index of the last match is also returned.
// Each expression is evaluated in order, and the last matching
// include/exclude wins.
func (set ExprSet) FindMatch(p monorepo.Path) (bool, int, error) {
	match := false
	matchIndex := -1
	for i, expr := range set {
		// Skip unneeded check
		if expr.include == match {
			continue
		}
		if m, err := expr.expr.Matches(p); err != nil {
			return false, -1, err
		} else if m {
			match = expr.include
			matchIndex = i
		}
	}
	return match, matchIndex, nil
}

// NewExprSet creates a new path expression set.
func NewExprSet(mr monorepo.Monorepo, relTo monorepo.Path, exprs []string) (ExprSet, error) {
	var set ExprSet
	for _, exprStr := range exprs {
		include := true
		if strings.HasPrefix(exprStr, "-") {
			include = false
			exprStr = exprStr[1:]
		}
		expr, err := NewExpr(mr, relTo, exprStr)
		if err != nil {
			return nil, err
		}
		set = append(set, exprAndIncludeExclude{expr, include})
	}
	return set, nil
}

func (e Expr) FindFiles(mr monorepo.Monorepo) ([]monorepo.Path, error) {
	wci := math.MaxInt32
	for _, wc := range []string{"...", "*"} {
		if i := strings.Index(string(e), wc); i != -1 && i < wci {
			wci = i
		}
	}
	if wci == math.MaxInt32 {
		// No wildcards of any kind, must be a direct reference to a file.
		p := monorepo.Path(e)
		if files.FileExists(mr.ResolvePath(p)) {
			return []monorepo.Path{p}, nil
		}
		return nil, nil
	}
	// Wildcards present. Find the earliest wildcard, then walk the entire directory
	// structure starting from that directory, matching against the expression.
	var root monorepo.Path
	lastSlash := strings.LastIndex(string(e)[:wci], "/")
	if lastSlash != -1 {
		root = monorepo.Path(string(e)[:lastSlash])
	}
	var result []monorepo.Path
	if err := filepath.Walk(mr.ResolvePath(root), func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		mrp, err := mr.RelPath(p)
		if err != nil {
			return err
		}
		if m, err := e.Matches(mrp); err != nil {
			return err
		} else if m {
			result = append(result, mrp)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return result, nil
}
