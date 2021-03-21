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

package swarmlib

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"reflect"
	"sort"
	"strings"
	"time"

    "sge-monorepo/libs/go/log"
)

const (
	formEncoded = "application/x-www-form-urlencoded"
	jsonEncoded = "application/json"
)

type Context struct {
	Host     string // base address e.g. https://foo.dev/
	Port     int    // port e.g. 9000
	Username string // Credentials.
	Password string // Credentials.

	// Client is an HTTP client to be used for http request.
	// Users of the library can set it to override the default one.
	Client *http.Client
	Ctx    context.Context
}

type impl struct {
    ctx *Context
}

// BuildUrl constructs a swarm url based on host, port and endpoint.
func (i *impl) BuildUrl(endpoint string) string {
	url := fmt.Sprintf("%s:%d/%s", i.ctx.Host, i.ctx.Port, endpoint)

	// golang's url parser gets tripped up if the url doesn't begin with http(s)://. This
	// might happen is the Host provided to Context is just the host part.
	if !strings.HasPrefix(url, "http") {
		url = fmt.Sprintf("https://%s", url)
	}
	return url
}

func (i *impl) Context() *Context {
    return i.ctx
}

// GetReview returns a swarm review identified by |id|.
func (i *impl) GetReview(id int) (*Review, error) {
	endpoint := fmt.Sprintf("api/v9/reviews/%d", id)
	// The response wraps the review in a JSON object with a "review" key.
	msg := struct {
		Review *Review
	}{}
	if err := i.doSwarmRequestResponse("GET", endpoint, nil, &msg); err != nil {
		return nil, err
	}
	return msg.Review, nil
}

func (i *impl) getReviewsPage(after int, args string) (ReviewCollection, error) {
	var rc ReviewCollection

	endpoint := "api/v9/reviews"
	if after != 0 {
		if len(args) > 0 {
			args += "&"
		}
		args += fmt.Sprintf("after=%d", after)
	}
	if len(args) > 0 {
		endpoint += "?" + args
	}

	if err := i.doSwarmRequestResponse("GET", endpoint, nil, &rc); err != nil {
		return rc, fmt.Errorf("swarm.getReviewsPage %v", err)
	}
	return rc, nil
}

// GetReviews returns a collection of swarm reviews
func (i *impl) GetReviews(args string) (ReviewCollection, error) {
	var rc ReviewCollection

	page, err := i.getReviewsPage(0, args)
	if err != nil {
		return rc, err
	}
	for len(page.Reviews) > 0 {
		rc.Reviews = append(rc.Reviews, page.Reviews...)
		page, err = i.getReviewsPage(page.LastSeen, args)
		if err != nil {
			return rc, err
		}
	}
	return rc, nil
}

// GetReviewsForChangelists returns a colllection containing reviews for all specified changelists
func (i *impl) GetReviewsForChangelists(changeLists []int) (ReviewCollection, error) {
	var rc ReviewCollection
	endpoint := ""
	for _, c := range changeLists {
		if len(endpoint) > 0 {
			endpoint += "&"
		}
		endpoint += fmt.Sprintf("change[]=%d", c)
		if len(endpoint) > 8000 {
			r, err := i.GetReviews(endpoint)
			if err != nil {
				return rc, err
			}
			rc.Reviews = append(rc.Reviews, r.Reviews...)
			endpoint = ""
		}
	}
	if len(endpoint) > 0 {
		r, err := i.GetReviews(endpoint)
		if err != nil {
			return rc, err
		}
		rc.Reviews = append(rc.Reviews, r.Reviews...)
	}
	return rc, nil
}

// GetOpenReviews returns a colllection containing reviews for all specified changelists
func (i *impl) GetOpenReviews(username string) (ReviewCollection, error) {
	return i.GetReviews(fmt.Sprintf("participants=%s&state=needsReview", username))
}

func (i *impl) getCommentsPage(after int, args string) (CommentCollection, error) {
	var cc CommentCollection
	endpoint := "api/v9/comments"
	if after != 0 {
		if len(args) > 0 {
			args += "&"
		}
		args += fmt.Sprintf("after=%d", after)
	}
	if len(args) > 0 {
		endpoint += "?" + args
	}
	if err := i.doSwarmRequestResponse("GET", endpoint, nil, &cc); err != nil {
		return cc, fmt.Errorf("swarm.getCommentsPage %v", err)
	}
	return cc, nil
}

