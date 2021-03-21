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

// package swarmLib wraps perforce CLI commands in a convenient interface

package swarm

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
	"strconv"
	"strings"
	"time"

	"sge-monorepo/libs/go/log"
)

const (
	formEncoded = "application/x-www-form-urlencoded"
	jsonEncoded = "application/json"
)

// Context represents the associated state needed to communicate with Swarm.
type Context struct {
	Host     string // base address e.g. https://my-url.com/
	Port     int    // port e.g. 9000
	Username string // Credentials.
	Password string // Credentials.

	// Client is an HTTP client to be used for http request.
	// Users of the library can set it to override the default one.
	Client *http.Client
	Ctx    context.Context
}

// New returns a context with which to make Swarm requests.
// Usage:
//      s := swarm.New(host, port, username, password)
//      review, err := swarm.GetReview(s, 1)
//      ...
func New(host string, port int, username, password string) *Context {
	return &Context{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		Ctx:      context.Background(),
	}
}

// Error wraps HTTP Status values as errors, enabling checks on specific Swarm
// errors such as NotFound.
type Error int

func (e Error) Error() string {
	return http.StatusText(int(e))
}
func (e Error) Status() int {
	return int(e)
}

// Structs -----------------------------------------------------------------------------------------

type VersionID int
type SwarmBool bool

// CommentContext contains metadata related to a comment
type CommentContext struct {
	Attribute string    `json:"attribute"` // ?? seems to always be "description"
	Change    int       `json:"change"`    // changelist ID
	Comment   int       `json:"comment"`   // ID of previous comment this is a reply to
	Content   []string  `json:"content"`   // optional code excerpt gives lines surrounding diff
	File      string    `json:"file"`      // depot file
	Line      int       `json:"line"`      // line to attach inline comment to (usually same as rightLine)
	LeftLine  int       `json:"leftLine"`  // left-side diff line to attach inline comment to
	MD5       string    `json:"md5"`       // ??
	Name      string    `json:"name"`      // filename (without full path)
	RightLine int       `json:"rightLine"` // right-side diff line to attach inline comment to
	Review    int       `json:"review"`    // id of review
	Type      string    `json:"type"`      // perforce type of file ["text", "text+x", "binary" etc]
	Version   VersionID `json:"version"`   // specifies the version of review to attach comment to
}

type CommentContextWrapper CommentContext

// Comment contains the body of a swarm content
// Note: a number of fields are currently commented out in these structures
// These are option fields that are causing issues with json unmarshalling due to swarm's inconsistent serialisation code
type Comment struct {
	ID        int             `json:"id"`        // id of comment
	Body      string          `json:"body"`      // text of comment
	Context   *CommentContext `json:"context"`   // context of comment
	Edited    *int            `json:"edited"`    // unix time of last edit, null if never edited
	Likes     []string        `json:"likes"`     // array of usernames who like comment
	ReadBy    []string        `json:"readBy"`    // array of usernames who marked comment as read
	TaskState string          `json:"taskState"` // optional state of comment, can be "comment" or "open"
	Time      int             `json:"time"`      // unix time of comment creation
	Topic     string          `json:"topic"`     // topic that comment is related to (reviews/id, changes/id, jobs/id)
	Updated   int             `json:"updated"`   // unix time of comment update
	User      string          `json:"user"`      // user name of comment author

	//	Attachments []string            `json:"attachments"`
	Flags []string `json:"flags"` // array of flags
}

// CommentAddContext details the file an position where a comment should reside
type CommentAddContext struct {
	File      string   `json:"file"`      // depot file
	LeftLine  int      `json:"leftLine"`  // left-side diff line to attach inline comment to
	RightLine int      `json:"rightLine"` // right-side diff line to attach inline comment to
	Version   string   `json:"version"`   // specfies the version of review to attach comment to
	Comment   int      `json:"comment"`   // optional comment being replied to
	Content   []string `json:"content"`   // optional lines being commented on
}

