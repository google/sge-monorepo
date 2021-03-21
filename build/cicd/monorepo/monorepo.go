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

// Package monorepo provides path manipulation inside an SGE monorepo.
package monorepo

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"sge-monorepo/libs/go/trie"
)

// Marker is a file that lives in the root of every monorepo.
const Marker = "MONOREPO"

var debugMonorepoRoot = flag.String("debug_monorepo_root", "", "overrides where the monorepo root is for debugging")

// Monorepo is a monorepo root + the repo map
// The repo map maps subdirectories in the monorepo
// to virtual names, similar to Bazel external monorepo.
type Monorepo struct {
	// Root is the absolute path to the monorepo.
	Root string

	// map of repo name -> monorepo path.
	repoMap map[string]Path

	// trie of path -> repo name.
	repoTrie *trie.PathTrie
}

type repoEntry struct {
	name string
	path Path
}

// New returns a new monorepo.
// root is the absolute path to the root of the monorepo.
// repoMap is a map of repo name -> monorepo relative paths with the location of subrepos.
func New(root string, repoMap map[string]Path) Monorepo {
	t := trie.NewPathTrie()
	for k, v := range repoMap {
		t.Put(k, &repoEntry{
			name: k,
			path: v,
		})
	}
	return Monorepo{
		Root:     normalize(root),
		repoMap:  repoMap,
		repoTrie: t,
	}
}

// NewFromPwd constructs a monorepo by recursing up the directory tree until it finds a MONOREPO file.
// It returns the monorepo and the pwd as a monorepo relative path.
func NewFromPwd() (Monorepo, Path, error) {
	// Allow debug override of where the monorepo is. Used when executing a binary for debugging when
	// the pwd isn't inside the monorepo, for example when executing bazel run.
	// To use, pass -debug_monorepo_root=<my monorepo root> when executing the binary.
	if *debugMonorepoRoot != "" {
		mr, err := NewFromDir(*debugMonorepoRoot)
		if err != nil {
			return Monorepo{}, "", err
		}
		return mr, "", err
	}

	pwd, err := os.Getwd()
	if err != nil {
		return Monorepo{}, "", err
	}
	dir := pwd
	for {
		// We iterate until we find the same dir or a separator
		if len(dir) == 1 && (dir[0] == '.' || dir[0] == os.PathSeparator) {
			break
		}
		if len(dir) == 0 || dir[len(dir)-1] == os.PathSeparator {
			break
		}
		mr := filepath.Join(dir, Marker)
		if stat, err := os.Stat(mr); err == nil && stat.Mode().IsRegular() {
			mr, err := NewFromDir(dir)
			if err != nil {
				return Monorepo{}, "", err
			}
			rel, err := mr.RelPath(pwd)
			if err != nil {
				return Monorepo{}, "", err
			}
			return mr, rel, nil
		}
		dir = filepath.Dir(dir)
	}
	return Monorepo{}, "", fmt.Errorf("could not locate MONOREPO for %q", pwd)
}

// NewFromDir constructs a monorepo at the given monorepo root directory.
func NewFromDir(dir string) (Monorepo, error) {
	repos, err := parseWorkspace(dir)
	if err != nil {
		return Monorepo{}, err
	}
	return New(dir, repos), nil
}

var localRepoRegex = regexp.MustCompile(`local_repository\(\r?\n\s*name = "([^"]+)",\r?\n\s*path = "([^"]+)",\r?\n\s*\)`)
var sgebLoadRegex = regexp.MustCompile(`# sgeb:load (.*)\n`)

func parseWorkspace(root string) (map[string]Path, error) {
	ret := map[string]Path{}
	if err := parseLocalRepositories(root, "WORKSPACE", ret); err != nil {
		return nil, err
	}
	return ret, nil
}

func parseLocalRepositories(root string, p string, repos map[string]Path) error {
	f := filepath.Join(root, p)
	buf, err := ioutil.ReadFile(f)
	if err != nil {
		return fmt.Errorf("could not load %s: %v", f, err)
	}
	for _, match := range localRepoRegex.FindAllStringSubmatch(string(buf), -1) {
		repos[match[1]] = NewPath(match[2])
	}
	for _, match := range sgebLoadRegex.FindAllStringSubmatch(string(buf), -1) {
		if err := parseLocalRepositories(root, strings.TrimSpace(match[1]), repos); err != nil {
			return err
		}
	}
	return nil
}

