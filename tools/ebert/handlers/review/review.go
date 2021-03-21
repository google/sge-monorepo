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

// Package review contains handlers for reviews.
package review

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"sge-monorepo/build/cicd/cirunner/protos/cirunnerpb"
	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/swarm"
	"sge-monorepo/tools/ebert/diff"
	"sge-monorepo/tools/ebert/ebert"
)

var clRegex = regexp.MustCompile(`^(?:" )?(\d+)(?: \/")?`)

func Handle(ctx *ebert.Context, r *http.Request, args *struct{ suffix string }) (interface{}, error) {
	user, err := ebert.UserFromRequest(r)
	if err != nil {
		return nil, err
	}

	matches := clRegex.FindStringSubmatch(args.suffix)
	if len(matches) < 1 {
		return nil, ebert.NewError(
			fmt.Errorf("review url malformed: %s : %w", args.suffix, err),
			fmt.Sprintf("Invalid path: %s", r.URL.Path),
			http.StatusBadRequest,
		)
	}

	suffix := matches[1]
	id, err := strconv.Atoi(suffix)
	if err != nil {
		return nil, ebert.NewError(
			fmt.Errorf("review:atoi: %w", err),
			fmt.Sprintf("Invalid path suffix: %s : %s", r.URL.Path, suffix),
			http.StatusBadRequest,
		)
	}

	review, shelved, err := fetchReview(ctx, id)
	if err != nil {
		return nil, err
	}

	version := 1
	cl := id
	pending := false

	if len(review.Versions) > 0 {
		version = len(review.Versions) - 1
		cl = review.Versions[version].Change
		pending = review.Versions[version].Pending
	}

	pairs, err := getFilePairs(ctx, 0, cl, pending, shelved)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"user":   user,
		"base":   0,
		"curr":   version + 1,
		"review": review,
		"pairs":  pairs,
	}, nil
}

func Approve(ctx *ebert.Context, r *http.Request, args *struct{ rid int }) (interface{}, error) {
	uctx, err := ctx.UserContext(r)
	if err != nil {
		return nil, fmt.Errorf("login error: %w", err)
	}

	review, err := swarm.SetState(&uctx.Swarm, args.rid, "approved")
	if err != nil {
		// Check if review is already approved.
		annotated, _, ferr := fetchReview(ctx, args.rid)
		if ferr != nil || annotated.State != "approved" {
			// Failed to get review or review wasn't approved, so return
			// the original error.
			return nil, err
		}
		review = annotated.Review
		// Ensure this user has upvoted the review.
		participant, ok := annotated.Participants[uctx.Swarm.Username]
		if !ok || participant.Vote.Value <= 0 || participant.Vote.IsStale {
			err = swarm.SetVote(&uctx.Swarm, args.rid, "up")
		}
	}
	return review, err
}

func Diff(ctx *ebert.Context, r *http.Request, args *struct {
	from     string
	to       string
	fileType string
	action   string
}) (interface{}, error) {
	from := args.from
	to := args.to
	fileType := args.fileType
	action := args.action
	if action == "move/delete" {
		depotFile := strings.Split(to, "@=")[0]
		depotFile = strings.Split(depotFile, "#")[0]
		return map[string]string{
			"response": fmt.Sprintf("-moved to %s\n", depotFile),
		}, nil
	}

	revs := []string{}
	if to != "" && !strings.Contains(action, "delete") {
		revs = append(revs, to)
	}
	if from != "" {
		revs = append(revs, from)
	}

	if len(revs) < 1 {
		return fmt.Sprintf("=nothing to diff for %s/%s - %s", from, to, action), nil
	}

	details, err := ctx.P4.PrintEx(revs...)
	if err != nil {
		return fmt.Sprintf("=diff failed: %v", err), err
	}
	if len(details) != len(revs) {
		return "=diff failed", fmt.Errorf("expected %d files, got %d", len(revs), len(details))
	}

	toContent := details[0].Content
	var fromContent []byte
	if len(details) > 1 {
		fromContent = details[1].Content
	}
	if strings.Contains(action, "delete") {
		// In general, 'from' is the second item in 'revs' and thus 'details'.
		// But in the case of a delete, there is no 'to', so 'revs' contains
		// only the 'from' revision, and thus details[0] is 'from', not 'to'
		// as initialized.  So for a delete, we swap from/to to show the correct
		// diff.
		fromContent, toContent = toContent, fromContent
	}

	var diff interface{}
	if strings.Contains(fileType, "binary") {
		diff, err = binaryDiff(ctx, fromContent, toContent)
	} else {
		diff, err = textDiff(ctx, fromContent, toContent)
	}

	if err != nil {
		return nil, ebert.NewError(
			fmt.Errorf("failed to build diffs from '%s' to '%s': %w", from, to, err),
			"Can't build diffs. Perhaps the change needs to be shelved?",
			http.StatusNotFound,
		)
	}
	return map[string]interface{}{"response": diff}, nil
}