// CommentAdd contains details about a comment to add to a review
type CommentAdd struct {
	Body              string             `json:"body"`              // text of comment
	Topic             string             `json:"topic"`             // topic that comment is related to (reviews/id, changes/id, jobs/id)
	Context           *CommentAddContext `json:"context"`           // context of comment
	DelayNotification string             `json:"delayNotification"` // Set to "true" to delay sending notifications
	Flags             []string           `json:"flags"`             // array of flags
	//	SilenceNotification string `json:"silenceNotification"`
	//	TaskState   string `json:"taskState"`
}

// CommentUpdate contains details about an update to a comment
type CommentUpdate struct {
	ID    int      `json:"id"`    // id of comment
	Body  string   `json:"body"`  // text of comment
	Topic string   `json:"topic"` // topic that comment is related to (revies/id, changes/id, jobs/id)
	Flags []string `json:"flags"` // array of flags
}

// CommentCollection contains a collection of comments
type CommentCollection struct {
	Comments []Comment `json:"comments"` // array of comments
	LastSeen int       `json:"lastSeen"` // last seen can be used an offset for pagination by using "after" parameter for subsequent requests
}

type CommitStatusWrapper CommitStatus

// CommitStatus details if the changelist has been commited and by whom
type CommitStatus struct {
	Start     int    `json:"start"`     // start time of commit
	Change    int    `json:"change"`    // changelist number
	Status    string `json:"status"`    // status of changelist (usually "Committed")
	Committer string `json:"committer"` // user who committed change
	End       int    `json:"end"`       // end time of commit
}

// Participant details the vote for a participiant in a review
type Participant struct {
	Required bool `json:"required"`
	Vote     Vote `json:"vote"`
}

// Vote contains a vote (+1 for upvote,-1 for downvote) and version of review it was applied to
type Vote struct {
	Value   int  `json:"value"`
	Version int  `json:"version"`
	IsStale bool `json:"isStale"`
}

type ParticipantWrapper Participant

// Review contains details about a swarm revie
type Review struct {
	ID            int                    `json:"id"`            // id of review
	Author        string                 `json:"author"`        // author of review
	Approvals     map[string][]int       `json:"approvals"`     // map of users to review versions that they approved
	Changes       []int                  `json:"changes"`       // array of changes associated with review
	Comments      []int                  `json:"comments"`      // array of comment ids
	Commits       []int                  `json:"commits"`       // array of commited changelists associated with this review
	CommitStatus  CommitStatus           `json:"commitStatus"`  // details about committed cl
	Created       int                    `json:"created"`       // unix time of review creation
	DeployDetails []string               `json:"deployDetails"` //
	DeployStatus  string                 `json:"deployStatus"`  //
	Description   string                 `json:"description"`   // changelist description
	Groups        []string               `json:"groups"`        // array of access groups associated with changelist
	Participants  map[string]Participant `json:"participants"`  // map of usernames and related votes
	Pending       SwarmBool              `json:"pending"`       // if true, change is still in pending status
	//Projects []map[string][]string `json:"projects"`

	ReviewerGroups []string `json:"reviewerGroups"` //
	State          string   `json:"state"`
	StateLabel     string   `json:"stateLabel"`
	//TestDetails []TestDetails `json:"testDetails"`
	TestStatus  string    `json:"testStatus"` // status of associated tests [null,"pass","fail","running"]
	Type        string    `json:"type"`
	Updated     int       `json:"updated"`     // unix time when review was updated
	UpdatedDate string    `json:"updatedDate"` // plain text formatted update date
	Versions    []Version `json:"versions"`    // array of iterations of this review
}

// ReviewMap contains a map that maps review IDs to reviews
type ReviewMap struct {
	LastSeen   *int           `json:"lastSeen"`   // id of last review, can be used for pagination of requests
	Reviews    map[int]Review `json:"reviews"`    // array of reviews
	TotalCount int            `json:"totalCount"` // total count of reviews in collection
}

