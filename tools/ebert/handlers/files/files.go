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

// Package files contains the handler for files.
package files

import (
	"fmt"
	"net/http"
	"strings"

	"sge-monorepo/tools/ebert/ebert"
)

// fileHandler returns the actual file contents with no HTML framing.
func Handle(ectx *ebert.Context, r *http.Request, args *struct{ path string }) (interface{}, error) {
	user, err := ebert.UserFromRequest(r)
	if err != nil {
		return nil, ebert.NewError(
			fmt.Errorf("file:get-user: %w", err),
			"Couldn't determine user's identity",
			http.StatusUnauthorized,
		)
	}
	ctx, err := ectx.Login(user)
	if err != nil {
		return nil, ebert.NewError(
			fmt.Errorf("file:login: %w", err),
			"Login failed",
			http.StatusUnauthorized,
		)
	}
	path := fmt.Sprintf("//%s", args.path)

	if strings.HasSuffix(path, "/") {
		return nil, fmt.Errorf("invalid filename: %s", path)
	}

	details, err := ctx.P4.PrintEx(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	if len(details) != 1 {
		return nil, fmt.Errorf("no file read")
	}
	return details[0].Content, nil
}