// GetComments returns a collection of comments
func (i *impl) GetComments(args string) (CommentCollection, error) {
	var cc CommentCollection

	page, err := i.getCommentsPage(0, args)
	if err != nil {
		return cc, err
	}
	for len(page.Comments) > 0 {
		cc.Comments = append(cc.Comments, page.Comments...)
		page, err = i.getCommentsPage(page.LastSeen, args)
		if err != nil {
			return cc, err
		}
	}
	return cc, nil
}

// GetCommentsForReview returns details about comments for specified review
func (i *impl) GetCommentsForReview(reviewIndex int) (CommentCollection, error) {
	return i.GetComments(fmt.Sprintf("topic=reviews/%d", reviewIndex))
}

// UpdateComment updates a comment in a review
// https://www.perforce.com/manuals/swarm/Content/Swarm/swarm-apidoc_endpoint_comments.html#Edit_a_Comment
func (i *impl) UpdateComment(comment *Comment) error {
	endpoint := fmt.Sprintf("api/v9/comments/%d", comment.ID)
	scu := CommentUpdate{
		Body:  comment.Body,
		ID:    comment.ID,
		Topic: comment.Topic,
		Flags: comment.Flags,
	}
	if err := i.doSwarmRequestResponse("PATCH", endpoint, scu, nil); err != nil {
		return fmt.Errorf("swarm.UpdateComment %v", err)
	}
	return nil
}

// AddComment adds a comment to a review
// https://www.perforce.com/manuals/swarm/Content/Swarm/swarm-apidoc_endpoint_comments.html#Edit_a_Comment
func (i *impl) AddComment(comment *Comment) error {
	_, err := i.AddCommentEx(comment, false)
	return err
}
func (i *impl) AddCommentEx(comment *Comment, delayNotification bool) (*Comment, error) {
	sca := CommentAdd{
		Body:    comment.Body,
		Topic:   comment.Topic,
		Context: &CommentAddContext{},
		Flags:   comment.Flags,
	}
	if delayNotification {
		sca.DelayNotification = "true"
	}
	if comment.Context != nil {
		sca.Context.File = comment.Context.File
		sca.Context.LeftLine = comment.Context.LeftLine
		sca.Context.RightLine = comment.Context.RightLine
		sca.Context.Comment = comment.Context.Comment
		sca.Context.Content = comment.Context.Content
	}

	// We want to avoid the comment having a null context, but we need either
	// File or Content to be set, or Swarm will reject the comment.  For
	// review level comments or replies, we don't have either, so assign
	// dummy Content.
	if sca.Context.File == "" && len(sca.Context.Content) == 0 {
		sca.Context.Content = []string{"^--"}
	}

	var response struct {
		Comment Comment `json:"comment"`
		Error   string  `json:"error"`
		Details struct {
			Context string `json:"context"`
		}
	}
	if err := i.doSwarmRequestResponse("POST", "api/v9/comments", sca, &response); err != nil {
		return nil, fmt.Errorf("swarm.AddCommentEx %v", err)
	}
	if response.Error != "" {
		return nil, fmt.Errorf("swarm.AddCommentEx %s : %s", response.Error, response.Details.Context)
	}

	return &response.Comment, nil
}

// SendNotifications tells Swarm to send notifications for the specified review.
// Returns any informational message from Swarm, or an error on failure.
func (i *impl) SendNotifications(review int) (string, error) {
	notify := url.Values{
		"topic": []string{fmt.Sprintf("reviews/%d", review)},
	}
	payload := []byte(notify.Encode())
	resp, err := i.doSwarmRequest("POST", "api/v9/comments/notify", formEncoded, payload)
	if err != nil {
		return "", fmt.Errorf("swarm.SendNotifications %v", err)
	}
	var response struct {
		IsValid bool   `json:"isValid"`
		Message string `json:"message"`
		Error   string `json:"error"`
	}
	if err := json.Unmarshal(resp, &response); err != nil {
		return "", fmt.Errorf("swarm.SendNotifications unmarshal: %v", err)
	}
	if !response.IsValid {
		return response.Message, fmt.Errorf("swarm.SendNotifications %v", response.Error)
	}
	return response.Message, nil
}

// SetVote sets the vote for the logged in user on the specified review.
// The 'vote' parameter should be 'up', 'down', or 'clear'.
func (i *impl) SetVote(review int, vote string) error {
	var v struct {
		Vote struct {
			Value string `json:"value"`
		} `json:"vote"`
	}
	v.Vote.Value = vote
	var response struct {
		IsValid  bool        `json:"isValid"`
		Messages interface{} `json:"messages"`
	}
	if err := i.doSwarmRequestResponse("POST", fmt.Sprintf("api/v9/reviews/%d/vote", review), v, &response); err != nil {
		return fmt.Errorf("swarm.SetVote %v", err)
	}
	return nil
}