// ReviewMap contains a collection of reviews
type ReviewCollection struct {
	LastSeen   int      `json:"lastSeen"`   // id of last review, can be used for pagination of requests
	Reviews    []Review `json:"reviews"`    // array of reviews
	TotalCount int      `json:"totalCount"` // total count of reviews in collection
}

// ReviewPatch contains fields used to update a review.
type ReviewPatch struct {
	Description       *string  `json:"description,omitempty"`
	Reviewers         []string `json:"reviewers"`
	RequiredReviewers []string `json:"requiredReviewers"`
}

// TestDetails shows the start and times of tests
type TestDetails struct {
	StartTimes []int `json:"startTimes"` // array of unix times for test starts
	EndTimes   []int `json:"endTimes"`   // array of unix times for test ends (can be empty)
}

// User holds details about a swarm user
type User struct {
	Access     string `json:"Access"`     // date user last accessed swarm
	AuthMethod string `json:"AuthMethod"` // type of authentication
	Email      string `json:"Email"`      // email address
	FullName   string `json:"FullName"`   // date user details were updated
	Reviews    []int  `json:"Reviews"`    // ??
	Type       string `json:"Type"`       // type of user
	Update     string `json:"Update"`     // date user details were updated
	User       string `json:"User"`       // username
}

// Version contains details about a swarm review version
type Version struct {
	AddChangeMode        string `json:"addChangeMode"`        //
	Change               int    `json:"change"`               // changelist number
	Difference           int    `json:"difference"`           //
	Pending              bool   `json:"pending"`              // cl status
	Stream               string `json:"stream"`               //
	StreamSpecDifference int    `json:"streamSpecDifference"` //
	TestRuns             []int  `json:"testRuns"`             // indices of text runs
	Time                 int    `json:"time"`                 // unix time
	User                 string `json:"user"`                 // user name
}

// TestRun contains details about a test run for a swarm review
type TestRun struct {
	ID            int      `json:"id"`
	Change        int      `json:"change"`
	Version       int      `json:"version"`
	Test          string   `json:"test"`
	StartTime     int64    `json:"startTime"`
	CompletedTime int64    `json:"completedTime"`
	Status        string   `json:"status"`
	Messages      []string `json:"messages"`
	URL           string   `json:"url"`
	UUID          string   `json:"uuid"`
}

// TestRuns holds details about test runs.
type TestRunsMap map[int]TestRun

// BallotEntry represents a single vote in a review
type BallotEntry struct {
	User string
	Vote int
}

// Ballot contains all votes for a review
type Ballot struct {
	Entries     []BallotEntry
	UpVoteCount int
}

// TestRunResponseType represents what kind of message we want to send the CI system in response of
// a requested test run. For more information see:
//
// https://www.perforce.com/manuals/swarm/Content/Swarm/quickstart.integrate_test_suite.html
type TestRunResponseType int

const (
	// None means an invalid request. Used for error returning.
	TestRunNone TestRunResponseType = iota

	// TestRunStart means "starting to work".
	TestRunStart

	// TestRunPass means the test run was successful.
	TestRunPass

	// TestRunFail means the test run failed.
	TestRunFail
)

// TestRunResponse represents the json payload of an HTTP message to be sent to swarm in response
// to a test run request.
type TestRunResponse struct {
	Status   string   `json:"status"`
	Url      string   `json:"url"`
	Messages []string `json:"messages"`
}

// API ---------------------------------------------------------------------------------------------

// the swarm version field has inconsistent encoding (sometimes it is a string, other times it is int)
// we implement a custom json unmarshaller to handle both the string and it cases
func (t *VersionID) UnmarshalJSON(data []byte) error {
	s := string(data)
	v, err := strconv.Atoi(s)
	if err != nil && len(s) > 1 {
		v, err = strconv.Atoi(s[1 : len(s)-1])
		err = nil
	}
	*t = VersionID(v)
	return err
}

