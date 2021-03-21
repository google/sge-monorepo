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

package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"strings"
	"sync"

	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/swarm"

	"github.com/AllenDang/giu"
	"github.com/golang/glog"
)

const gigantickVersion = "0.0.2"
const appName = "gigantick"

const swarmHost = "INSERT_HOST"
const swarmPort = 9000

var gContext gigantickContext

type opState int

const (
	opStateNone = iota
	opStateProcessing
	opStateDone
)

type filePair struct {
	left  string
	right string
}

type filePairDiff struct {
	pair      filePair
	fdiff     fileDiff
	diffs     []p4lib.Diff
	diffsChan chan []p4lib.Diff
	state     opState
}

type fileData struct {
	data     string
	filename string
	dataChan chan string
	state    opState
}

type gigantickFile struct {
	fstat    p4lib.FileStat
	comments []*swarm.Comment
}

type gigantickContext struct {
	username string
	exitFlag bool

	pendingChangesDone   bool
	submittedChangesDone bool
	ticketsDone          bool
	swarmDone            bool

	swarm      swarm.Context
	newComment swarm.Comment

	allChanges   map[int]*gigantickChange
	allFileData  map[string]*fileData
	allFileDiffs map[filePair]*filePairDiff
	allReviews   map[int]*swarm.Review
	allTickets   map[string]*p4lib.Ticket

	dashboardReviews []int

	p4    p4lib.P4
	info  p4lib.Info
	users []p4lib.User

	pendingChanges   []int
	submittedChanges []int

	changeToReviewIdMap map[int]int
	needsReviews        []int

	focusChange    int
	focusChangeNew int
	focusChanges   []int

	localAppArgs string
	localAppPath string
	uriArg       *string

	port      *string
	user      *string
	workspace *string

	goRoutDetails []gigantickGoroutine
	goRoutMutex   sync.Mutex

	dashboardReviewsChan chan []swarm.Review
	dashboardChangesChan chan []p4lib.Change

	infoChan    chan p4lib.Info
	ticketsChan chan []p4lib.Ticket
	usersChan   chan []p4lib.User

	pendingChangesChan   chan []p4lib.Change
	submittedChangesChan chan []p4lib.Change

	focusChangeChan chan int
	focusReviewChan chan int

	uriChan chan string
}

func (ctx *gigantickContext) init() error {
	ctx.p4 = p4lib.New()

	// get username without domain
	current, err := user.Current()
	if err != nil {
		return err
	}
	lastSlash := strings.LastIndex(current.Username, "\\") + 1
	ctx.username = string(current.Username[lastSlash:])

	ctx.localAppPath = "code"
	ctx.localAppArgs = "-g $(file_name):$(line)"

	ctx.allChanges = make(map[int]*gigantickChange)
	ctx.allFileData = make(map[string]*fileData)
	ctx.allFileDiffs = make(map[filePair]*filePairDiff)
	ctx.allReviews = make(map[int]*swarm.Review)
	ctx.allTickets = make(map[string]*p4lib.Ticket)

	ctx.changeToReviewIdMap = make(map[int]int)

	ctx.infoChan = make(chan p4lib.Info)
	ctx.usersChan = make(chan []p4lib.User)
	ctx.ticketsChan = make(chan []p4lib.Ticket)

	ctx.pendingChangesChan = make(chan []p4lib.Change)
	ctx.submittedChangesChan = make(chan []p4lib.Change)

	ctx.dashboardReviewsChan = make(chan []swarm.Review)
	ctx.dashboardChangesChan = make(chan []p4lib.Change)

	ctx.focusChangeChan = make(chan int)
	ctx.focusReviewChan = make(chan int)

	ctx.uriChan = make(chan string)

	go fetchUsers(ctx)
	go fetchTickets(ctx)
	go fetchInfo(ctx)
	go fetchPending(ctx)
	go fetchSubmitted(ctx)

	return nil
}

func (ctx *gigantickContext) changeAdd(change *gigantickChange) *gigantickChange {
	old, ok := ctx.allChanges[change.changeList]
	if ok {
		return old
	}

	ctx.allChanges[change.changeList] = change
	return change
}

func updateChangesDashboard(ctx *gigantickContext) {
	select {
	case dchanges := <-ctx.dashboardChangesChan:
		for i := range dchanges {
			gc := newGigantickChange(&dchanges[i])
			ctx.changeAdd(gc)
		}
	default:
	}
}

func updateFileData(ctx *gigantickContext) {
	for _, v := range ctx.allFileData {
		select {
		case v.data = <-v.dataChan:
			v.state = opStateDone
		default:
		}
	}
}

func updateFileDiffs(ctx *gigantickContext) {
	for _, v := range ctx.allFileDiffs {
		select {
		case v.diffs = <-v.diffsChan:
			v.state = opStateDone
		default:
		}
	}
}

