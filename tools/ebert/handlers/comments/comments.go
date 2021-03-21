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

// Package comments contains the handler for comments.
package comments

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/swarm"
	"sge-monorepo/tools/ebert/ebert"
)

const (
	ebertDraftCommentKeyFmt       = "ebert-draft-comments-%v-%v-%v"
	ebertDraftCommentsCountKeyFmt = "ebert-draft-comments-%v-%v:count"
)

func Handle(ctx *ebert.Context, r *http.Request, args *struct {
	rid     int
	cid     int
	publish bool
}) (interface{}, error) {
	rid := args.rid
	user, err := ebert.UserFromRequest(r)
	if err != nil {
		return nil, fmt.Errorf("couldn't determine user: %w", err)
	}

	switch r.Method {
	case http.MethodGet:
		// GET is for retrieving comments.  Right now we only return
		// all comments, but in the future we might examine the path
		// and only return specific comments.
		return getComments(ctx, user, rid)
	case http.MethodPost, http.MethodPatch:
		// POST is for creating new comments.
		// PATCH is for editing draft comments.
		var comment struct {
			swarm.Comment
			LGTM    bool `json:"lgtm"`
			Approve bool `json:"approve"`
		}
		err = json.NewDecoder(r.Body).Decode(&comment)
		if err != nil {
			return nil, fmt.Errorf("couldn't decode comment: %w", err)
		}
		if r.Method == http.MethodPatch {
			return editComment(ctx, &comment.Comment, user, rid, args.cid)
		}
		// Channel has room for 2 errors, in case both approve and lgtm fail.
		ech := make(chan error, 2)
		// Do approval/upvote concurrently.
		if args.publish && (comment.LGTM || comment.Approve) {
			go func() {
				uctx, err := ctx.Login(user)
				if err != nil {
					ech <- fmt.Errorf("error logging in %s: %w", user, err)
					return
				}
				if comment.Approve {
					_, err := swarm.SetState(&uctx.Swarm, rid, "approved")
					if err != nil {
						err = fmt.Errorf("error approving %d: %v", rid, err)
					}
					ech <- err
					if err == nil {
						// Approve also upvotes, so we don't need to check LGTM
						// on success, but we fallthrough and check on failure.
						return
					}
				}
				if comment.LGTM || comment.Approve {
					err = swarm.SetVote(&uctx.Swarm, rid, "up")
					if err != nil {
						err = fmt.Errorf("error upvoting %d: %v", rid, err)
					}
					ech <- err
				}
			}()
		} else {
			ech <- nil
		}
		r, err := addComment(ctx, &comment.Comment, user, rid, args.publish)
		bgErr := <-ech
		if err != nil {
			return nil, fmt.Errorf("couldn't post comment: %w", err)
		}
		if bgErr != nil {
			return nil, bgErr
		}
		return r, nil
	case http.MethodDelete:
		// DELETE is for deleting (draft) comments.
		return deleteComment(ctx, user, rid, args.cid)
	}
	return nil, fmt.Errorf("unexpected method: %s", r.Method)
}

func getComments(ctx *ebert.Context, user string, rid int) (*swarm.CommentCollection, error) {
	type asyncComments struct {
		comments *swarm.CommentCollection
		err      error
	}
	draftCh := make(chan asyncComments)
	go func() {
		comments, err := getDraftComments(ctx, user, rid)
		draftCh <- asyncComments{
			comments: comments,
			err:      err,
		}
	}()
	comments, err := swarm.GetCommentsForReview(&ctx.Swarm, rid)
	drafts := <-draftCh
	if drafts.err != nil {
		// We don't want to not return comments just because of an error
		// retrieving drafts, so log the error and continue.
		log.Warningf("getComments drafts.err: %v", err)
	}
	if drafts.comments != nil {
		comments.Comments = append(comments.Comments, drafts.comments.Comments...)
	}
	for _, c := range comments.Comments {
		if c.Context.Review == 0 {
			log.Warningf("no review id set for comment %d", c.ID)
			c.Context.Review = rid
		}
	}

	return &comments, err
}

func getDraftComments(ctx *ebert.Context, user string, rid int) (*swarm.CommentCollection, error) {
	topic := fmt.Sprintf("reviews-%d", rid)
	draftPattern := fmt.Sprintf(ebertDraftCommentKeyFmt, user, topic, "*")
	drafts, err := ctx.P4.Keys(draftPattern)
	if err != nil {
		return nil, fmt.Errorf("error retrieving draft comments: %v", err)
	}
	keys := make([]string, 0, len(drafts))
	for k := range drafts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	comments := &swarm.CommentCollection{}
	comments.Comments = make([]swarm.Comment, 0, len(keys))
	errs := make([]string, 0, len(keys))
	for _, k := range keys {
		var comment swarm.Comment
		if err := json.Unmarshal([]byte(drafts[k]), &comment); err != nil {
			errs = append(errs, fmt.Sprintf("(%s): %v", k, err))
			continue
		}
		comments.Comments = append(comments.Comments, comment)
	}
	if len(errs) > 0 {
		return comments, fmt.Errorf("unmarshal errors: %v", strings.Join(errs, ", "))
	}
	return comments, nil
}