// PatchReview updates the specified review.
func (i *impl) PatchReview(review int, patch *ReviewPatch) (*Review, error) {
	var response struct {
		Review *Review `json:"review"`
	}
	if err := i.doSwarmRequestResponse("PATCH", fmt.Sprintf("api/v9/reviews/%d", review), patch, &response); err != nil {
		return nil, fmt.Errorf("swarm.PatchReview: %w", err)
	}
	if response.Review == nil {
		return nil, fmt.Errorf("swarm.PatchReview invalid response")
	}
	return response.Review, nil
}

// UpdateDescription updates the description for the specified review.
func (i *impl) UpdateDescription(review int, description string) (*Review, error) {
	return i.PatchReview(review, &ReviewPatch{
		Description: &description,
	})
}

func (i *impl) SetState(review int, state string) (*Review, error) {
	v := map[string]string{
		"state":       state,
		"description": fmt.Sprintf("Review %d has been %s by %s.", review, state, i.ctx.Username),
	}
	var response struct {
		Review *Review `json:"review"`
	}
	if err := i.doSwarmRequestResponse("PATCH", fmt.Sprintf("api/v9/reviews/%d/state/", review), v, &response); err != nil {
		return nil, fmt.Errorf("swarm.SetState: %w", err)
	}
	if response.Review == nil {
		return nil, fmt.Errorf("swarm.SetState invalid response")
	}
	return response.Review, nil
}

// BallotBuild builds details about all votes for a review
func (review *Review) BallotBuild() Ballot {
	var b Ballot
	for k, v := range review.Participants {
		if k != review.Author {
			b.Entries = append(b.Entries, BallotEntry{
				User: k,
				Vote: v.Vote.Value,
			})
			if v.Vote.Value > 0 {
				b.UpVoteCount++
			}
		}
	}
	sort.Slice(b.Entries, func(i, j int) bool {
		return b.Entries[i].User < b.Entries[j].User
	})
	return b
}

// GetActionDashboard returns a dashboard of activity
// https://www.perforce.com/manuals/swarm/Content/Swarm/swarm-apidoc_endpoint_reviews.html#Get_reviews_for_action_dashboard
func (i *impl) GetActionDashboard() ([]Review, error) {
	var rc ReviewMap

	endpoint := "api/v9/dashboards/action"

	if err := i.doSwarmRequestResponse("GET", endpoint, nil, &rc); err != nil {
		return nil, fmt.Errorf("swarm.GetActionDashboard %v", err)
	}

	var reviews []Review
	for _, r := range rc.Reviews {
		reviews = append(reviews, r)
	}
	sort.Slice(reviews, func(i, j int) bool {
		return reviews[i].ID > reviews[j].ID
	})

	return reviews, nil
}

// SendSwarmRequest replies to swarm a message of |responseType|.
// SendTestRunRequest sends a response to a test run initiated by Swarm.
// |updateUrl| is the endpoint Swarm provided to be call back on with this test run requests.
// |resultsUrl| (optional) is the results/logs url given back to Swarm so that it can display it
// in the review page.
// If successful, returns the body of the response provided by Swarm.
func (i *impl) SendTestRunRequest(responseType TestRunResponseType, updateUrl, resultsUrl string) (string, error) {
	// |updateUrl| is normally a full url (https://foo.com/bar) in which the host is very possibly
	// different to the one we're using to communicate with Swarm. We need to strip the host part
	// in order to use it and an endpoint.
	parsedUrl, err := url.Parse(updateUrl)
	if err != nil {
		return "", err
	}
	path := parsedUrl.Path
	// If the path starts with a slash, we remove it.
	if path[0] == '/' {
		path = path[1:]
	}

	// Create the payload to be send.
	err = nil
	var payload []byte
	switch responseType {
	case TestRunStart:
		payload, err = createTestRunPayload("update", "presubmit is starting", resultsUrl)
	case TestRunPass:
		payload, err = createTestRunPayload("pass", "presubmit was successful", resultsUrl)
	case TestRunFail:
		payload, err = createTestRunPayload("fail", "presubmit failed", resultsUrl)
	default:
		return "", fmt.Errorf("invalid request type: %v", responseType)
	}
	if err != nil {
		return "", err
	}

	response, err := i.doSwarmRequest("POST", path, jsonEncoded, payload)
	if err != nil {
		return "", err
	}
	return string(response), nil
}

