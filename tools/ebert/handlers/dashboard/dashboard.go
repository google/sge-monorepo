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

// Package dashboard contains handlers for the dashboard.
package dashboard

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"time"

	"sge-monorepo/libs/go/swarm"
	"sge-monorepo/tools/ebert/ebert"
)

var (
	maxChanges = flag.Int("max_changes", 128, "Maximum # of changes to consider when looking for pending/recently submitted CLs.")
)

func Handle(ctx *ebert.Context, r *http.Request) (interface{}, error) {
	user, err := ebert.UserFromRequest(r)
	if err != nil {
		return nil, ebert.NewError(
			fmt.Errorf("dashboard:getUser: %w", err),
			"Couldn't determine identity",
			http.StatusUnauthorized,
		)
	}
	info, err := dashboard(ctx, user)
	if err != nil {
		return nil, ebert.NewError(
			err,
			fmt.Sprintf("Couldn't build the dashboard for %s. Confirm that:\n\t* you're logged in\n\t* your session hasn't expired, and\n\t* (if in dev mode) you have admin access rights", user),
			http.StatusInternalServerError,
		)
	}
	return map[string]interface{}{
		"user":      user,
		"incoming":  info["incoming"],
		"outgoing":  info["outgoing"],
		"pending":   info["pending"],
		"submitted": info["submitted"],
	}, nil
}

func dashboard(ctx *ebert.Context, user string) (map[string][]swarm.Review, error) {
	ctx, err := ctx.Login(user)
	if err != nil {
		return nil, fmt.Errorf("login: %w", err)
	}

	summary := map[string][]swarm.Review{
		"incoming":  []swarm.Review{},
		"outgoing":  []swarm.Review{},
		"pending":   []swarm.Review{},
		"submitted": []swarm.Review{},
	}

	type status struct {
		reviews []swarm.Review
		err     error
	}
	getReviews := func(values url.Values, ch chan status) {
		rc, err := swarm.GetReviews(&ctx.Swarm, values.Encode())
		ch <- status{reviews: rc.Reviews, err: err}
	}
	authorCh := make(chan status)
	go getReviews(url.Values{
		"author": []string{user},
	}, authorCh)
	participantCh := make(chan status)
	go getReviews(url.Values{
		"participants": []string{user},
	}, participantCh)

	// In my testing it seems to be faster to request all changes and
	// filter out changes that are too early vs. trying to construct
	// the arguments to have p4 filter by date.
	changes, err := ctx.P4.Changes("-l", "-m", fmt.Sprintf("%d", *maxChanges), "-u", user)
	if err != nil {
		return nil, fmt.Errorf("p4.Changes: %w", err)
	}
	cls := []string{}
	pending := []swarm.Review{}
	for _, change := range changes {
		if change.Status == "pending" {
			pending = append(pending, swarm.Review{
				ID:          change.Cl,
				Author:      change.User,
				Description: change.Description,
				Created:     int(change.DateUnix),
				Updated:     int(change.DateUnix),
				Pending:     true,
			})
		} else if time.Since(time.Unix(change.DateUnix, 0)).Hours() < 7*24 {
			cls = append(cls, fmt.Sprintf("%d", change.Cl))
		}
	}
	submittedSet := map[int]bool{}
	if len(cls) > 0 {
		submitted, err := swarm.GetReviews(&ctx.Swarm, url.Values{
			"change": cls,
		}.Encode())
		if err != nil {
			return nil, fmt.Errorf("get submitted reviews: %w", err)
		}
		if submitted.Reviews != nil {
			summary["submitted"] = submitted.Reviews
			for _, review := range submitted.Reviews {
				submittedSet[review.ID] = true
			}
		}
	}

	asyncAuthor := <-authorCh
	if asyncAuthor.err != nil {
		return nil, fmt.Errorf("swarm.GetReviews: %w", asyncAuthor.err)
	}

	asyncParticipant := <-participantCh
	if asyncParticipant.err != nil {
		return nil, fmt.Errorf("swarm.GetReviews: %w", asyncParticipant.err)
	}

	for k, reviews := range map[string][]swarm.Review{
		"incoming": asyncParticipant.reviews,
		"outgoing": asyncAuthor.reviews,
	} {
		for _, r := range reviews {
			since := time.Since(time.Unix(int64(r.Updated), 0))
			// If the reviewed CL has been commited and the review is in
			// the "outgoing" set, add it to the "submitted" set instead
			if k == "outgoing" && len(r.Commits) != 0 && since.Hours() < 7*24 {
				if !submittedSet[r.ID] {
					submittedSet[r.ID] = true
					summary["submitted"] = append(summary["submitted"], r)
				}
				continue
			}
			// Filter out archived reviews and reviews that have already
			// been committed.  These don't belong on the dashboard.
			if len(r.Commits) != 0 || r.State == "archived" {
				continue
			}
			// Filter out old reviews since we have several reviews in Swarm
			// for CLs that have been committed, but Swarm doesn't know have
			// been committed.
			// TODO: figure out a better way to filter stale reviews.
			if time.Since(time.Unix(int64(r.Updated), 0)).Hours() > 30*24 {
				continue
			}
			summary[k] = append(summary[k], r)
		}
	}

	// Make a final pass over "pending" to remove cls that are covered by
	// "outgoing".
	outgoing := map[int]bool{}
	for _, review := range summary["outgoing"] {
		for _, change := range review.Changes {
			outgoing[change] = true
		}
	}
	for _, change := range pending {
		if !outgoing[change.ID] {
			summary["pending"] = append(summary["pending"], change)
		}
	}
	// Now sort everything by ID.
	for _, reviews := range summary {
		sort.Slice(reviews, func(i, j int) bool {
			return reviews[i].ID > reviews[j].ID
		})
	}
	return summary, nil
}
