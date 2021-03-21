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
	"log"
	"strings"
	"time"

	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/swarm"
)

type lineType int

const (
	lineTypeCode = iota
	lineTypeCommentHeader
	lineTypeCommentBody
	lineTypeCommentFooter
	lineTypeExpander
)

type diffLine struct {
	lineIndex int
	content   string
	diffType  p4lib.DiffType
}

type diffDisplayLine struct {
	lineType   lineType
	subContent string
	subIndex   int
	comment    *swarm.Comment
	side       int
	isAuthor   bool
}

type diffTotal struct {
	addCount    int
	deleteCount int
	changeCount int
}

type fileDiff struct {
	files filePair
	left  []string
	right []string

	leftLines  []diffLine
	rightLines []diffLine

	displayLines []diffDisplayLine

	diffTotal diffTotal

	state   opState
	rebuild bool
}

type diffSpan struct {
	start int
	end   int
}

type diffSpanner struct {
	spans []diffSpan
}

func (g *diffSpanner) insert(start int, end int) {
	if start > end {
		start, end = end, start
	}

	// we extend the area of interest by +-5 lines around diff
	start -= 5
	if start < 0 {
		start = 0
	}
	end += 5

	// find where to insert span in list of spans
	for i := range g.spans {
		// attempt to merge span with existing span
		if (end >= g.spans[i].start && start <= g.spans[i].end) || (start >= g.spans[i].start && start <= g.spans[i].end) {
			if start < g.spans[i].start {
				g.spans[i].start = start
			}
			if end > g.spans[i].end {
				g.spans[i].end = end
			}
			g.validate()
			return
		}
		// if span is before current span, insert and finish
		if (i > 0) && end < g.spans[i].start && start > g.spans[i-1].end {
			g.spans = append(g.spans, diffSpan{})
			copy(g.spans[i+1:], g.spans[i:])
			g.spans[i] = diffSpan{start: start, end: end}
			g.validate()
			return
		}
	}
	// span must be at end of list
	g.spans = append(g.spans, diffSpan{start: start, end: end})
	g.validate()
}

func (g *diffSpanner) validate() {
	for i := range g.spans {
		if g.spans[i].start > g.spans[i].end {
			log.Fatalf("gspans mixup")
		}
		if i > 0 {
			if g.spans[i-1].end > g.spans[i].start {
				log.Fatalf("gspans mixup")
			}
		}
	}
}

