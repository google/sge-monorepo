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

// Package exec runs external commands.  It wraps the standard os/exec package
// with additional functionality for hiding console windows on Windows and
// running detached programs.
package exec

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

type Cmd = exec.Cmd
type Error = exec.Error
type ExitError = exec.ExitError

var (
	ErrNotFound = exec.ErrNotFound
	LookPath    = exec.LookPath

	Command        = exec.Command
	CommandContext = exec.CommandContext
)

// CommandTmp returns a Cmd struct to execute the named program with the
// given arguments.
//
// The command executable is first copied to a temporary directory.  This
// allows for running commands that may be overwritten during their execution.
// If the command were run from the original location, attempts to update the
// original binary would fail.
func CommandTmp(name string, args ...string) (*Cmd, error) {
	tmp, err := copyToTmp(name)
	if err != nil {
		return nil, err
	}
	cmd := Command(tmp, args...)
	return cmd, nil
}

// Cleanup deletes temporary files leftover from CommandTmp.
func Cleanup(tmp, name string) error {
	pattern := "tmp-*-" + filepath.Base(name) + ".deleteme"
	// First, remove any stale copies of the program.
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), pattern))
	if err != nil {
		return fmt.Errorf("glob failed: %v", err)
	}
	for _, match := range matches {
		if err := os.Remove(match); err != nil {
			return fmt.Errorf("failed to delete %s: %v", match, err)
		}
	}

	// Now, if we're running from TempDir, move the exe.
	// We can't just delete since it's still running.
	match, err := filepath.Match(filepath.Join(os.TempDir(), pattern), tmp)
	if err != nil {
		return fmt.Errorf("filpath.Match failed: %v", err)
	}
	if match {
		if err := os.Rename(tmp, tmp+".deleteme"); err != nil {
			return fmt.Errorf("rename %s -> %s.deleteme failed: %v", tmp, tmp, err)
		}
	}
	return nil
}

func copyToTmp(path string) (string, error) {
	src, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("error opening exe: %w", err)
	}
	defer src.Close()

	dst, err := ioutil.TempFile("", "tmp-*-"+filepath.Base(path))
	if err != nil {
		return "", fmt.Errorf("error creating temporary file: %w", err)
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	if err != nil {
		return "", fmt.Errorf("error copying exe: %w", err)
	}
	return dst.Name(), nil
}
