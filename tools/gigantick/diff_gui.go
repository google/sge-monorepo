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
	"time"

	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/swarm"

	"github.com/AllenDang/giu/imgui"
	"github.com/golang/glog"
)

func diffColor(dis *diffDisplayLine, d p4lib.DiffType) imgui.Vec4 {
	switch dis.lineType {
	case lineTypeExpander:
		return imgui.Vec4{0.2, 0.2, 0.2, 1}
	case lineTypeCommentHeader:
		return imgui.Vec4{0.4, 0.4, 0.4, 1}
	case lineTypeCommentBody:
		return imgui.Vec4{0.3, 0.3, 0.3, 1}
	case lineTypeCommentFooter:
		return imgui.Vec4{0.3, 0.3, 0.3, 1}
	}

	switch d {
	case p4lib.DiffNone:
		return imgui.Vec4{0, 0, 0, 1}
	case p4lib.DiffAdd:
		return imgui.Vec4{0, 0.2, 0, 1}
	case p4lib.DiffChange:
		return imgui.Vec4{0, 0, 0.2, 1}
	case p4lib.DiffDelete:
		return imgui.Vec4{0.2, 0, 0, 1}
	}
	return imgui.Vec4{0, 0, 0, 1}
}

func diffLineView(ctx *gigantickContext, change *gigantickChange, diff *fileDiff, lineIndex int, fileIndex int, side int) {
	dis := &diff.displayLines[lineIndex]
	diffLines := diff.leftLines
	if side == 1 {
		diffLines = diff.rightLines
	}
	if nil == diffLines {
		return
	}
	line := diffLines[dis.subIndex]
	imgui.PushStyleColor(imgui.StyleColorChildBg, diffColor(dis, line.diffType))

	newCommenter := func() {
		ctx.newComment = swarm.Comment{}
		ctx.newComment.Topic = fmt.Sprintf("reviews/%d", change.review.ID)
		ctx.newComment.Context.File = change.files[fileIndex].fstat.DepotFile
		ctx.newComment.User = ctx.username
		findr := func() int {
			for i := lineIndex; i > 0; i-- {
				if diff.displayLines[i].lineType == lineTypeCode {
					return diff.displayLines[i].subIndex
				}
			}
			return 0
		}
		lix := findr()
		if side == 0 {
			ctx.newComment.Context.LeftLine = lix
		} else {
			ctx.newComment.Context.RightLine = lix
		}
	}

	diffPopup := func(title string, contents *string, onOk func()) {
		imgui.SetNextWindowSize(imgui.Vec2{X: 1024, Y: 280})
		if imgui.BeginPopupModal(title) {
			imgui.PushItemWidth(-1)
			imgui.InputTextMultiline("###comment", contents)
			imgui.PopItemWidth()
			imgui.InvisibleButton("", imgui.Vec2{X: 16, Y: 8})
			if imgui.Button("Ok") {
				onOk()
				imgui.CloseCurrentPopup()
			}
			imgui.SameLine()
			imgui.InvisibleButton("", imgui.Vec2{X: 16, Y: 10})
			imgui.SameLine()
			if imgui.Button("Cancel") {
				imgui.CloseCurrentPopup()
			}
			imgui.EndPopup()
		}
	}

	imgui.BeginChildV(fmt.Sprintf("%d_%d_%p", side, dis.subIndex, dis), imgui.Vec2{X: imgui.WindowSize().X, Y: 26}, false, 0)
	switch dis.lineType {
	case lineTypeCode:
		imgui.Text(fmt.Sprintf("%d", line.lineIndex))
		imgui.SameLine()
		imgui.Text(line.content)
		if imgui.BeginPopupContextItemV("Code Options", 1) {
			if imgui.MenuItem("Open in IDE") {
				showFileLocal(ctx, change.files[fileIndex].fstat.DepotFile, line.lineIndex)
			}
			if imgui.MenuItem("Open in Swarm") {
				showFile(ctx, change.files[fileIndex].fstat.DepotFile, line.lineIndex)
			}
			if imgui.MenuItem(fmt.Sprintf("Add Comment To Line %d", line.lineIndex)) {
				imgui.CloseCurrentPopup()
				newCommenter()
				imgui.OpenPopup("Add Comment")
			}
			imgui.EndPopup()
		}

		diffPopup("Add Comment", &ctx.newComment.Body, func() {
			go func(comment *swarm.Comment) {
				defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "swarm comment add", fmt.Sprintf("%d - %d", ctx.newComment.Context.LeftLine, ctx.newComment.Context.RightLine)))
				if err := swarm.AddComment(&ctx.swarm, comment); err != nil {
					glog.Warningf("could not add swarm comment: %v", err)
				}
			}(&ctx.newComment)
			change.fileCommentsExtracted = false
			change.commentsAll.Comments = append(change.commentsAll.Comments, ctx.newComment)
		})
	case lineTypeCommentHeader:
		if dis.side == side {
			imgui.Text(dis.subContent)
		}
	case lineTypeCommentFooter:
		if dis.side == side {
			if dis.isAuthor {
				if imgui.Button("Edit") {
					imgui.OpenPopup("Edit Comment")
				}
				diffPopup("Edit Comment", &dis.comment.Body, func() {
					//					change.files[fileIndex].diff.rebuild = true
					go func(comment *swarm.Comment) {
						defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "swarm comment update", fmt.Sprintf("%d", dis.comment.ID)))
						if err := swarm.UpdateComment(&ctx.swarm, comment); err != nil {
							glog.Warningf("could not update swarm comment: %v", err)
						}
					}(dis.comment)
				})
				imgui.SameLine()
				imgui.InvisibleButton("", imgui.Vec2{X: 16, Y: 10})
				imgui.SameLine()
			}
			if imgui.Button("Reply") {
				imgui.OpenPopup("Reply Comment")
				newCommenter()
			}
			diffPopup("Reply Comment", &ctx.newComment.Body, func() {
				//				change.files[fileIndex].diff.rebuild = true
				go func(comment *swarm.Comment) {
					defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "swarm comment reply", fmt.Sprintf("%d - %d", ctx.newComment.Context.LeftLine, ctx.newComment.Context.RightLine)))
					if err := swarm.AddComment(&ctx.swarm, comment); err != nil {
						glog.Warningf("could not add swarm comment: %v", err)
					}
				}(&ctx.newComment)
				change.fileCommentsExtracted = false
				change.commentsAll.Comments = append(change.commentsAll.Comments, ctx.newComment)
				//				change.files[fileIndex].diff.rebuild = true
			})
		}

	case lineTypeCommentBody:
		if dis.side == side {
			imgui.Text(dis.subContent)
		}
	default:
		imgui.Text("...")
	}
	imgui.EndChild()

	imgui.PopStyleColor()
}