func updateFocusChange(ctx *gigantickContext) {
	select {
	case c := <-ctx.focusChangeChan:
		ctx.focusChanges = append(ctx.focusChanges, c)
	default:
	}
}

func updateFocusReview(ctx *gigantickContext) {
	select {
	case _ = <-ctx.focusReviewChan:
	default:
	}
}

func updateInfo(ctx *gigantickContext) {
	select {
	case ctx.info = <-ctx.infoChan:
	default:
	}
}

func updateReviewsDashboard(ctx *gigantickContext) {
	select {
	case dashReviews := <-ctx.dashboardReviewsChan:
		var dchanges []int
		for i := range dashReviews {
			for j := range dashReviews[i].Changes {
				dchanges = append(dchanges, dashReviews[i].Changes[j])
			}
			id := dashReviews[i].ID
			ctx.dashboardReviews = append(ctx.dashboardReviews, id)
			ctx.allReviews[id] = &dashReviews[i]
		}
		if len(dchanges) > 0 {
			go func() {
				defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "dashboard changes", ""))
				c, err := ctx.p4.Changes("-s", "pending")
				if err != nil {
					glog.Errorf("p4 changes failed: %v", err)
					return
				}
				ctx.dashboardChangesChan <- c
			}()
		}
	default:
	}
}

func updateTickets(ctx *gigantickContext) {
	select {
	case tickets := <-ctx.ticketsChan:
		for i := range tickets {
			ctx.allTickets[tickets[i].User] = &tickets[i]
		}
		ctx.ticketsDone = true
	default:
	}
}

func updateUri(ctx *gigantickContext) {
	select {
	case u := <-ctx.uriChan:
		fromLink(ctx, u)
	default:
	}
}

func updateUsers(ctx *gigantickContext) {
	select {
	case ctx.users = <-ctx.usersChan:
	default:
	}
}

func (ctx *gigantickContext) update() {
	select {
	case changes := <-ctx.pendingChangesChan:
		for i := range changes {
			gc := newGigantickChange(&changes[i])
			gc.getFiles(ctx)
			ctx.changeAdd(gc)
			go fetchDescription(ctx, gc)
			ctx.pendingChanges = append(ctx.pendingChanges, gc.changeList)
		}
		ctx.pendingChangesDone = true
	default:
	}

	select {
	case changes := <-ctx.submittedChangesChan:
		for i := range changes {
			gc := newGigantickChange(&changes[i])
			ctx.changeAdd(gc)
			ctx.submittedChanges = append(ctx.submittedChanges, gc.changeList)
			//			go fetchDescription(ctx, gc)
		}
		ctx.submittedChangesDone = true
	default:
	}

	updateChangesDashboard(ctx)
	updateFileData(ctx)
	updateFileDiffs(ctx)
	updateFocusChange(ctx)
	updateFocusReview(ctx)
	updateInfo(ctx)
	updateReviewsDashboard(ctx)
	updateTickets(ctx)
	updateUri(ctx)
	updateUsers(ctx)

	if !ctx.swarmDone {
		if ctx.pendingChangesDone && ctx.submittedChangesDone && ctx.ticketsDone {
			swarmInit(ctx)
			ctx.swarmDone = true
		}
	}

	for _, v := range ctx.allChanges {
		v.update(ctx)
	}

}

func fetchReviewsForChangelists(ctx *gigantickContext, changes []int) {
	for _, changeNum := range changes {
		ch, ok := ctx.allChanges[changeNum]
		if !ok {
			glog.Warningf("change not found: %d", changeNum)
			continue
		}
		ch.reviewsState = opStateProcessing
		go fetchReviewForChangelist(ctx, ch)
	}
}

func fetchActionDashboard(ctx *gigantickContext) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "action dashboard", ctx.username))
	r, err := swarm.GetActionDashboard(&ctx.swarm)
	if err != nil {
		glog.Errorf("can't retrieve action dashboard: %v", err)
		return
	}
	ctx.dashboardReviewsChan <- r
}

func fetchFileData(ctx *gigantickContext, fd *fileData) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "print", fd.filename))
	s, err := ctx.p4.Print(fd.filename)
	if err != nil {
		glog.Errorf("p4 print failed: %v", err)
		return
	}
	fd.dataChan <- s
}

func fetchFileDiffs(ctx *gigantickContext, fd *filePairDiff) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "diff2", fmt.Sprintf("%s <-> %s", fd.pair.left, fd.pair.right)))
	d, err := ctx.p4.Diff2(fd.pair.left, fd.pair.right)
	if err != nil {
		glog.Errorf("p4 diff err: %v\n", err)
		return
	}
	fd.diffsChan <- d
}

