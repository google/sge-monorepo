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
	"fmt"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"sge-monorepo/build/cicd/cicdfile"
	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/monorepo/universe"
	"sge-monorepo/build/cicd/presubmit"
	"sge-monorepo/build/cicd/presubmit/protos/presubmitpb"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/swarm"

	"github.com/golang/glog"
)

type gigantickChange struct {
	changeList  int
	change      *p4lib.Change
	description p4lib.Description
	sgepOutput  string

	commentsChange        []*swarm.Comment
	fileCommentsExtracted bool

	commentsAll swarm.CommentCollection
	review      swarm.Review

	leftVersionIndex  int
	rightVersionIndex int

	commentsState opState
	difState      opState
	fixState      opState
	reviewsState  opState
	sgepState     opState

	fixResult error
	fixer     *buildpb.Result
	results   []*presubmitpb.CheckResult
	checks    []presubmit.Check

	files     []gigantickFile
	fileState opState

	commentsChan   chan swarm.CommentCollection
	reviewsChan    chan swarm.ReviewCollection
	fstatChan      chan p4lib.FstatResult
	describesChan  chan []p4lib.Description
	sgepResultChan chan *presubmitpb.CheckResult
	sgepChecksChan chan []presubmit.Check
	sgepChan       chan string
	sgepFixChan    chan error
}

type stringWriter struct {
	external *string
}

func (s *stringWriter) Write(data []byte) (n int, err error) {
	*s.external += string(data)
	return len(data), nil
}

func newGigantickChange(ch *p4lib.Change) *gigantickChange {
	gc := &gigantickChange{
		changeList:     ch.Cl,
		change:         ch,
		fstatChan:      make(chan p4lib.FstatResult),
		reviewsChan:    make(chan swarm.ReviewCollection),
		commentsChan:   make(chan swarm.CommentCollection),
		describesChan:  make(chan []p4lib.Description),
		sgepChan:       make(chan string),
		sgepResultChan: make(chan *presubmitpb.CheckResult),
		sgepChecksChan: make(chan []presubmit.Check),
		sgepFixChan:    make(chan error),
	}
	return gc
}

func fetchChangeComments(ctx *gigantickContext, g *gigantickChange) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "swarm getcomments", fmt.Sprintf("%d", g.review.ID)))
	com, err := swarm.GetCommentsForReview(&ctx.swarm, g.review.ID)
	if err != nil {
		glog.Warningf("couldn't get comments for review %d: %v", g.review.ID, err)
		return
	}
	g.commentsChan <- com
}

func fetchDescription(ctx *gigantickContext, g *gigantickChange) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "describe", fmt.Sprintf("%d", g.changeList)))
	d, err := ctx.p4.Describe([]int{g.changeList})
	if err != nil {
		glog.Errorf("p4 describe failed: %v", err)
		return
	}
	g.describesChan <- d
}

type sgepCollector struct {
	change     *gigantickChange
	success    bool
	checkCount int
	checkPass  int
}

func (p *sgepCollector) OnPresubmitStart(mr monorepo.Monorepo, presubmitId string, checks []presubmit.Check) {
	p.change.sgepChecksChan <- checks
}

func (p *sgepCollector) OnCheckStart(presubmit.Check) {}

func (p *sgepCollector) OnCheckResult(mdPath monorepo.Path, check presubmit.Check, result *presubmitpb.CheckResult) {
	success := result.OverallResult.Success
	p.success = p.success && success
	p.checkCount++
	if success {
		p.checkPass++
	}
	go func() {
		p.change.sgepResultChan <- result
	}()
}

func (p *sgepCollector) OnPresubmitEnd(success bool) {}

func fetchSgep(ctx *gigantickContext, g *gigantickChange) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "sgep", fmt.Sprintf("%d", g.changeList)))
	startTime := time.Now()
	u, err := universe.New()
	if err != nil {
		glog.Errorf("couldn't create universe: %v", err)
		return
	}
	collector := &sgepCollector{change: g}
	runner := presubmit.NewRunner(u, ctx.p4, cicdfile.NewProvider(), func(options *presubmit.Options) {
		options.Change = fmt.Sprintf("%d", g.changeList)
		options.Listeners = append(options.Listeners, collector)
	})
	success, err := runner.Run()
	if err != nil {
		glog.Errorf("presubmit run failure: %v", err)
		return
	}
	msg := "SUCCEEDED"
	if !success {
		msg = "FAILED"
	}
	failCount := collector.checkCount - collector.checkPass
	d := time.Now().Sub(startTime)
	g.sgepChan <- fmt.Sprintf("%s %d tests %d failed [%.1f seconds]", msg, collector.checkCount, failCount, d.Seconds())
}

func fetchFstats(ctx *gigantickContext, ch *gigantickChange) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "fstat", fmt.Sprintf("%d", ch.changeList)))

	changeText := fmt.Sprintf("%d", ch.changeList)
	args := []string{"-e", changeText}
	if ch.change.Status == "pending" {
		args = append(args, "-Rs")
	}
	args = append(args, "//...")
	fs, err := ctx.p4.Fstat(args...)

	if err == nil && ch.change.Status == "pending" && len(fs.FileStats) == 0 {
		fs, err = ctx.p4.Fstat([]string{"-e", changeText, "-Ro", "//..."}...)
	}

	if err != nil {
		glog.Errorf("fstat error: %v", err)
		return
	}
	ch.fstatChan <- *fs
}

func fetchFix(ctx *gigantickContext, ch *gigantickChange, fix string) {
	defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "fix", fix))
	parts := strings.Split(fix, " ")
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	ch.sgepFixChan <- cmd.Run()
}

