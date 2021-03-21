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

// package browse contains is a basic file browser.  Paths that end in '/' are assumed
// to be directories, everything else is assumed to be an actual file.
// Text files have their contents returned so that they can be displayed
// directly in the browse page.  Binary files return a link to the actual
// contents.
package browse

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/tools/ebert/ebert"
)

func Handle(ectx *ebert.Context, r *http.Request, args *struct {
	path string
	CL   string
	L    int
}) (interface{}, error) {
	user, err := ebert.UserFromRequest(r)
	if err != nil {
		return nil, ebert.NewError(
			fmt.Errorf("browse:get-user: %w", err),
			"Couldn't determine user's identity",
			http.StatusUnauthorized,
		)
	}
	ctx, err := ectx.Login(user)
	if err != nil {
		return nil, ebert.NewError(
			fmt.Errorf("browse:login: %w", err),
			"Login failed",
			http.StatusUnauthorized,
		)
	}
	path := "//" + args.path

	// Add CL to P4 path if it's among the URL params.
	printPath := path
	cl := args.CL
	if cl != "" && cl != "0" {
		printPath = fmt.Sprintf("%s@%s", printPath, cl)
	} else {
		cl = "0"
	}

	if strings.HasSuffix(path, "/") {
		return browseDirHandler(ctx, path, cl)
	}

	// Browsing a file.
	details, err := ctx.P4.PrintEx(printPath)

	if err != nil && errors.Is(err, p4lib.ErrFileNotFound) {
		// If we failed to load a file, maybe it's a directory?
		if !strings.HasSuffix(path, "/") {
			dirs, err := ectx.P4.Dirs(path)
			if err != nil {
				return nil, fmt.Errorf("error getting path %s: %w", path, err)
			}
			if len(dirs) == 0 {
				return nil, fmt.Errorf("path %s is not a file or directory", path)
			}
			return browseDirHandler(ctx, fmt.Sprintf("%s/", path), cl)
		}
		return nil, fmt.Errorf("failed to read file %s: %w", printPath, err)
	}
	if len(details) != 1 {
		return nil, fmt.Errorf("no file read")
	}
	if strings.Contains(details[0].Type, "binary") {
		// Return content as a data URL.
		mimeType := http.DetectContentType(details[0].Content)
		tag := "link"
		if strings.HasPrefix(mimeType, "image") {
			tag = "img"
		}
		data := base64.StdEncoding.EncodeToString(details[0].Content)
		return map[string]interface{}{
			"path": path,
			tag:    fmt.Sprintf("data:%s;base64,%s", mimeType, data),
		}, nil
	}
	// Link to line number?
	line := args.L
	return map[string]interface{}{
		"path": path,
		"text": string(details[0].Content),
		"cl":   details[0].Change,
		"line": line,
	}, nil
}

// History retrieves the list of changes for the specified file or directory.
func History(ectx *ebert.Context, r *http.Request, args *struct{ path string }) (interface{}, error) {
	path := "//" + args.path
	if strings.HasSuffix(path, "/") {
		path = path + "*"
	}
	return ectx.P4.Changes("-L", path)
}

func browseDirHandler(ectx *ebert.Context, path, cl string) (interface{}, error) {
	wildcard := path + "*"
	if cl != "0" {
		wildcard = wildcard + "@" + cl
	}

	// There's no single p4 command that gets both files and directories
	// so we do 'p4 files' concurrently with 'p4 dirs', with 'p4 files'
	// running in a goroutine.
	// The fileStatus type exists only so that we can return
	// ([]p4lib.FileDetails, error) via a channel.
	type fileStatus struct {
		files []p4lib.FileDetails
		err   error
	}
	fileCh := make(chan fileStatus)
	go func() {
		if strings.HasPrefix(wildcard, "//*") {
			// There are only depots at the top level, no files, so
			// this is a no-op.
			fileCh <- fileStatus{files: []p4lib.FileDetails{}, err: nil}
			return
		}
		files, err := ectx.P4.Files(wildcard)
		fileCh <- fileStatus{files: files, err: err}
	}()
	dirs, err := ectx.P4.Dirs(wildcard)
	// Now wait for the files goroutine to finish.
	details := <-fileCh
	if err != nil {
		return nil, fmt.Errorf("failed to get dirs %s: %w", wildcard, err)
	}
	if details.err != nil && !errors.Is(details.err, p4lib.ErrFileNotFound) {
		return nil, fmt.Errorf("failed to get files %s: %w", wildcard, details.err)
	}
	subdirs := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		dir = strings.TrimPrefix(dir, path)
		subdirs = append(subdirs, dir)
	}
	files := make([]map[string]string, 0, len(details.files))
	for _, detail := range details.files {
		if strings.Contains(detail.Action, "delete") {
			continue
		}
		name := strings.TrimPrefix(detail.DepotFile, path)
		files = append(files, map[string]string{
			"name": name,
		})
	}
	return map[string]interface{}{
		"path":    path,
		"subdirs": subdirs,
		"cl":      cl,
		"files":   files,
	}, nil
}
