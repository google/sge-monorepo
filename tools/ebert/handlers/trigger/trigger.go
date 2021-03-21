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

// Package trigger handles Perforce triggers.
//
// Triggers are installed for actions like submit or shelve.  Ebert's response
// may vary based on the trigger action.  In many cases Ebert will update the
// internal state of a review, but Ebert may also enforce workflow rules by
// returning errors for triggers.
package trigger

import (
	"fmt"
	"net/http"
	"strconv"

	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/swarm"
	"sge-monorepo/tools/ebert/ebert"
	"sge-monorepo/tools/ebert/handlers/review"
)

// HandleTrigger handles trigger actions from Perforce.
func Handle(ectx *ebert.Context, r *http.Request, args *struct{ trigger string }) (interface{}, error) {
	switch args.trigger {
	case "submit":
		change, err := strconv.Atoi(r.FormValue("change"))
		if err != nil {
			return nil, fmt.Errorf("invalid change id in submit trigger: %w", err)
		}
		err = PostSubmit(ectx, change)
		if err != nil {
			return "error", err
		}
	default:
		return "error", fmt.Errorf("unhandled trigger: '%s'", args.trigger)
	}
	return "ok", nil
}

// PostSubmit processes submitted changes by updating associated reviews.
func PostSubmit(ctx *ebert.Context, change int) error {
	log.Infof("change %d submitted", change)

	reviews, err := swarm.GetReviewsForChangelists(&ctx.Swarm, []int{change})
	if err != nil {
		return fmt.Errorf("couldn't find reviews for %d: %w", change, err)
	}

	for _, r := range reviews.Reviews {
		annotated, err := review.AnnotateReview(ctx, &r)
		if err != nil {
			log.Errorf("annotate error: %v", err)
		}
		log.Infof("review %d BUGs=%v FIXes=%v", annotated.ID, annotated.Bugs, annotated.Fixes)
	}

	return nil
}