func addComment(ctx *ebert.Context, comment *swarm.Comment, user string, rid int, publish bool) (interface{}, error) {
	if comment.Topic == "" {
		comment.Topic = fmt.Sprintf("reviews/%d", rid)
		log.Warningf("comment topic not set, using %s", comment.Topic)
	}

	if publish {
		return publishComments(ctx, comment, user, rid)
	}

	// Turn "reviews/<id>" into "reviews-<id>"
	topic := strings.ReplaceAll(comment.Topic, "/", "-")
	countKey := fmt.Sprintf(ebertDraftCommentsCountKeyFmt, user, topic)

	next, err := ctx.P4.KeyInc(countKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create draft id: %v", err)
	}
	id, err := strconv.Atoi(strings.TrimSpace(next))
	if err != nil {
		return nil, fmt.Errorf("'%v' isn't an int: %v", next, err)
	}
	padded := fmt.Sprintf("%010v", next)
	draftKey := fmt.Sprintf(ebertDraftCommentKeyFmt, user, topic, padded)
	// Draft comments get a negative id so the front end can easily
	// distinguish them.
	comment.ID = -id
	comment.User = user
	if comment.Context == nil {
		comment.Context = &swarm.CommentContext{}
	}
	comment.Context.Review = rid

	// Swarm encodes comment values as JSON with some custom escaping.
	// Go's JSON encoder can only do some of the encoding (in particular
	// it doesn't encode ' the same way (Swarm encodes it as \u0027),
	// and though I haven't figured out where it's happening, it also
	// seems to be escaping forward slashes.  Go's json encoder can't be
	// configured to replicate this encoding, so for now I'm hoping it
	// doesn't actually matter.
	payload, err := json.Marshal(comment)
	if err != nil {
		return nil, fmt.Errorf("couldn't encode draft comment: %v", err)
	}

	err = ctx.P4.KeySet(draftKey, string(payload))
	if err != nil {
		return nil, fmt.Errorf("couldn't write draft comment: %v", err)
	}
	// Success
	return comment, nil
}

// editComment update's a draft comment, calling on a published comment results
// in an error. Only the comment body and resolved state are updated, the
// remaining metadata is unchanged.
// Draft comments are stored in p4 keys as JSON data, so updating a comment
// requires reading & decoding the existing comment, updating the body, then
// encoding and writing the modified comment.
func editComment(ctx *ebert.Context, comment *swarm.Comment, user string, rid, cid int) (*swarm.Comment, error) {
	if cid >= 0 {
		return &swarm.Comment{}, fmt.Errorf("cannot edit published comments")
	}
	if comment.Topic == "" {
		comment.Topic = fmt.Sprintf("reviews/%d", rid)
	}

	// Turn "reviews/<id>" into "reviews-<id>"
	topic := strings.ReplaceAll(comment.Topic, "/", "-")
	padded := fmt.Sprintf("%010v", -cid)
	draftKey := fmt.Sprintf(ebertDraftCommentKeyFmt, user, topic, padded)

	oldComment, err := ctx.P4.KeyGet(draftKey)
	if err != nil || oldComment == "0" {
		return nil, fmt.Errorf("error reading comment '%v': %v", draftKey, err)
	}
	// Preserve comment body and resolved state from request.
	body := comment.Body
	resolved := false
	for _, flag := range comment.Flags {
		if flag == "resolved" {
			resolved = true
			break
		}
	}
	// Replace comment with original.
	if err = json.Unmarshal([]byte(oldComment), comment); err != nil {
		log.Infof("failed to parse: '%v'", oldComment)
		return nil, fmt.Errorf("error unmarshalling comment: %v", err)
	}
	// Apply comment body and resolved state from request.
	comment.Body = body
	wasResolved := -1
	for i, flag := range comment.Flags {
		if flag == "resolved" {
			wasResolved = i
			break
		}
	}
	if resolved && wasResolved < 0 {
		// Add 'resolved' flag.
		comment.Flags = append(comment.Flags, "resolved")
	} else if !resolved && wasResolved >= 0 {
		// Swap the resolved flag with the last flag.
		last := len(comment.Flags) - 1
		comment.Flags[last], comment.Flags[wasResolved] = comment.Flags[wasResolved], comment.Flags[last]
		// Now drop the last flag.
		comment.Flags = comment.Flags[0:last]
	}

	payload, err := json.Marshal(comment)
	if err != nil {
		return nil, fmt.Errorf("couldn't encode draft comment: %v", err)
	}

	err = ctx.P4.KeySet(draftKey, string(payload))
	if err != nil {
		return nil, fmt.Errorf("couldn't update draft comment: %v", err)
	}
	// Success
	return comment, nil
}