// the swarm commitstatus object is sometimes returned as an empty array object
func (cs *CommitStatus) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "[]" {
		return nil
	}
	var csw CommitStatusWrapper
	if err := json.Unmarshal(data, &csw); err != nil {
		return err
	}
	*cs = CommitStatus(csw)
	return nil
}

// the swarm participant object is sometimes returned as an empty array object
func (v *Participant) UnmarshalJSON(data []byte) error {
	s := string(data)
	if s == "[]" {
		return nil
	}
	var vi ParticipantWrapper
	if err := json.Unmarshal(data, &vi); err != nil {
		return err
	}
	*v = Participant(vi)
	return nil
}

// the swarm comment context object is sometimes returned as an empty array
func (cc *CommentContext) UnmarshalJSON(data []byte) error {
	if string(data) == "[]" {
		return nil
	}
	var tmp CommentContextWrapper
	if err := json.Unmarshal(data, &tmp); err != nil {
		return err
	}
	*cc = CommentContext(tmp)
	return nil
}

// The swarm test run response returns an empty array as an empty map.
func (tr *TestRunsMap) UnmarshalJSON(data []byte) error {
	if string(data) == "[]" {
		return nil
	}
	var runs map[int]TestRun
	if err := json.Unmarshal(data, &runs); err != nil {
		return err
	}
	*tr = runs
	return nil
}

// Swarm sometimes returns integers for booleans.
func (sb *SwarmBool) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err != nil {
		i, ierr := strconv.Atoi(string(data))
		if ierr != nil {
			return err
		}
		*sb = i != 0
		return nil
	}
	*sb = SwarmBool(b)
	return nil
}

// GetReview returns a swarm review identified by |id|.
func GetReview(ctx *Context, id int) (*Review, error) {
	endpoint := fmt.Sprintf("api/v9/reviews/%d", id)
	// The response wraps the review in a JSON object with a "review" key.
	msg := struct {
		Review *Review
	}{}
	if err := ctx.doSwarmRequest("GET", endpoint, nil, &msg); err != nil {
		return nil, err
	}
	return msg.Review, nil
}

func getReviewsPage(ctx *Context, after int, args string) (ReviewCollection, error) {
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

	if err := ctx.doSwarmRequest("GET", endpoint, nil, &rc); err != nil {
		return rc, fmt.Errorf("swarm.getReviewsPage %v", err)
	}
	return rc, nil
}

// GetReviews returns a collection of swarm reviews
func GetReviews(ctx *Context, args string) (ReviewCollection, error) {
	var rc ReviewCollection

	page, err := getReviewsPage(ctx, 0, args)
	if err != nil {
		return rc, err
	}
	for len(page.Reviews) > 0 {
		rc.Reviews = append(rc.Reviews, page.Reviews...)
		page, err = getReviewsPage(ctx, page.LastSeen, args)
		if err != nil {
			return rc, err
		}
	}
	return rc, nil
}

// GetReviewsForChangelists returns a colllection containing reviews for all specified changelists
func GetReviewsForChangelists(ctx *Context, changeLists []int) (ReviewCollection, error) {
	var rc ReviewCollection
	endpoint := ""
	for _, c := range changeLists {
		if len(endpoint) > 0 {
			endpoint += "&"
		}
		endpoint += fmt.Sprintf("change[]=%d", c)
		if len(endpoint) > 8000 {
			r, err := GetReviews(ctx, endpoint)
			if err != nil {
				return rc, err
			}
			rc.Reviews = append(rc.Reviews, r.Reviews...)
			endpoint = ""
		}
	}
	if len(endpoint) > 0 {
		r, err := GetReviews(ctx, endpoint)
		if err != nil {
			return rc, err
		}
		rc.Reviews = append(rc.Reviews, r.Reviews...)
	}
	return rc, nil
}

