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

// binary perculator displays statistics about usage of perforce and swarm

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/swarm"

	"github.com/golang/glog"
)

const perculatorVersion = "0.0.2"
const appName = "perculator"

const swarmHost = "INSERT_HOST"
const swarmPort = 9000

var uriArg = flag.String("uri", "", "uri argument")

var gContext perculatorContext

const (
	periodDay = iota
	periodWeek
	periodMonth
	periodQuarter
	periodYear
	periodLifetime

	periodLen
)

const (
	statClCount = iota
	statFileCount
	statFileEditCount
	statFileSize

	statReviewsAuthored
	statReviewsParticipant
	statReviewsApprovalsGiven
	statReviewsApprovalsReceived
	statReviewsUpvotesGiven
	statReviewsUpvotesReceived
	statReviewsDownvotesGiven
	statReviewsDownvotesReceived
	statReviewsNovotesGiven
	statReviewsNovotesReceived
	statReviewsVotesGiven
	statReviewsVotesReceived

	statCommentsGiven

	statLen
)

const (
	ratioClWithReview = iota
	ratioReviewVoted
	ratioReviewCommented

	ratioLen
)

const (
	dayByteCount = iota
	dayClCount
	dayFileCount
	daySize
	dayUserCount

	dayLen
)

type accumulator struct {
	counter [periodLen]int64
}

type perculatorUser struct {
	userName string

	accs [statLen]accumulator
}

type dayDetails struct {
	counts [dayLen]int64
	users  map[string]bool
	date   time.Time
}

type changeDetails struct {
	Cl      int
	Files   int
	Date    time.Time
	Bytes   uint64
	Actions [p4lib.ActionLen]int
}

type perculatorContext struct {
	p4 p4lib.P4

	period        int
	changesFile   string
	detailsFile   string
	username      string
	tabName       string
	tabSelectName string

	users    []perculatorUser
	usersMap map[string]*perculatorUser

	dayMap map[time.Time]*dayDetails
	days   []*dayDetails
	dayMin time.Time
	dayMax time.Time

	details []changeDetails

	changesMap map[int]*p4lib.Change
	changes    []p4lib.Change

	ticketsMap map[string]p4lib.Ticket

	swarm            swarm.Context
	comments         swarm.CommentCollection
	reviewCollection swarm.ReviewCollection

	changesChan  chan []p4lib.Change
	commentsChan chan swarm.CommentCollection
	detailsChan  chan []changeDetails
	reviewsChan  chan swarm.ReviewCollection
	uriChan      chan string
	usersChan    chan []p4lib.User
	runSave      chan bool
	cancelSave   chan bool
}

func (acc *accumulator) add(d time.Time, value int64) {
	diff := time.Now().Sub(d)
	hours := diff.Hours()
	days := hours / 24
	weeks := days / 7
	months := days / 30
	quarters := months / 3
	years := days / 365

	if days <= 2 {
		acc.counter[periodDay] += value
	}
	if weeks <= 1 {
		acc.counter[periodWeek] += value
	}
	if months <= 1 {
		acc.counter[periodMonth] += value
	}
	if quarters <= 1 {
		acc.counter[periodQuarter] += value
	}
	if years <= 1 {
		acc.counter[periodYear] += value
	}
	acc.counter[periodLifetime] += value
}