func (g *fileDiff) build(ctx *gigantickContext, comments []swarm.Comment, diffs []p4lib.Diff, leftData string, rightData string) {
	if g.state == opStateDone {
		return
	}

	g.left = strings.Split(leftData, "\n")
	g.right = strings.Split(rightData, "\n")

	cursorL := 0
	cursorR := 0

	checkConsistency := func() {
		last := 0
		for i, dl := range g.displayLines {
			if dl.lineType == lineTypeCode {
				if dl.subIndex < last {
					log.Fatalf("out of whack at line %d", i)
				}
				last = dl.subIndex
			}
		}
	}

	for i := range diffs {
		ldiff := diffs[i].LeftEndLine - diffs[i].LeftStartLine
		rdiff := diffs[i].RightEndLine - diffs[i].RightStartLine
		dmax := ldiff
		if rdiff > dmax {
			dmax = rdiff
		}
		dmax += 1
		switch diffs[i].DiffType {
		case p4lib.DiffAdd:
			g.diffTotal.addCount += dmax
		case p4lib.DiffChange:
			g.diffTotal.changeCount += dmax
		case p4lib.DiffDelete:
			g.diffTotal.deleteCount += dmax
		}
	}

	max := 0
	for _, d := range diffs {
		if d.LeftEndLine > max {
			max = d.LeftEndLine
		}
		if d.RightEndLine > max {
			max = d.RightEndLine
		}
	}
	for len(g.left) <= max {
		g.left = append(g.left, "")
	}
	for len(g.right) <= max {
		g.right = append(g.right, "")
	}

	var spanner diffSpanner

	g.leftLines = nil
	g.rightLines = nil

	for _, d := range diffs {
		for ; cursorL < d.LeftStartLine; cursorL++ {
			g.leftLines = append(g.leftLines, diffLine{lineIndex: cursorL, content: g.left[cursorL], diffType: p4lib.DiffNone})
		}
		for ; cursorR < d.RightStartLine; cursorR++ {
			g.rightLines = append(g.rightLines, diffLine{lineIndex: cursorR, content: g.right[cursorR], diffType: p4lib.DiffNone})
		}

		start := len(g.leftLines)

		switch d.DiffType {
		case p4lib.DiffAdd:
			for ; cursorL <= d.LeftStartLine; cursorL++ {
				g.leftLines = append(g.leftLines, diffLine{lineIndex: cursorL, content: g.left[cursorL], diffType: p4lib.DiffNone})
			}
			for ; cursorR <= d.RightEndLine; cursorR++ {
				g.leftLines = append(g.leftLines, diffLine{diffType: p4lib.DiffDelete})
				g.rightLines = append(g.rightLines, diffLine{lineIndex: cursorR, content: g.right[cursorR], diffType: p4lib.DiffAdd})
			}
		case p4lib.DiffChange:
			for cursorL <= d.LeftEndLine || cursorR <= d.RightEndLine {
				if cursorL <= d.LeftEndLine {
					g.leftLines = append(g.leftLines, diffLine{lineIndex: cursorL, content: g.left[cursorL], diffType: p4lib.DiffChange})
					cursorL++
				} else {
					g.leftLines = append(g.leftLines, diffLine{diffType: p4lib.DiffChange})
				}
				if cursorR <= d.RightEndLine {
					g.rightLines = append(g.rightLines, diffLine{lineIndex: cursorR, content: g.right[cursorR], diffType: p4lib.DiffChange})
					cursorR++
				} else {
					g.rightLines = append(g.rightLines, diffLine{diffType: p4lib.DiffChange})
				}
			}
		case p4lib.DiffDelete:
			for ; cursorR <= d.RightStartLine; cursorR++ {
				g.rightLines = append(g.rightLines, diffLine{lineIndex: cursorR, content: g.right[cursorR], diffType: p4lib.DiffNone})
			}
			for ; cursorL <= d.LeftEndLine; cursorL++ {
				g.leftLines = append(g.leftLines, diffLine{lineIndex: cursorL, content: g.left[cursorL], diffType: p4lib.DiffAdd})
				g.rightLines = append(g.rightLines, diffLine{diffType: p4lib.DiffDelete})
			}
		}

		end := len(g.leftLines)
		spanner.insert(start, end)
	}
	checkConsistency()

	g.displayLines = nil
	for _, s := range spanner.spans {
		if len(g.displayLines) == 0 || g.displayLines[len(g.displayLines)-1].lineType != lineTypeExpander {
			if s.start > 1 {
				g.displayLines = append(g.displayLines, diffDisplayLine{
					lineType: lineTypeExpander,
					subIndex: s.start - 1,
				})
			}
		}
		for j := s.start; j <= s.end; j++ {
			if j > 0 {
				if j < len(g.leftLines) {
					g.displayLines = append(g.displayLines, diffDisplayLine{
						lineType: lineTypeCode,
						subIndex: j,
					})
				}
			}
		}
		if s.end < len(g.leftLines) {
			g.displayLines = append(g.displayLines, diffDisplayLine{
				lineType: lineTypeExpander,
				subIndex: s.end + 1,
			})
		}
	}
	checkConsistency()

	finder := func(c *swarm.Comment) int {
		for i := range g.displayLines {
			if g.displayLines[i].lineType == lineTypeCode {
				if c.Context.RightLine > 0 && g.displayLines[i].subIndex >= c.Context.RightLine ||
					c.Context.LeftLine > 0 && g.displayLines[i].subIndex >= c.Context.LeftLine {
					return i
				}
			}
		}
		return len(g.displayLines)
	}

	for i := len(comments) - 1; i >= 0; i-- {
		var comSlices []diffDisplayLine

		isAuthor := ctx.username == comments[i].User

		side := 1
		if comments[i].Context.LeftLine != 0 {
			side = 0
		}

		ts := time.Unix(int64(comments[i].Time), 0)
		comSlices = append(comSlices, diffDisplayLine{
			lineType:   lineTypeCommentHeader,
			subContent: fmt.Sprintf("%s %s", ts.Format("2006-01-02 15:04:05"), comments[i].User),
			side:       side,
			isAuthor:   isAuthor,
		})

		var comLines []string
		comLinesMain := strings.Split(comments[i].Body, "\n")
		sline := ""
		for _, c := range comLinesMain {
			if len(c) > 80 {
				words := strings.Split(c, " ")
				for _, w := range words {
					if len(w) > 80 {
						if len(sline) > 0 {
							comLines = append(comLines, sline)
							comLines = append(comLines, w)
							sline = ""
						}
					} else if len(w)+len(sline) > 80 {
						comLines = append(comLines, sline)
						sline = w
					} else {
						if len(sline) > 0 {
							sline += " "
						}
						sline += w
					}
				}
			} else {
				comLines = append(comLines, c)
			}
		}
		if len(sline) > 0 {
			comLines = append(comLines, sline)
		}

		for j, com := range comLines {
			comSlices = append(comSlices, diffDisplayLine{
				lineType:   lineTypeCommentBody,
				subContent: com,
				subIndex:   j,
				side:       side,
				isAuthor:   isAuthor,
			})
		}

		comSlices = append(comSlices, diffDisplayLine{
			lineType: lineTypeCommentFooter,
			side:     side,
			comment:  &comments[i],
			isAuthor: isAuthor,
		})

		index := finder(&comments[i]) + 1
		if index >= len(g.displayLines) {
			g.displayLines = append(g.displayLines, comSlices...)
		} else {
			for _ = range comSlices {
				g.displayLines = append(g.displayLines, diffDisplayLine{})
			}
			copy(g.displayLines[index+len(comSlices):], g.displayLines[index:])
			for k := range comSlices {
				g.displayLines[k+index] = comSlices[k]
			}
		}
	}
	checkConsistency()

	g.state = opStateProcessing
	g.rebuild = false
}