// GetOpenReviews returns a colllection containing reviews for all specified changelists
func GetOpenReviews(ctx *Context, username string) (ReviewCollection, error) {
	return GetReviews(ctx, fmt.Sprintf("participants=%s&state=needsReview", username))
}

func getCommentsPage(ctx *Context, after int, args string) (CommentCollection, error) {
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
	if err := ctx.doSwarmRequest("GET", endpoint, nil, &cc); err != nil {
		return cc, fmt.Errorf("swarm.getCommentsPage %v", err)
	}
	return cc, nil
}

// GetComments returns a collection of comments
func GetComments(ctx *Context, args string) (CommentCollection, error) {
	var cc CommentCollection

	page, err := getCommentsPage(ctx, 0, args)
	if err != nil {
		return cc, err
	}
	for len(page.Comments) > 0 {
		cc.Comments = append(cc.Comments, page.Comments...)
		page, err = getCommentsPage(ctx, page.LastSeen, args)
		if err != nil {
			return cc, err
		}
	}
	return cc, nil
}

// GetCommentsForReview returns details about comments for specified review
func GetCommentsForReview(ctx *Context, reviewIndex int) (CommentCollection, error) {
	return GetComments(ctx, fmt.Sprintf("topic=reviews/%d", reviewIndex))
}

// UpdateComment updates a comment in a review
// https://www.perforce.com/manuals/swarm/Content/Swarm/swarm-apidoc_endpoint_comments.html#Edit_a_Comment
func UpdateComment(ctx *Context, comment *Comment) error {
	endpoint := fmt.Sprintf("api/v9/comments/%d", comment.ID)
	scu := CommentUpdate{
		Body:  comment.Body,
		ID:    comment.ID,
		Topic: comment.Topic,
		Flags: comment.Flags,
	}
	if err := ctx.doSwarmRequest("PATCH", endpoint, scu, nil); err != nil {
		return fmt.Errorf("swarm.UpdateComment %v", err)
	}
	return nil
}

// AddComment adds a comment to a review
// https://www.perforce.com/manuals/swarm/Content/Swarm/swarm-apidoc_endpoint_comments.html#Edit_a_Comment
func AddComment(ctx *Context, comment *Comment) error {
	_, err := AddCommentEx(ctx, comment, false)
	return err
}
func AddCommentEx(ctx *Context, comment *Comment, delayNotification bool) (*Comment, error) {
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
	if err := ctx.doSwarmRequest("POST", "api/v9/comments", sca, &response); err != nil {
		return nil, fmt.Errorf("swarm.AddCommentEx %v", err)
	}
	if response.Error != "" {
		return nil, fmt.Errorf("swarm.AddCommentEx %s : %s", response.Error, response.Details.Context)
	}

	return &response.Comment, nil
}

