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

package git

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"sge-monorepo/tools/vendor_bender/protos/metadatapb"
)

func execGit(pwd string, verbose bool, args ...string) (string, error) {
	com := exec.Command("git", args...)
	com.Dir = pwd
	var stdoutBuf, stderrBuf bytes.Buffer
	if verbose {
		fmt.Println(com)
		com.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)
		com.Stderr = io.MultiWriter(os.Stderr, &stderrBuf)
	} else {
		com.Stdout = bufio.NewWriter(&stdoutBuf)
		com.Stderr = bufio.NewWriter(&stderrBuf)
	}
	err := com.Run()
	return string(stdoutBuf.Bytes()), err
}

func getSha(s string) (string, error) {
	if len(s) < 40 {
		return "", fmt.Errorf("expected at least 20 hex encoded bytes")
	}
	sha := s[:40]
	_, err := hex.DecodeString(sha)
	if err != nil {
		return "", fmt.Errorf("failed to decode SHA ref of the input branch: %v", err)
	}
	return sha, nil
}

func CommitFromRef(mrRoot, url, ref string, verbose bool) (string, error) {
	gitGetShaArgs := []string{
		"ls-remote",
		url,
		// Considering we would either have a tag or branch having the same name
		fmt.Sprintf("refs/heads/%s", ref),
		fmt.Sprintf("refs/tags/%s", ref),
	}
	shaInfo, err := execGit(mrRoot, verbose, gitGetShaArgs...)
	if err != nil {
		return "", fmt.Errorf("failed to get the SHA ref of the input branch or tag: %v", err)
	}
	sha, err := getSha(shaInfo)
	if err != nil {
		return "", fmt.Errorf("failed to acquire sha ref or branch or tag: %v", err)
	}
	return sha, nil
}

func GitPkg(mrRoot string, name, url, sha, dst string, verbose bool) (*metadatapb.Metadata, error) {
	if _, err := execGit(mrRoot, verbose, []string{"init", dst}...); err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("git init failed: %v", err)
	}
	if _, err := execGit(dst, verbose, []string{"remote", "add", "origin", url}...); err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("git origin add failed: %v", err)
	}
	if _, err := execGit(dst, verbose, []string{"fetch", "origin", sha}...); err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("git fetch failed: %v", err)
	}
	if _, err := execGit(dst, verbose, []string{"reset", "--hard", "FETCH_HEAD"}...); err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("git reset failed: %v", err)
	}
	if err := os.RemoveAll(filepath.Join(dst, ".git")); err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("failed to delete .git directory in %s: %v", dst, err)
	}
	metadata := &metadatapb.Metadata{
		Name: name,
		ThirdParty: &metadatapb.ThirdParty{
			Source: &metadatapb.Source{
				GitPkg: &metadatapb.GitPkg{
					Url: url,
					Sha: sha,
				},
			},
		},
	}
	return metadata, nil
}