func (ctx *perculatorContext) init() error {
	ctx.period = periodLifetime

	base, _ := files.GetAppDir("sge", appName)
	ctx.changesFile = filepath.Join(base, "perc_changes.json")
	ctx.detailsFile = filepath.Join(base, "perc_details.json")

	ctx.p4 = p4lib.New()
	// get username without domain
	current, err := user.Current()
	if err != nil {
		return err
	}
	lastSlash := strings.LastIndex(current.Username, "\\") + 1
	ctx.username = string(current.Username[lastSlash:])

	ctx.changesMap = make(map[int]*p4lib.Change)
	ctx.changesChan = make(chan []p4lib.Change)
	ctx.detailsChan = make(chan []changeDetails)

	ctx.dayMin = time.Now()
	ctx.dayMap = make(map[time.Time]*dayDetails)

	ctx.commentsChan = make(chan swarm.CommentCollection)

	ctx.reviewsChan = make(chan swarm.ReviewCollection)
	ctx.ticketsMap = make(map[string]p4lib.Ticket)

	ctx.usersMap = make(map[string]*perculatorUser)
	ctx.usersChan = make(chan []p4lib.User)
	ctx.uriChan = make(chan string)

	fromLink(ctx, *uriArg)

	ctx.runSave = make(chan bool)
	ctx.cancelSave = make(chan bool)
	go threadGeneric(ctx.cancelSave, ctx.runSave, ctx, processSaveDetails)

	go fetchUsers(ctx)

	return nil
}

func (ctx *perculatorContext) deinit() error {
	ctx.cancelSave <- true
	return nil
}

var excludedUsers = []string{"jenkins", "p4admin", "super", "swarm"}

func isUserExcluded(name string) bool {
	for _, u := range excludedUsers {
		if u == name {
			return true
		}
	}
	return false
}

func getFilteredUsers(input []p4lib.User) []string {
	var filtered []string
	for i := range input {
		if !isUserExcluded(input[i].User) {
			filtered = append(filtered, input[i].User)
		}
	}
	return filtered
}

func fetchChanges(ctx *perculatorContext) {
	var firstChanges []p4lib.Change
	files.JsonLoad(ctx.changesFile, &firstChanges)
	firstCl := 0
	for i := range firstChanges {
		if firstChanges[i].Cl > firstCl {
			firstCl = firstChanges[i].Cl
		}
	}
	changes, err := ctx.p4.Changes("-s", "submitted", fmt.Sprintf("@%d,@now", firstCl+1))
	if err != nil {
		glog.Errorf("could not retrieve submitted changelists: %v", err)
	}
	changes = append(firstChanges, changes...)
	ctx.changesChan <- changes
}

func fetchComments(ctx *perculatorContext) {
	c, err := swarm.GetComments(&ctx.swarm, "max=10000")
	if err != nil {
		glog.Warningf("could not get swarm comments: %v", err)
	} else {
		ctx.commentsChan <- c
	}
}

func fetchReviews(ctx *perculatorContext) {
	rc, err := swarm.GetReviews(&ctx.swarm, "max=10000")
	if err != nil {
		glog.Warningf("couldn't get reviews: %v", err)
	} else {
		ctx.reviewsChan <- rc
	}
}

func fetchTickets(ctx *perculatorContext) {
	tickets, err := ctx.p4.Tickets()
	if err != nil {
		glog.Errorf("could not get p4 tickets: %v", err)
		return
	}

	for _, t := range tickets {
		ctx.ticketsMap[t.User] = t
	}

	tick, ok := ctx.ticketsMap[ctx.username]
	if !ok {
		glog.Warningf("could not find p4 ticket for user %s", ctx.username)
		return
	}

	ctx.swarm = swarm.Context{
		Host:     swarmHost,
		Port:     swarmPort,
		Username: ctx.username,
		Password: tick.ID,
	}

	go fetchComments(ctx)
	go fetchReviews(ctx)
}

func fetchUsers(ctx *perculatorContext) {
	u, err := ctx.p4.Users()
	if err != nil {
		glog.Errorf("could not fetch users: %v", err)
	} else {
		ctx.usersChan <- u
	}
}

func updateUsers(ctx *perculatorContext) {
	select {
	case rawUsers := <-ctx.usersChan:
		users := getFilteredUsers(rawUsers)

		ctx.users = make([]perculatorUser, len(users))
		for i, u := range users {
			ctx.users[i] = perculatorUser{userName: u}
			ctx.usersMap[u] = &ctx.users[i]
		}

		go fetchChanges(ctx)
		go fetchTickets(ctx)

	default:
	}
}

func saveChanges(ctx *perculatorContext) {
	if len(ctx.changes) > 0 {
		files.JsonSave(ctx.changesFile, &ctx.changes)
	}
}