// SendNotifications tells Swarm to send notifications for the specified review.
// Returns any informational message from Swarm, or an error on failure.
func SendNotifications(ctx *Context, review int) (string, error) {
	notify := url.Values{
		"topic": []string{fmt.Sprintf("reviews/%d", review)},
	}
	payload := []byte(notify.Encode())
	resp, err := doSwarmRequest(ctx, "POST", "api/v9/comments/notify", formEncoded, payload)
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
func SetVote(ctx *Context, review int, vote string) error {
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
	if err := ctx.doSwarmRequest("POST", fmt.Sprintf("api/v9/reviews/%d/vote", review), v, &response); err != nil {
		return fmt.Errorf("swarm.SetVote %v", err)
	}
	return nil
}

// PatchReview updates the specified review.
func PatchReview(ctx *Context, review int, patch *ReviewPatch) (*Review, error) {
	var response struct {
		Review *Review `json:"review"`
	}
	if err := ctx.doSwarmRequest("PATCH", fmt.Sprintf("api/v9/reviews/%d", review), patch, &response); err != nil {
		return nil, fmt.Errorf("swarm.PatchReview: %w", err)
	}
	if response.Review == nil {
		return nil, fmt.Errorf("swarm.PatchReview invalid response")
	}
	return response.Review, nil
}

// UpdateDescription updates the description for the specified review.
func UpdateDescription(ctx *Context, review int, description string) (*Review, error) {
	return PatchReview(ctx, review, &ReviewPatch{
		Description: &description,
	})
}

func SetState(ctx *Context, review int, state string) (*Review, error) {
	v := map[string]string{
		"state":       state,
		"description": fmt.Sprintf("Review %d has been %s by %s.", review, state, ctx.Username),
	}
	var response struct {
		Review *Review `json:"review"`
	}
	if err := ctx.doSwarmRequest("PATCH", fmt.Sprintf("api/v9/reviews/%d/state/", review), v, &response); err != nil {
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
func GetActionDashboard(ctx *Context) ([]Review, error) {
	var rc ReviewMap

	endpoint := "api/v9/dashboards/action"

	if err := ctx.doSwarmRequest("GET", endpoint, nil, &rc); err != nil {
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
func SendTestRunRequest(ctx *Context, responseType TestRunResponseType, updateUrl, resultsUrl string) (string, error) {
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

	response, err := doSwarmRequest(ctx, "POST", path, jsonEncoded, payload)
	if err != nil {
		return "", err
	}
	return string(response), nil
}

// CreateTestRun creates a test run entry for the given review and UUID.
func CreateTestRun(ctx *Context, review, version int, uuid string) (*TestRun, error) {
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
	if err := ctx.doSwarmRequest("POST", fmt.Sprintf("api/v10/reviews/%d/testruns", review), req, &resp); err != nil {
		return nil, err
	}

	if len(resp.Data.Testruns) != 1 {
		return nil, fmt.Errorf("wanted 1 testrun, got %d", len(resp.Data.Testruns))
	}
	return &resp.Data.Testruns[0], nil
}

func TestRunDetails(ctx *Context, review, version int) (map[int]TestRun, error) {
	var runs struct {
		Error    string   `json:"error"`
		Messages []string `json:"messages"`
		Data     struct {
			Testruns TestRunsMap `json:"testruns"`
		} `json:"data"`
		Status string `json:"status"`
	}
	err := ctx.doSwarmRequest("GET", fmt.Sprintf("api/v10/reviews/%d/testruns?version=%d", review, version), nil, &runs)
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
func doSwarmRequest(ctx *Context, action, endpoint, encoding string, payload []byte) ([]byte, error) {
	url := BuildUrl(ctx, endpoint)
	req, err := http.NewRequestWithContext(ctx.Ctx, action, url, bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(ctx.Username, ctx.Password)
	req.Header.Set("Content-Type", encoding)

	client := ctx.client()
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

func (ctx *Context) doSwarmRequest(action, endpoint string, req, resp interface{}) error {
	var payload []byte = nil
	var err error
	if req != nil {
		payload, err = json.Marshal(req)
		if err != nil {
			return fmt.Errorf("couldn't marshal %v to json: %v", reflect.TypeOf(req).Name(), err)
		}
	}
	payload, err = doSwarmRequest(ctx, action, endpoint, jsonEncoded, payload)
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

// BuildUrl constructs a swarm url based on host, port and endpoint.
func BuildUrl(context *Context, endpoint string) string {
	url := fmt.Sprintf("%s:%d/%s", context.Host, context.Port, endpoint)

	// golang's url parser gets tripped up if the url doesn't begin with http(s)://. This
	// might happen is the Host provided to Context is just the host part.
	if !strings.HasPrefix(url, "http") {
		url = fmt.Sprintf("https://%s", url)
	}
	return url
}

func createTestRunPayload(status, body, resultsUrl string) ([]byte, error) {
	message := &TestRunResponse{
		Status:   status,
		Url:      resultsUrl,
		Messages: []string{body},
	}
	return json.Marshal(message)
}
