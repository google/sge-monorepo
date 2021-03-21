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

// Package project contains handlers for the project dashboard.
package project

import (
	"flag"
	"fmt"
	"net/http"

	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/swarm"
	"sge-monorepo/tools/ebert/ebert"
)

type ProjectInfo struct {
	Name string
	Path string
}

type ChangeInfo struct {
	Cl     p4lib.Change
	Review swarm.Review
}

var (
	maxProjectChanges = flag.Int("max_project_changes", 1024, "Maximum # of changes to consider when looking for pending/recently submitted CLs.")
	projectInfos      = []ProjectInfo{}
)

func projInfo(name string) (*ProjectInfo, error) {
	log.Infof("searching for proj %s", name)
	for i, p := range projectInfos {
		if p.Name == name {
			return &projectInfos[i], nil
		}
	}
	return nil, fmt.Errorf("project not found %s", name)
}

func Handle(ctx *ebert.Context, r *http.Request, args *struct{ name string }) (interface{}, error) {
	user, err := ebert.UserFromRequest(r)
	if err != nil {
		return nil, ebert.NewError(
			fmt.Errorf("dashboard:getUser: %w", err),
			"Couldn't determine identity",
			http.StatusUnauthorized,
		)
	}

	name := args.name

	proj, err := projInfo(name)
	if err != nil {
		return nil, ebert.NewError(
			err,
			fmt.Sprintf("Couldn't find project %s", name),
			http.StatusInternalServerError,
		)
	}

	info, err := project(ctx, user, proj)
	if err != nil {
		return nil, ebert.NewError(
			err,
			fmt.Sprintf("Couldn't build dashboard for %s", user),
			http.StatusInternalServerError,
		)
	}

	//	rc, err := swarm.GetReviews(&ctx.Swarm, "")

	var changeNums []int
	for _, c := range info {
		changeNums = append(changeNums, c.Cl)
	}

	rc, err := swarm.GetReviewsForChangelists(&ctx.Swarm, changeNums)
	if err != nil {
		return nil, fmt.Errorf("couldn't get swarm reviews %v", err)
	}

	m := make(map[int]*swarm.Review)
	for i, r := range rc.Reviews {
		for _, com := range r.Commits {
			m[com] = &rc.Reviews[i]
		}
	}

	var clInfos []ChangeInfo
	for _, c := range info {
		var ci ChangeInfo
		ci.Cl = c
		if r, ok := m[c.Cl]; ok {
			ci.Review = *r
		}
		clInfos = append(clInfos, ci)
	}

	return map[string]interface{}{
		"user":    user,
		"project": proj.Name,
		"cls":     clInfos,
	}, nil
}

func project(ctx *ebert.Context, user string, info *ProjectInfo) ([]p4lib.Change, error) {
	ctx, err := ctx.Login(user)
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}

	changes, err := ctx.P4.Changes("-l", "-m", fmt.Sprintf("%d", *maxProjectChanges), info.Path)
	if err != nil {
		return nil, fmt.Errorf("p4.Changes: %w", err)
	}

	return changes, nil
}

func HandleProjects(ctx *ebert.Context, r *http.Request) (interface{}, error) {
	return map[string]interface{}{
		"projects": projectInfos,
	}, nil
}