func fetchInfo(ctx *gigantickContext) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "info", ""))
	i, _ := ctx.p4.Info()
	ctx.infoChan <- *i
}

func fetchPending(ctx *gigantickContext) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "pending", ctx.username))
	c, err := ctx.p4.Changes("-u", ctx.username, "-l", "-s", "pending")
	if err != nil {
		glog.Errorf("err getting pending changes: %v", err)
		return
	}
	ctx.pendingChangesChan <- c
}

func fetchReviewForChangelist(ctx *gigantickContext, ch *gigantickChange) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "swarm getreviews", fmt.Sprintf("%d", ch.changeList)))
	r, err := swarm.GetReviewsForChangelists(&ctx.swarm, []int{ch.changeList})
	if err != nil {
		glog.Warningf("could not get swarm reviews: %v", err)
		return
	}
	ch.reviewsChan <- r
}

func fetchSubmitted(ctx *gigantickContext) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "submitted", ctx.username))
	c, err := ctx.p4.Changes("-u", ctx.username, "-l", "-s", "submitted")
	if err != nil {
		glog.Errorf("could not get submitted changes: %v", err)
		return
	}
	ctx.submittedChangesChan <- c
}

func fetchTickets(ctx *gigantickContext) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "tickets", ""))
	t, _ := ctx.p4.Tickets()
	ctx.ticketsChan <- t
}

func fetchUsers(ctx *gigantickContext) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "users", ""))
	u, _ := ctx.p4.Users()
	ctx.usersChan <- u
}

func getFileData(ctx *gigantickContext, filename string) (string, error) {
	fd, ok := ctx.allFileData[filename]
	if !ok {
		fd = &fileData{filename: filename, dataChan: make(chan string)}
		ctx.allFileData[filename] = fd
		go fetchFileData(ctx, fd)
		return "", fmt.Errorf("not loaded")
	}
	if fd.state == opStateDone {
		return fd.data, nil
	}
	return "", fmt.Errorf("not loaded")
}

func getFileDiffs(ctx *gigantickContext, fp filePair) (*filePairDiff, error) {
	fd, ok := ctx.allFileDiffs[fp]
	if !ok {
		fd = &filePairDiff{pair: fp, diffsChan: make(chan []p4lib.Diff)}
		ctx.allFileDiffs[fp] = fd
		go fetchFileDiffs(ctx, fd)
		return nil, fmt.Errorf("not loaded")
	}
	if fd.state == opStateDone {
		return fd, nil
	}
	return nil, fmt.Errorf("not loaded")
}

func swarmInit(ctx *gigantickContext) error {
	ticket, ok := ctx.allTickets[gContext.username]
	if !ok {
		return fmt.Errorf("couldn't find p4 ticket for user")
	}

	ctx.swarm = swarm.Context{
		Host:     swarmHost,
		Port:     9000,
		Username: gContext.username,
		Password: ticket.ID,
	}

	fetchReviewsForChangelists(ctx, ctx.pendingChanges)
	fetchReviewsForChangelists(ctx, ctx.submittedChanges)

	go fetchActionDashboard(ctx)

	return nil
}

func (g *gigantickFile) commentsExtracts(comments []swarm.Comment) {
	g.comments = nil
	for i, _ := range comments {
		if comments[i].Context.File == g.fstat.DepotFile {
			g.comments = append(g.comments, &comments[i])
		}
	}
}

func main() {
	gContext.uriArg = flag.String("uri", "", "uri argument")
	changes := flag.String("change", "", "changelist to display")
	gContext.port = flag.String("port", "u", "changelist to display")
	gContext.user = flag.String("user", "u", "changelist to display")
	gContext.workspace = flag.String("workspace", "u", "changelist to display")
	// glog to both stderr and to file
	flag.Set("alsologtostderr", "true")
	if ad, err := files.GetAppDir("sge", appName); err == nil {
		// set directory for glog to %APPDATA%/sge/gigantick
		flag.Set("log_dir", ad)
	}
	flag.Parse()
	glog.Info("application start")
	glog.Infof("%v", os.Args)

	if err := gContext.init(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if err := install(); err != nil {
		glog.Errorf("error installing p4v custom tools: %v", err)
	}
	if err := serverInit(&gContext); err != nil {
		// whilst some functionality is lost without the server, it shouldn't block rest of program from running
		glog.Errorf("couldn't start server: %v", err)
	}
	fromLink(&gContext, *gContext.uriArg)
	c, err := strconv.Atoi(strings.TrimSpace(*changes))
	if err == nil && c != 0 {
		gContext.focusChangeNew = c
	}
	wnd := giu.NewMasterWindow("Gigantick", 1920, 1080, 0, loadFonts)
	wnd.Main(loop)

	glog.Info("application exit")
	glog.Flush()
}