func updateChangeComments(ctx *gigantickContext, g *gigantickChange) {
	select {
	case g.commentsAll = <-g.commentsChan:
		g.commentsState = opStateDone
	default:
	}
}

func updateChangeDescription(ctx *gigantickContext, g *gigantickChange) {
	select {
	case ds := <-g.describesChan:
		if len(ds) > 0 {
			g.description = ds[0]
		}
	default:
	}
}

func updateChangeFstats(ctx *gigantickContext, g *gigantickChange) {
	select {
	case fs := <-g.fstatChan:
		g.files = make([]gigantickFile, len(fs.FileStats))
		for i := range fs.FileStats {
			g.files[i].fstat = fs.FileStats[i]
		}
		g.fileState = opStateDone
	default:
	}
}

func updateChangeReviews(ctx *gigantickContext, g *gigantickChange) {
	select {
	case reviews := <-g.reviewsChan:
		if reviews.TotalCount > 0 {
			g.review = reviews.Reviews[0]
			if len(g.review.Changes) > 1 {
				g.rightVersionIndex = 2 + len(g.review.Changes) - 2
			}
			g.commentsState = opStateProcessing
			go fetchChangeComments(ctx, g)
		}
		g.reviewsState = opStateDone
	default:
	}
}

func updateChangeSgep(ctx *gigantickContext, g *gigantickChange) {
	select {
	case g.fixResult = <-g.sgepFixChan:
		g.fixState = opStateDone
	default:
	}

	select {
	case g.checks = <-g.sgepChecksChan:
	default:
	}

	select {
	case r := <-g.sgepResultChan:
		g.results = append(g.results, r)
	default:
	}

	select {
	case g.sgepOutput = <-g.sgepChan:
		g.sgepState = opStateDone
	default:
	}
}

func (g *gigantickChange) update(ctx *gigantickContext) {
	updateChangeComments(ctx, g)
	updateChangeFstats(ctx, g)
	updateChangeReviews(ctx, g)
	updateChangeDescription(ctx, g)
	updateChangeSgep(ctx, g)

	// wait until both files and comments have been obtained before processing
	if !g.fileCommentsExtracted && g.commentsState == opStateDone && g.fileState == opStateDone {
		// extract file specific comments
		g.build(ctx)
		g.fileCommentsExtracted = true
	}
}

func (g *gigantickChange) build(ctx *gigantickContext) {
	for i := range g.files {
		g.files[i].commentsExtracts(g.commentsAll.Comments)
	}
	// comments without a file are changelist level comments, extract these
	g.commentsChange = nil
	for i := range g.commentsAll.Comments {
		if len(g.commentsAll.Comments[i].Context.File) == 0 {
			g.commentsChange = append(g.commentsChange, &g.commentsAll.Comments[i])
		}
	}
}

func (g *gigantickChange) getDiffs(ctx *gigantickContext, f *gigantickFile) (*fileDiff, error) {
	fp, err := g.getFilePair(ctx, f)
	if err != nil {
		return nil, err
	}
	fpd, err := getFileDiffs(ctx, *fp)
	if err != nil {
		return nil, err
	}

	left, err := getFileData(ctx, fp.left)
	if err != nil {
		return nil, err
	}

	right, err := getFileData(ctx, fp.right)
	if err != nil {
		return nil, err
	}

	fpd.fdiff.build(ctx, g.commentsAll.Comments, fpd.diffs, left, right)

	return &fpd.fdiff, nil
}

func (g *gigantickChange) getFilePair(ctx *gigantickContext, f *gigantickFile) (*filePair, error) {
	if g.fileState != opStateDone {
		return nil, fmt.Errorf("files not loaded")
	}
	if g.reviewsState != opStateDone {
		return nil, fmt.Errorf("files not loaded")
	}

	df := f.fstat.DepotFile

	chRev := f.fstat.HeadRev
	prevRev := chRev

	left := ""
	right := ""

	switch g.leftVersionIndex {
	case 0:
		left = fmt.Sprintf("%s#%d", df, prevRev)
	case 1:
		left = df
	default:
		left = fmt.Sprintf("%s@=%d", df, g.review.Changes[g.leftVersionIndex-1])
	}

	switch g.rightVersionIndex {
	case 0:
		right = fmt.Sprintf("%s#%d", df, prevRev)
	case 1:
		right = df
	default:
		right = fmt.Sprintf("%s@=%d", df, g.review.Changes[g.rightVersionIndex-1])
	}

	return &filePair{
		left:  left,
		right: right,
	}, nil
}

func (g *gigantickChange) getFiles(ctx *gigantickContext) []gigantickFile {
	if g.fileState != opStateNone {
		return g.files
	}
	g.fileState = opStateProcessing
	go fetchFstats(ctx, g)
	return g.files
}

func (g *gigantickChange) buildSgep(ctx *gigantickContext) error {
	if g.sgepState == opStateProcessing {
		return fmt.Errorf("already running sgep")
	}
	g.sgepState = opStateProcessing
	g.sgepOutput = ""
	g.checks = nil
	g.results = nil
	go fetchSgep(ctx, g)
	return nil
}

func (g *gigantickChange) getDiffTotalCount(ctx *gigantickContext) diffTotal {
	var total diffTotal
	/*
		for i := range g.files {
			if d := g.files[i].getDiffs(ctx, g); d != nil {
				total.addCount += d.diffTotal.addCount
				total.deleteCount += d.diffTotal.deleteCount
				total.changeCount += d.diffTotal.changeCount
			}
		}
	*/
	return total
}