func Pairs(ctx *ebert.Context, r *http.Request, args *struct {
	base        int
	curr        int
	currPending bool
}) (interface{}, error) {
	return getFilePairs(ctx, args.base, args.curr, args.currPending, true)
}

func TestRuns(ctx *ebert.Context, r *http.Request, args *struct {
	rid     int
	version int
}) (interface{}, error) {
	switch r.Method {
	case http.MethodGet:
		return swarm.TestRunDetails(&ctx.Swarm, args.rid, args.version)
	case http.MethodPost:
		return runTests(ctx, args.rid, args.version)
	default:
		return nil, fmt.Errorf("unexpected method %s", r.Method)
	}
}

func runTests(ctx *ebert.Context, rid, version int) (interface{}, error) {
	if ctx.Jenkins == nil {
		return nil, fmt.Errorf("can't connect to Jenkins")
	}

	// We actually want the 'raw' review, not the Swarm processed version since
	// we need the review 'token'.
	review, err := rawReview(ctx, rid)
	if err != nil {
		return nil, fmt.Errorf("couldn't fetch review %d: %w", rid, err)
	}
	if version <= 0 || version > len(review.Versions) {
		return nil, fmt.Errorf("can't run tests for invalid version %d of review %d", version, rid)
	}
	if len(review.Commits) > 0 {
		return nil, fmt.Errorf("won't run tests on committed review %d", rid)
	}
	uuid := fmt.Sprintf("%s.v%d", review.Token, version)

	// Create the testrun with the Swarm API.  This needs to happen before we
	// send the request to Jenkins since the testrun ID needs to be part of the
	// callback url.
	testrun, err := swarm.CreateTestRun(&ctx.Swarm, rid, version, uuid)
	if err != nil {
		return nil, fmt.Errorf("couldn't create testrun: %w", err)
	}

	// Start cicd test runner.
	change := review.Versions[version-1].Change
	// Note: The CI/CD runners may ignore the host part of the update URL, in
	// which case it doesn't matter what host is used.  If the host does matter,
	// we set Ebert as the host.  Ebert will forward to Swarm, so everything
	// will work as expected, plus we'll have the option of intercepting the
	// request if desired.
	swarmURL := fmt.Sprintf("https://INSERT_HOST/api/v10/testruns/%d/%s", testrun.ID, uuid)
	err = ctx.Jenkins.SendPresubmitRequest(&cirunnerpb.RunnerInvocation_Presubmit{
		Review:    int64(rid),
		Change:    int64(change),
		UpdateUrl: swarmURL,
	})
	if err != nil {
		return nil, fmt.Errorf("error triggering CI/CD: %w", err)
	}
	return testrun, nil
}