// CreateTestRun creates a test run entry for the given review and UUID.
func (i *impl) CreateTestRun(review, version int, uuid string) (*TestRun, error) {
	req := map[string]interface{}{
		"change":    review,
		"version":   version,
		"startTime": time.Now().Unix(),
		"status":    "running",
		"test":      "project:presubmit:test",
		"uuid":      uuid,
	}
	var resp struct {
		Data struct {
			Testruns []TestRun `json:"testruns"`
		} `json:"data"`
	}
	if err := i.doSwarmRequestResponse("POST", fmt.Sprintf("api/v10/reviews/%d/testruns", review), req, &resp); err != nil {
		return nil, err
	}

	if len(resp.Data.Testruns) != 1 {
		return nil, fmt.Errorf("wanted 1 testrun, got %d", len(resp.Data.Testruns))
	}
	return &resp.Data.Testruns[0], nil
}

func (i *impl) TestRunDetails(review, version int) (map[int]TestRun, error) {
	var runs struct {
		Error    string   `json:"error"`
		Messages []string `json:"messages"`
		Data     struct {
			Testruns TestRunsMap `json:"testruns"`
		} `json:"data"`
		Status string `json:"status"`
	}
	err := i.doSwarmRequestResponse("GET", fmt.Sprintf("api/v10/reviews/%d/testruns?version=%d", review, version), nil, &runs)
	if err != nil {
		return nil, fmt.Errorf("swarm.TestRunDetails: %w", err)
	}
	if runs.Error != "" {
		return nil, fmt.Errorf("swarm.TestRunDetails: %s [%s]", runs.Error, strings.Join(runs.Messages, ", "))
	}
	return runs.Data.Testruns, nil
}

// Misc --------------------------------------------------------------------------------------------

// doSwarmRequest sends an HTTP request to swarm, returning the byte payload is successful.
// |action| is an HTTP action (GET, POST, etc.).
// |endpoint| is the path to be queried by the request (eg. https://sge-swarm:9000/<ENDPOINT>).
func (i *impl) doSwarmRequest(action, endpoint, encoding string, payload []byte) ([]byte, error) {
	url := i.BuildUrl(endpoint)
	req, err := http.NewRequestWithContext(i.ctx.Ctx, action, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(i.ctx.Username, i.ctx.Password)
	req.Header.Set("Content-Type", encoding)

	client := i.ctx.client()
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	data, err := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("couldn't read response for %s %v: %w", action, url, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusAccepted {
		log.Warningf("unexpected status for %s %v: %v (%s)", action, url, resp.Status, data)
	}

	// Don't bother checking the status code since Swarm sometimes returns
	// unexpected status codes on success.  Instead, check if the response
	// contains isValid == false to check for failure.
	var isValid struct {
		Error    string   `json:"error"`
		Messages []string `json:"messages"`
		IsValid  *bool    `json:"isValid"`
	}
	err = json.Unmarshal(data, &isValid)
	if err == nil && isValid.IsValid != nil && !(*isValid.IsValid) {
		return data, fmt.Errorf("invalid response for %s %v: %s", action, url, data)
	}
	if err == nil && isValid.Error != "" {
		return data, fmt.Errorf("error response for %s %v: %s", action, url, data)
	}
	return data, nil
}

func (ctx *Context) client() *http.Client {
	if ctx.Client != nil {
		return ctx.Client
	}
	return &http.Client{}
}

func (i *impl) doSwarmRequestResponse(action, endpoint string, req, resp interface{}) error {
	var payload []byte = nil
	var err error
	if req != nil {
		payload, err = json.Marshal(req)
		if err != nil {
			return fmt.Errorf("couldn't marshal %v to json: %v", reflect.TypeOf(req).Name(), err)
		}
	}
	payload, err = i.doSwarmRequest(action, endpoint, jsonEncoded, payload)
	if err != nil {
		return err
	}
	if resp != nil {
		if err := json.Unmarshal(payload, resp); err != nil {
			return fmt.Errorf("couldn't unmarshal json '%s' to %v: %v", payload, reflect.TypeOf(resp).Name(), err)
		}
	}
	return nil
}

func createTestRunPayload(status, body, resultsUrl string) ([]byte, error) {
	message := &TestRunResponse{
		Status:   status,
		Url:      resultsUrl,
		Messages: []string{body},
	}
	return json.Marshal(message)
}