// RepoForPath returns the name of the subrepo that contains a particular path,
// as well the path relative to that repo.
func (mr Monorepo) repoForPath(p Path) (string, Path) {
	val := mr.repoTrie.GetLongestPrefix(string(p))
	if val == nil {
		return "", p
	}
	entry := val.(*repoEntry)
	rel, _ := filepath.Rel(string(entry.path), string(p))
	if rel == "." {
		rel = ""
	}
	return entry.name, NewPath(rel)
}

// RepoPath returns the path of the given repo.
func (mr Monorepo) RepoPath(name string) Path {
	return mr.repoMap[name]
}

// parseRepo attempts to parse '@reponame//' from a string.
// Return values:
// repo  The name of the repository
// repoPath The path of the repo is the repo map
// rem  The remaining part of the string after the repo is parsed
func (mr Monorepo) parseRepo(s string) (repo string, repoPath Path, rem string, err error) {
	if len(s) > 0 && s[0] == '@' {
		idx := strings.Index(s, "//")
		if idx == -1 {
			err = fmt.Errorf("'@' must be followed by '$repo//' in %s", s)
			return
		}
		repo = s[1:idx]
		s = s[idx:]
		var ok bool
		if repoPath, ok = mr.repoMap[repo]; !ok {
			err = fmt.Errorf("no such repo %q referenced by %s", repo, s)
			return
		}
	}
	rem = s
	return
}

// Pkg is a Bazel package.
// For example, in '//foo/bar:baz', 'foo/bar' is the package.
// This can be different from a monorepo path inside a subrepo.
// For example, @repo//foo/bar:baz has package 'foo/bar' but monorepo path 'repo/foo/bar'
type Pkg string

// Label is a tuple of a repo, pkg, and a target.
type Label struct {
	// Name of the repository the label belongs to.
	Repo string
	// Repo-relative package directory.
	Pkg Pkg
	// Name of the target.
	Target string
}

func (l Label) String() string {
	var repo string
	if l.Repo != "" {
		repo = "@" + l.Repo
	}
	return fmt.Sprintf("%s//%s:%s", repo, l.Pkg, l.Target)
}

// TargetExpression converts a label to a target expression.
func (l Label) TargetExpression() TargetExpression {
	return TargetExpression(l.String())
}

// NewLabel converts a label string into a (repo, pkg, target) label.
// pkg is relative to the repo.
// Omitting the ":" is short-hand for a target with the same name as the
// top-level directory, eg. '//foo' expands to '//foo:foo'.
func (mr Monorepo) NewLabel(relTo Path, s string) (Label, error) {
	return mr.NewLabelWithShorthand(relTo, s, "")
}

// NewLabel converts a label string into a (repo, pkg, target) label.
// pkg is relative to the repo.
// Omitting the ":" is short-hand for a target with the shorthand applied,
// eg. '//foo' with shorthand 'tests' expands to '//foo:tests'.
// If shorthand is empty then the package name is used.
func (mr Monorepo) NewLabelWithShorthand(relTo Path, s, shorthand string) (Label, error) {
	if len(s) == 0 {
		return Label{}, fmt.Errorf("label cannot be empty")
	}
	repo, repoPath, s, err := mr.parseRepo(s)
	if err != nil {
		return Label{}, fmt.Errorf("invalid label: %v", err)
	}
	if repo != "" {
		// If repo maps to self, just drop it.
		if repoPath == "" {
			repo = ""
		}
	} else {
		repo, relTo = mr.repoForPath(relTo)
	}
	parts := strings.SplitN(s, ":", 2)
	var pkg, target string
	if len(parts) == 2 {
		pkg, target = parts[0], parts[1]
	} else {
		pkg = s
		if shorthand != "" {
			target = shorthand
		} else {
			idx := strings.LastIndex(pkg, "/")
			target = pkg[idx+1:]
		}
	}
	if strings.HasPrefix(pkg, "//") {
		pkg = pkg[2:]
	} else {
		pkg = path.Join(string(relTo), pkg)
	}
	return Label{repo, Pkg(pkg), target}, nil
}