// HandleRest handles REST style Review requests.
func HandleRest(ctx *ebert.Context, r *http.Request, args *struct{ rid int }) (interface{}, error) {
	rid := args.rid

	if r.Method == http.MethodGet {
		review, _, err := fetchReview(ctx, rid)
		return review, err
	}

	if r.Method != http.MethodPatch {
		return nil, fmt.Errorf("unexpected %s %v", r.Method, r.URL)
	}

	var patch struct {
		swarm.ReviewPatch
		Bugs  []int `json:"bugs"`
		Fixes []int `json:"fixes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		return nil, fmt.Errorf("couldn't parse patch: %w", err)
	}

	uctx, err := ctx.UserContext(r)
	if err != nil {
		return nil, fmt.Errorf("login error: %w", err)
	}

	bugChan := make(chan error)
	defer close(bugChan)
	go func() {
		bugChan <- updateBugs(ctx, rid, patch.Bugs, patch.Fixes)
	}()

	review := &Review{}
	review.Review, err = swarm.PatchReview(&uctx.Swarm, rid, &patch.ReviewPatch)
	if err != nil {
		// May be patching CL, not review.  Check why UpdateDescription failed.
		var serr swarm.Error
		if !errors.As(err, &serr) || serr.Status() != http.StatusNotFound {
			return nil, err
		}
		// We don't actually update anything on fake reviews, but we do need
		// to return the review.
		if review, err = fakeReview(ctx, rid); err != nil {
			return nil, err
		}
	}

	bugErr := <-bugChan
	if err = annotateReview(ctx, review); err != nil {
		err = fmt.Errorf("multiple errors: %v %w", err, bugErr)
	} else {
		err = bugErr
	}

	return review, err
}

type Review struct {
	*swarm.Review

	Token string `json:"token"`
	Bugs  []int  `json:"bugs"`
	Fixes []int  `json:"fixes"`
	Fake  bool   `json:"fake"`
}

// Users returns a list of all p4 users.
func Users(ctx *ebert.Context, r *http.Request) (interface{}, error) {
	users, err := ctx.P4.Users()
	if err != nil {
		return nil, fmt.Errorf("error retrieving users: %w", err)
	}
	return &struct {
		Users []p4lib.User
	}{users}, nil
}

type fileRev struct {
	name string
	rev  int
	cl   int
}

func (f fileRev) String() string {
	if f.name == "" {
		return ""
	}
	if f.cl != 0 {
		return fmt.Sprintf("%s@=%d", f.name, f.cl)
	}
	return fmt.Sprintf("%s#%d", f.name, f.rev)
}
func (f fileRev) MarshalJSON() ([]byte, error) {
	json := fmt.Sprintf("\"%v\"", f)
	return []byte(json), nil
}
func (f fileRev) empty() bool {
	return f.name == ""
}

// FilePair defines an operation on a file.
type FilePair struct {
	From     fileRev // Starting revision.
	To       fileRev // Ending revision.
	FileType string  // Type of file, "text", "unicode", "utf8" or "binary".
	Action   string  // Action taken: "edit", "add", "delete", "move/add", or "move/delete"
	toDigest string  // Used to detect unchanged files.
}

// getFilePairs returns the set of FilePairs which describe the transformation
// from baseCl to currCl.  If baseCl is 0, use the previous submitted state.
// The currPending parameter is true if the corresponding cl has not been
// submitted.
func getFilePairs(ctx *ebert.Context, baseCl, currCl int, currPending, shelved bool) (map[string]*FilePair, error) {
	cls := []int{currCl}
	if baseCl != 0 {
		cls = append(cls, baseCl)
	}
	descs, err := func(cls []int, shelved bool) ([]p4lib.Description, error) {
		if shelved {
			return ctx.P4.DescribeShelved(cls...)
		}
		return ctx.P4.Describe(cls)
	}(cls, shelved)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve cl data: %v", err)
	}
	if len(descs) != len(cls) {
		return nil, fmt.Errorf("expected %d descs from %v, got %d", len(cls), cls, len(descs))
	}
	files := make(map[string]*FilePair)

	currDesc := &descs[0]
	// Set up filepairs based on curr CL.
	for _, fa := range currDesc.Files {
		rev := fa.Revision
		// If the CL has been submitted, perforce reports the updated revision.
		// In this case, we want "from" to be the prior revision.
		// So if currCl edited revision 3 of //some/file:
		//   if currCl is still pending, rev == 3
		//   if currCl is submitted, rev == 4, but we want to diff against 3
		if !currPending {
			rev = rev - 1
		}
		fromRev := fileRev{}
		if fa.FromFile != "" {
			// This is a move/add, so our diff base is FromFile#FromRev.
			fromRev = fileRev{name: fa.FromFile, rev: fa.FromRev}
		} else if fa.Action != "add" {
			// For any other action that isn't "add", assume the diff base
			// is DepotPath#rev.  If baseCl != 0, this will likely be updated
			// when processing baseCl.
			fromRev = fileRev{name: fa.DepotPath, rev: rev}
		}
		toRev := fileRev{name: fa.DepotPath, cl: currCl}
		files[fa.DepotPath] = &FilePair{
			From:     fromRev,
			To:       toRev,
			FileType: fa.Type,
			Action:   fa.Action,
			toDigest: fa.Digest,
		}
	}

	// If baseCL is non-zero, update our FilePairs.  Mostly this will be
	// changing From to indicate the revision from baseCl, but add/delete
	// may need to be adjusted as well.
	if len(descs) == 2 {
		baseDesc := &descs[1]
		for _, fa := range baseDesc.Files {
			pair, ok := files[fa.DepotPath]
			if !ok {
				// If a file exists in a previous version but not in the
				// later version, it must have been added then deleted,
				// or deleted then added.  Figure out which.
				if strings.Contains(fa.Action, "add") {
					// added at or before baseCl, then deleted before currCl,
					// so mark as 'delete'
					pair = &FilePair{
						Action: strings.ReplaceAll(fa.Action, "add", "delete"),
						From: fileRev{
							name: fa.DepotPath,
							cl:   baseCl,
						},
						FileType: fa.Type,
					}
				} else {
					// deleted at or before baseCl, then added back before
					// currCl, so mark as 'add'
					pair = &FilePair{
						Action: "add",
						From: fileRev{
							name: fa.DepotPath,
							rev:  fa.Revision,
						},
						To: fileRev{
							name: fa.DepotPath,
							cl:   currCl,
						},
					}
					if fa.FromFile != "" {
						pair.From = fileRev{
							name: fa.FromFile,
							rev:  fa.FromRev,
						}
						pair.Action = strings.ReplaceAll(fa.Action, "delete", "add")
					}
				}
				pair.FileType = fa.Type
				files[fa.DepotPath] = pair
			} else {
				// File exists in both versions.  If the digests are the same,
				// there are no changes and we can drop this file from the
				// set of diffs.
				if pair.toDigest == fa.Digest {
					delete(files, fa.DepotPath)
					continue
				}
				// An action on this depot file exists in the current CL.
				// Expected combinations are:
				//   base-action    curr-action
				//   [move/]add     [move/]add
				//   [move/]delete  [move/]delete
				//   edit           edit
				//   [move/]delete  edit
				//   edit           [move/]delete
				expected := pair.Action == fa.Action
				expected = expected || (strings.Contains(fa.Action, "delete") && pair.Action != "edit")
				expected = expected || (strings.Contains(pair.Action, "delete") && fa.Action != "edit")
				if !expected {
					log.Infof("unexpected action mismatch for %s: %s != %s", fa.DepotPath, fa.Action, pair.Action)
				}
				if !strings.Contains(fa.Action, "delete") {
					pair.From = fileRev{
						name: fa.DepotPath,
						cl:   baseCl,
					}
					if !strings.Contains(pair.Action, "delete") {
						pair.Action = "edit"
					}
				}
			}
		}
	}

	// Fixup move pairs
	for _, pair := range files {
		if strings.Contains(pair.Action, "move") {
			if strings.Contains(pair.Action, "add") {
				if file, ok := files[pair.From.name]; ok {
					if file.Action != "move/delete" {
						log.Warningf("expected move/delete for %s, got %s", pair.From.name, file.Action)
					}
					file.To = pair.To
				}
			}
		}
	}
	// Final pass to ensure valid data.
	for _, pair := range files {
		if pair.From.empty() {
			pair.Action = "add"
		}
	}

	return files, nil
}

func textDiff(ctx *ebert.Context, from, to []byte) (string, error) {
	diff, err := diff.Compute(from, to)
	if err != nil {
		return fmt.Sprintf("=diff failed: %v", err), err
	}
	if diff == "" {
		// Both files are empty.  If we're dealing with an add, from will be
		// nil, if we're dealing with a delete, to will be nil.
		switch {
		case from == nil:
			diff = "+<empty file>"
		case to == nil:
			diff = "-<empty file>"
		default:
			diff = "=<empty file>"
		}
	}
	return diff, nil
}

func binaryDiff(ctx *ebert.Context, from, to []byte) (interface{}, error) {
	fromType := http.DetectContentType(from)
	toType := http.DetectContentType(to)
	if (len(from) > 0 && !strings.HasPrefix(fromType, "image")) || (len(to) > 0 && !strings.HasPrefix(toType, "image")) {
		if bytes.Compare(from, to) == 0 {
			return "=Binary files are identical.", nil
		}
		switch {
		case from == nil:
			return fmt.Sprintf("+<binary file (%d bytes)>", len(to)), nil
		case to == nil:
			return fmt.Sprintf("-<binary file (%d bytes)>", len(from)), nil
		default:
			return fmt.Sprintf("-Binary files differ (%d bytes).\n+Binary files differ (%d bytes).", len(from), len(to)), nil
		}
	}
	response := map[string]string{}
	if len(from) > 0 {
		response["from"] = fmt.Sprintf("data:%s;base64,%s", fromType, base64.StdEncoding.EncodeToString(from))
	}
	if len(to) > 0 {
		response["to"] = fmt.Sprintf("data:%s;base64,%s", toType, base64.StdEncoding.EncodeToString(to))
	}
	return response, nil
}

var (
	bugRE       = regexp.MustCompile(`^(BUG=|FIX=)`)
	bugurlRE = regexp.MustCompile(`(?:https://)?(?:b/)?(\d+)`)
)

func parseBugs(line string) ([]int, error) {
	var ids []int
	for _, item := range strings.Split(line, ",") {
		item := strings.TrimSpace(item)
		matches := bugurlRE.FindStringSubmatch(item)
		if len(matches) == 2 && matches[1] != "" {
			id, err := strconv.Atoi(matches[1])
			if err != nil {
				return nil, err
			}
			ids = append(ids, id)
		} else {
			return nil, fmt.Errorf("missing bug id in %s", item)
		}
	}
	return ids, nil
}

// AnnotateReview converts a raw Swarm Review to an Ebert Review.
func AnnotateReview(ctx *ebert.Context, review *swarm.Review) (*Review, error) {
	r := &Review{
		Review: review,
	}
	err := annotateReview(ctx, r)
	return r, err
}

func annotateReview(ctx *ebert.Context, review *Review) error {
	bugs, fixes, err := bugsFromAux(ctx, review.ID, review.Description)
	if err != nil {
		return err
	}
	review.Bugs = bugs
	review.Fixes = fixes

	return nil
}

// aux is for holding auxiliary information for a review -- information that
// we want associated with the review, but doesn't fit into Swarm's schema.
type aux struct {
	Bugs  []int
	Fixes []int
}

func reviewAux(ctx *ebert.Context, key string) (*aux, string, error) {
	raw, err := ctx.P4.KeyGet(key)
	var a aux
	// "0" indicates not found.
	if err != nil || raw == "0" {
		return &a, "", err
	}
	if err = json.Unmarshal([]byte(raw), &a); err != nil {
		return &a, raw, fmt.Errorf("unmarshal(%s) error: %w", raw, err)
	}
	return &a, raw, err
}

func bugsFromAux(ctx *ebert.Context, rid int, description string) ([]int, []int, error) {
	a, _, err := reviewAux(ctx, auxKeyForReview(rid))
	if err != nil {
		return nil, nil, err
	}

	bugs, fixes := bugsFromDescription(description)
	bugs = mergeIds(bugs, a.Bugs)
	fixes = mergeIds(fixes, a.Fixes)
	return bugs, fixes, err
}

func updateBugs(ctx *ebert.Context, rid int, bugs, fixes []int) error {
	key := auxKeyForReview(rid)
	// Update auxiliary info transactionally, retrying up to 3 times.
	for i := 0; i < 3; i++ {
		a, orig, err := reviewAux(ctx, key)
		// Unfortunately there's no way to distinguish "0" from not found.
		unset := orig == ""
		if err != nil && !unset {
			return err
		}

		a.Bugs = bugs
		a.Fixes = fixes

		updated, err := json.Marshal(&a)
		if err != nil {
			return err
		}

		if unset {
			// KeyCas doesn't work unless the key already had a value, so just
			// use KeySet.  This does introduce a race condition on the first
			// write to a key.
			return ctx.P4.KeySet(key, string(updated))
		}
		err = ctx.P4.KeyCas(key, orig, string(updated))
		if err == p4lib.ErrCasMismatch {
			select {
			case <-ctx.Ctx.Done():
				return fmt.Errorf("update bugs context done")
			default:
			}
			continue
		}
		return err
	}
	return fmt.Errorf("update bugs too many retries")
}

func mergeIds(fromDesc, fromKeys []int) []int {
	merged := append([]int{}, fromDesc...)
next:
	for _, x := range fromKeys {
		for _, y := range fromDesc {
			if x == y {
				continue next
			}
		}
		merged = append(merged, x)
	}
	return merged
}

func auxKeyForReview(id int) string {
	return fmt.Sprintf("ebert-review-aux-%x", 0xffffffff-id)
}

func fetchReview(ctx *ebert.Context, id int) (*Review, bool, error) {
	shelved := true // All CLs that are part of Swarm reviews are shelved.
	r := &Review{}
	var err error
	r.Review, err = swarm.GetReview(&ctx.Swarm, id)
	if err != nil {
		var rc swarm.ReviewCollection
		ids := []int{id}
		// if no error we have a valid review and retrieve review from swarm reviews
		// otherwise we fall through and use original CL
		if rc, err = swarm.GetReviewsForChangelists(&ctx.Swarm, ids); err == nil && len(rc.Reviews) > 0 {
			r.Review, err = swarm.GetReview(&ctx.Swarm, rc.Reviews[0].ID)
		} else if len(rc.Reviews) <= 0 {
			err = fmt.Errorf("no reviews found for %v", ids)
		}
	}
	if err != nil {
		log.Warningf("swarm.GetReview: %v, constructing from /cl/%d", err, id)
		fake, err := fakeReview(ctx, id)
		if err != nil {
			return nil, false, ebert.NewError(
				err,
				fmt.Sprintf("No review or change numbered %d", id),
				http.StatusNotFound,
			)
		}
		r = fake
		shelved = false
	}

	annotateReview(ctx, r)
	return r, shelved, nil
}

func keyForReview(id int) string {
	return fmt.Sprintf("swarm-review-%x", 0xffffffff-id)
}

func rawReview(ctx *ebert.Context, id int) (*Review, error) {
	raw, err := ctx.P4.KeyGet(keyForReview(id))
	if err != nil {
		return nil, err
	}
	var r Review
	if err = json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func bugsFromDescription(description string) ([]int, []int) {
	// Extract bug info from description.
	lines := strings.Split(description, "\n")
	var bugs, fixes []int
	for _, line := range lines {
		if match := bugRE.FindString(line); match != "" {
			ids, err := parseBugs(line[len(match):])
			if err != nil {
				// Don't fail the function if we can't parse bugs.
				log.Warningf("error parsing bug ids: %v", err)
			}
			if match == "BUG=" {
				bugs = append(bugs, ids...)
			}
			if match == "FIX=" {
				fixes = append(fixes, ids...)
			}
			continue
		}
	}

	return bugs, fixes
}

// fakeReview builds a swarm.Review from a change description.  The purpose
// is to allow reviewing pending CLs in Ebert, leveraging the existing review
// frontend.
// Why not just build a new page for changes that's different from reviews?
// 1. That's not what Critique does.  Critique shows the review page for
//    pending CLs, even with no reviewers, etc.
// 2. Examining a CL is almost the same as a review -- some of the boxes are
//    empty or non-functional, but it's mostly the same, so building a custom
//    UI doesn't feel like the right bang for the buck.
// That said, if/when I ever figure out a real build step for the frontend
// that enables reusing components across multiple pages, we might revisit this.
func fakeReview(ctx *ebert.Context, cl int) (*Review, error) {
	descs, err := ctx.P4.DescribeShelved(cl)
	if err != nil {
		return &Review{}, err
	}
	if len(descs) != 1 {
		return &Review{}, fmt.Errorf("expected 1 cl, got %d", len(descs))
	}
	desc := descs[0]
	return &Review{
		Review: &swarm.Review{
			ID:          desc.Cl,
			Author:      desc.User,
			Description: desc.Description,
			Created:     int(desc.DateUnix),
			Updated:     int(desc.DateUnix),
			Pending:     desc.Status == "pending",
			Changes:     []int{desc.Cl},
			Commits:     []int{},
			Versions: []swarm.Version{
				{
					Change:  desc.Cl,
					User:    desc.User,
					Time:    int(desc.DateUnix),
					Pending: desc.Status == "pending",
				},
			},
		},
		Fake: true,
	}, nil
}