func diffView(ctx *gigantickContext, change *gigantickChange, fileIndex int) error {
	diff, err := change.getDiffs(ctx, &change.files[fileIndex])
	if err != nil {
		imgui.Text("loading " + animatingDots())
		return nil
	}

	imgui.ColumnsV(2, "dc", true)
	var lc imgui.ListClipper
	ty := imgui.CursorPosY()

	imgui.PushStyleVarVec2(imgui.StyleVarItemSpacing, imgui.Vec2{X: 8, Y: 0})

	lc.Begin(len(diff.displayLines))
	for lc.Step() {
		for i := lc.DisplayStart; i < lc.DisplayEnd; i++ {
			//			diffLineView(ctx, change, &diff.displayLines[i], diff.leftLines, fileIndex, 0)
			diffLineView(ctx, change, diff, i, fileIndex, 0)
		}
	}
	imgui.NextColumn()
	imgui.SetCursorPos(imgui.Vec2{X: imgui.CursorPosX(), Y: ty})
	lc.Begin(len(diff.displayLines))
	for lc.Step() {
		for i := lc.DisplayStart; i < lc.DisplayEnd; i++ {
			//			diffLineView(ctx, change, &diff.displayLines[i], diff.rightLines, fileIndex, 1)
			diffLineView(ctx, change, diff, i, fileIndex, 1)
		}
	}

	imgui.PopStyleVar()
	imgui.Columns()

	for _, c := range change.commentsChange {
		ts := time.Unix(int64(c.Time), 0)
		imgui.Text(fmt.Sprintf("%s %s", ts.Format("2006-01-02 15:04:05"), c.User))
		imgui.Text(c.Body)
	}

	return nil
}

func prefetchDiffs(ctx *gigantickContext, cl int) {
	ch, ok := ctx.allChanges[cl]
	if !ok {
		return
	}
	var max = 32
	if len(ch.files) < max {
		max = len(ch.files)
	}
	for i := 0; i < max; i++ {
		ch.getDiffs(ctx, &ch.files[i])
	}
}
