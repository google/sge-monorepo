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

// Package swarmlib wraps the Swarm API.
package swarmlib

import (
    "net/http"
)

// Structs -----------------------------------------------------------------------------------------

// Error wraps HTTP Status values as errors, enabling checks on specific Swarm
// errors such as NotFound.
type Error int

func (e Error) Error() string {
	return http.StatusText(int(e))
}
func (e Error) Status() int {
	return int(e)
}

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

// SwarmApi is an abstract interface you can use to call into a Swarm-like system.
// Usage:
//      swarm := swarmlib.New()
//      review, err := swarm.GetReview(id)
type SwarmApi interface {
    BuildUrl(endpoint string) string
    Context() *Context
    GetReview(id int) (*Review, error)
    GetReviews(args string) (ReviewCollection, error)
    GetReviewsForChangelists(changeLists []int) (ReviewCollection, error)
    GetOpenReviews(username string) (ReviewCollection, error)
    GetComments(args string) (CommentCollection, error)
    GetCommentsForReview(reviewIndex int) (CommentCollection, error)
    UpdateComment(comment *Comment) error
    AddComment(comment *Comment) error
    AddCommentEx(comment *Comment, delayNotification bool) (*Comment, error)
    SendNotifications(review int) (string, error)
    SetVote(review int, vote string) error
    PatchReview(review int, patch *ReviewPatch) (*Review, error)
    UpdateDescription(review int, description string) (*Review, error)
    SetState(review int, state string) (*Review, error)
    GetActionDashboard() ([]Review, error)
    SendTestRunRequest(responseType TestRunResponseType, updateUrl, resultsUrl string) (string, error)
    CreateTestRun(review, version int, uuid string) (*TestRun, error)
    TestRunDetails(review, version int) (map[int]TestRun, error)
}

func New(ctx *Context) SwarmApi {
    return &impl{ctx}
}