func fetchDetails(ctx *perculatorContext) {
	var firstDetails []changeDetails

	files.JsonLoad(ctx.detailsFile, &firstDetails)
	firstCl := 0
	for i := range firstDetails {
		if firstDetails[i].Cl > firstCl {
			firstCl = firstDetails[i].Cl
		}
	}
	ctx.detailsChan <- firstDetails

	var cls []int
	for i := range ctx.changes {
		if ctx.changes[i].Cl > firstCl {
			cls = append(cls, ctx.changes[i].Cl)
		}
	}
	// run a maximum of 32 concurrent jobs
	limiter := make(chan bool, 32)
	for _, cl := range cls {
		glog.Infof("fetchDetails %d", cl)
		go func(cl int) {
			limiter <- true
			processDetails(ctx, cl)
			<-limiter
		}(cl)
	}
}

func processDetails(ctx *perculatorContext, cl int) {
	glog.Infof("processDetails %d", cl)
	desc, e := ctx.p4.Describe([]int{cl})
	if e != nil {
		glog.Errorf("p4 describe failed: %v", e)
		return
	}
	for _, d := range desc {
		sizes, err := ctx.p4.Sizes(fmt.Sprintf("//...@%d,@%d", d.Cl-1, d.Cl))
		if err != nil {
			glog.Errorf("error getting sizes for cl :%d : %v", d.Cl, err)
			return
		}
		cd := changeDetails{
			Cl:    d.Cl,
			Date:  time.Unix(d.DateUnix, 0),
			Files: int(sizes.TotalFileCount),
			Bytes: sizes.TotalFileSize,
		}
		for j := range d.Files {
			if at, err := p4lib.GetActionType(d.Files[j].Action); err == nil {
				cd.Actions[at]++
			}
		}
		ctx.detailsChan <- []changeDetails{cd}
	}
}

func processSaveDetails(ctx *perculatorContext) {
	if len(ctx.details) > 0 {
		files.JsonSave(ctx.detailsFile, &ctx.details)
	}
}

func threadGeneric(cancel, run <-chan bool, ctx *perculatorContext, job func(*perculatorContext)) {
	for {
		select {
		case _ = <-cancel:
			break
		case _ = <-run:
			job(ctx)
		}
	}
}

func updateChanges(ctx *perculatorContext) {
	select {
	case ctx.changes = <-ctx.changesChan:
		for i := range ctx.changes {
			date := time.Unix(ctx.changes[i].DateUnix, 0)
			if user, ok := ctx.usersMap[ctx.changes[i].User]; ok {
				user.accs[statClCount].add(date, 1)
			}
			ctx.changesMap[ctx.changes[i].Cl] = &ctx.changes[i]
		}

		go saveChanges(ctx)
		go fetchDetails(ctx)

	default:
	}
}

func updateDetails(ctx *perculatorContext) {
	select {
	case newDeets := <-ctx.detailsChan:
		ctx.details = append(ctx.details, newDeets...)
		go func() {
			ctx.runSave <- true
		}()

		//		dayMap := make(map[time.Time]dayDetails)
		for _, d := range newDeets {
			c, ok := ctx.changesMap[d.Cl]
			if !ok {
				glog.Warningf("could not find changelist: %d", d.Cl)
				continue
			}
			if user, ok := ctx.usersMap[c.User]; ok {
				user.accs[statFileCount].add(d.Date, int64(d.Files))
				user.accs[statFileEditCount].add(d.Date, int64(d.Actions[p4lib.ActionEdit]))
				user.accs[statFileSize].add(d.Date, int64(d.Bytes))
			}

			quant := time.Date(d.Date.Year(), d.Date.Month(), d.Date.Day(), 0, 0, 0, 0, time.UTC)

			dd, ok := ctx.dayMap[quant]
			if !ok {
				dd = &dayDetails{
					users: make(map[string]bool),
				}
				ctx.days = append(ctx.days, dd)
			}
			dd.counts[dayClCount]++
			dd.counts[dayFileCount] += int64(d.Files)
			dd.counts[daySize] += int64(d.Bytes)
			dd.users[c.User] = true
			dd.date = quant
			ctx.dayMap[quant] = dd
		}

		for i := range ctx.days {
			ctx.days[i].counts[dayUserCount] = int64(len(ctx.days[i].users))
		}

	default:
	}
}