func swarmCommentKey(cid int) string {
	return fmt.Sprintf("swarm-comment-%010d", cid)
}

// MarkRead bypasses the Swarm API to update the comment's 'readBy' field
// directly.  This is necessary because the Swarm API does not provide any
// mechanism to update a comment's readBy field.
// To make this as safe as possible, we use check-and-set semantics.  That is,
// we get the latest version of the comment, modify the 'readBy' field, but
// only write back if the comment has not changed in the meantime.  If the
// comment has changed, we retry the entire operation.
func MarkRead(ctx *ebert.Context, r *http.Request, args *struct{ cid int }) (interface{}, error) {
	cid := args.cid
	if cid < 0 {
		return nil, fmt.Errorf("Can't mark draft comments as read")
	}

	user, err := ebert.UserFromRequest(r)
	if err != nil {
		return nil, fmt.Errorf("Can't identify user: %v", err)
	}

	key := swarmCommentKey(cid)
	for {
		raw, err := ctx.P4.KeyGet(key)
		if err != nil {
			return nil, err
		}
		var obj map[string]interface{}
		var readBy struct {
			ReadBy []string `json:"readBy"`
		}
		if err := json.Unmarshal([]byte(raw), &obj); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(raw), &readBy); err != nil {
			return nil, err
		}
		for _, reader := range readBy.ReadBy {
			if reader == user {
				return struct{}{}, nil
			}
		}
		obj["readBy"] = append(readBy.ReadBy, user)
		updated, err := json.Marshal(obj)
		if err != nil {
			return nil, err
		}
		if err := ctx.P4.KeyCas(key, raw, string(updated)); err != nil {
			if err == p4lib.ErrCasMismatch {
				continue
			}
			return nil, err
		}
		break
	}
	return struct{}{}, nil
}

func deleteComment(ctx *ebert.Context, user string, rid, cid int) (int, error) {
	if cid >= 0 {
		return 0, fmt.Errorf("can only delete draft comments, id = %d", cid)
	}
	padded := fmt.Sprintf("%010v", -cid)
	topic := fmt.Sprintf("reviews-%v", rid)
	draftKey := fmt.Sprintf(ebertDraftCommentKeyFmt, user, topic, padded)

	_, err := ctx.P4.ExecCmd("key", "-d", draftKey)
	if err != nil {
		return 0, fmt.Errorf("couldn't delete draft comment: %v", err)
	}
	return cid, nil
}

type commentUpdates struct {
	// Comments holds the published comments.
	Comments []swarm.Comment `json:"comments"`
	// Drafts holds the comment IDs of successfully published drafts.
	Drafts []int `json:"drafts"`
	// Message holds any message from Swarm regarding notifications.
	Message string `json:"message"`
}

type errors []error

func (errs errors) Error() string {
	msgs := make([]string, 0, len(errs))
	for _, err := range errs {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("errors:\n    %s", strings.Join(msgs, "\n    "))
}

func publishComments(ctx *ebert.Context, comment *swarm.Comment, user string, rid int) (*commentUpdates, error) {
	uctx, err := ctx.Login(user)
	if err != nil {
		return nil, fmt.Errorf("couldn't login user %s: %v", user, err)
	}

	publish, err := getDraftComments(ctx, user, rid)
	if err != nil {
		// Don't fail the operation for an error retrieving drafts.
		// Log and continue.
		log.Warningf("publishComments get drafts: %v", err)
	}

	if comment.Body != "" {
		// Add the comment to the collection of comments to publish.
		if comment.Context == nil {
			comment.Context = &swarm.CommentContext{}
		}
		comment.Context.Review = rid
		publish.Comments = append(publish.Comments, *comment)
	}

	published := &commentUpdates{}
	published.Comments = make([]swarm.Comment, 0, len(publish.Comments))
	var errs errors
	lock := &sync.Mutex{}
	wg := &sync.WaitGroup{}
	for _, c := range publish.Comments {
		wg.Add(1)
		go func(comment swarm.Comment) {
			defer wg.Done()
			cid := comment.ID
			comment.ID = 0
			added, err := swarm.AddCommentEx(&uctx.Swarm, &comment, true)
			if err == nil && cid < 0 {
				// Succesfully published a draft comment, so delete the draft.
				_, err = deleteComment(ctx, user, rid, cid)
			}

			lock.Lock()
			defer lock.Unlock()
			if err != nil {
				log.Warningf("failed publishing comment: %v", err)
				errs = append(errs, err)
				return
			}
			if cid < 0 {
				published.Drafts = append(published.Drafts, cid)
			}
			published.Comments = append(published.Comments, *added)
		}(c)
	}

	wg.Wait()
	msg, err := swarm.SendNotifications(&uctx.Swarm, rid)
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to send notifications: %v(%s)", err, msg))
	}
	published.Message = msg
	if len(errs) != 0 {
		return published, errs
	}
	return published, nil
}