// ResolveLabelPkgDir returns the monorepo path of the label's package.
// For instance, ResolveLabelPkgDir('//foo/bar:baz') returns 'foo/bar'
func (mr Monorepo) ResolveLabelPkgDir(l Label) (Path, error) {
	if l.Repo != "" {
		if repoPath, ok := mr.repoMap[l.Repo]; ok {
			return NewPath(path.Join(string(repoPath), string(l.Pkg))), nil
		} else {
			return "", fmt.Errorf("no such repo %q for label %s", l.Repo, l)
		}
	}
	return NewPath(string(l.Pkg)), nil
}

// Target expression is a string starting with //
// and ending in :<target>, :all, or '...'
type TargetExpression string

// NewTargetExpression returns a normalised monorepo-relative target expression.
func (mr Monorepo) NewTargetExpression(relTo Path, s string) (TargetExpression, error) {
	return mr.NewTargetExpressionWithShorthand(relTo, s, "")
}

// NewTargetExpression returns a normalised monorepo-relative target expression.
// If the target expression omits the ":" part the shorthand is used as a target name.
// Eg. "//foo:foo" -> "//foo:foo", but "//foo" -> "//foo:tests"
func (mr Monorepo) NewTargetExpressionWithShorthand(relTo Path, s, shorthand string) (TargetExpression, error) {
	if len(s) == 0 {
		return "", fmt.Errorf("target expression cannot be empty")
	}
	repo, repoPath, s, err := mr.parseRepo(s)
	if err != nil {
		return "", fmt.Errorf("invalid target expression: %v", err)
	}
	if repo != "" {
		// If repo maps to self, just drop it.
		if repoPath == "" {
			repo = ""
		}
	} else {
		repo, relTo = mr.repoForPath(relTo)
	}
	te := s
	colonIdx := strings.LastIndex(s, ":")
	if colonIdx == -1 && !strings.HasSuffix(s, "...") {
		var target string
		if shorthand != "" {
			target = shorthand
		} else {
			idx := strings.LastIndex(s, "/")
			target = s[idx+1:]
		}
		te += ":" + target
	}
	if !strings.HasPrefix(te, "//") {
		if len(te) > 0 && te[0] == ':' {
			te = string(relTo) + te
		} else {
			te = path.Join(string(relTo), te)
		}
		te = "//" + te
	}
	if len(repo) != 0 {
		te = "@" + repo + te
	}
	return TargetExpression(te), nil
}

// Path is a path relative to the monorepo root.
type Path string

// New path creates a normalized monorepo path from a string.
func NewPath(s string) Path {
	return Path(normalize(s))
}

func normalize(s string) string {
	return strings.ReplaceAll(s, "\\", "/")
}

// IsParentOf determines whether |c| is contained by this path.
// This is a non-strict version (|p| matches with itself).
func (p Path) IsParentOf(c Path) bool {
	return p != "" && (p == c || strings.HasPrefix(string(c), string(p)+"/"))
}

func (p Path) Dir() Path {
	if i := strings.LastIndex(string(p), "/"); i != -1 {
		return NewPath(string(p[0:i]))
	}

	return NewPath("")
}

// NewPath returns a monorepo path.
func (mr Monorepo) NewPath(relTo Path, s string) (Path, error) {
	var repoPath Path
	repo, repoPath, s, err := mr.parseRepo(s)
	if err != nil {
		return "", fmt.Errorf("invalid path: %v", err)
	}
	if repo == "" {
		repo, relTo = mr.repoForPath(relTo)
		if repo != "" {
			repoPath = mr.repoMap[repo]
		}
	}
	if strings.HasPrefix(s, "//") {
		return NewPath(path.Join(string(repoPath), s[2:])), nil
	}
	return NewPath(path.Join(string(repoPath), string(relTo), s)), nil
}

// RelPath makes a monorepo path from an absolute path.
func (mr Monorepo) RelPath(p string) (Path, error) {
	rel, err := filepath.Rel(mr.Root, p)
	if err != nil {
		return "", err
	}
	return NewPath(rel), nil
}

// ResolvePath returns the absolute path of the given monorepo path.
func (mr Monorepo) ResolvePath(p Path) string {
	return path.Join(mr.Root, string(p))
}