func updateReviews(ctx *perculatorContext) {
	select {
	case ctx.reviewCollection = <-ctx.reviewsChan:
		for _, r := range ctx.reviewCollection.Reviews {
			created := time.Unix(int64(r.Created), 0)
			updated := time.Unix(int64(r.Updated), 0)

			user, ok := ctx.usersMap[r.Author]
			if ok {
				user.accs[statReviewsAuthored].add(created, 1)
			}
			if len(r.Approvals) > 0 {
				user.accs[statReviewsApprovalsReceived].add(updated, 1)
			}
			for k, p := range r.Participants {
				puser, ok := ctx.usersMap[k]
				if ok {
					if puser.userName != user.userName {
						puser.accs[statReviewsParticipant].add(updated, 1)
						if p.Vote.Value > 0 {
							user.accs[statReviewsUpvotesReceived].add(updated, 1)
							puser.accs[statReviewsUpvotesGiven].add(updated, 1)
							user.accs[statReviewsVotesReceived].add(updated, 1)
							puser.accs[statReviewsVotesGiven].add(updated, 1)
						} else if p.Vote.Value < 0 {
							user.accs[statReviewsDownvotesReceived].add(updated, 1)
							puser.accs[statReviewsDownvotesGiven].add(updated, 1)
							user.accs[statReviewsVotesReceived].add(updated, 1)
							puser.accs[statReviewsVotesGiven].add(updated, 1)
						} else {
							user.accs[statReviewsNovotesReceived].add(updated, 1)
							puser.accs[statReviewsNovotesGiven].add(updated, 1)
						}
					}
				} else {
					glog.Warningf("user not found: %s", k)
				}
			}
		}
	default:
	}
}

func updateComments(ctx *perculatorContext) {
	select {
	case ctx.comments = <-ctx.commentsChan:
		for _, c := range ctx.comments.Comments {
			user, ok := ctx.usersMap[c.User]
			updated := time.Unix(int64(c.Updated), 0)
			if ok {
				user.accs[statCommentsGiven].add(updated, 1)
			}
		}
	default:
	}
}

func updateUri(ctx *perculatorContext) {
	select {
	case u := <-ctx.uriChan:
		fromLink(ctx, u)
	default:
	}
}

func toLink(ctx *perculatorContext) (string, error) {
	base := "sge://perculator"
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("couldn't parse url %q: %v", base, err)
	}
	args := url.Values{}
	args.Add("t", ctx.tabName)
	args.Add("p", fmt.Sprintf("%d", ctx.period))
	u.RawQuery = args.Encode()
	return u.String(), nil
}

func fromLink(ctx *perculatorContext, link string) error {
	url, err := url.Parse(link)
	if err != nil {
		return fmt.Errorf("couldn't parse url %q: %v", link, err)
	}
	args := url.Query()
	if t, ok := args["t"]; ok && len(t) > 0 {
		ctx.tabSelectName = t[0]
	}
	if p, ok := args["p"]; ok && len(p) > 0 {
		if val, err := strconv.Atoi(p[0]); err == nil {
			ctx.period = val
		}
	}
	return nil
}

func (ctx *perculatorContext) update() {
	updateChanges(ctx)
	updateComments(ctx)
	updateDetails(ctx)
	updateReviews(ctx)
	updateUri(ctx)
	updateUsers(ctx)
}

func handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		fmt.Fprintf(w, "perculator post")
		var uri string
		if err := json.NewDecoder(r.Body).Decode(&uri); err != nil {
			glog.Errorf("could not decode posted JSON: %v", err)
		} else {
			// send this in a go func to effectively have unbounded channel capacity
			go func(uri string) {
				gContext.uriChan <- uri
			}(uri)
		}
	}
}
