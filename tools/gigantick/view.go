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
	"time"

	"sge-monorepo/libs/go/imguix"
	"sge-monorepo/libs/go/swarm"

	"github.com/AllenDang/giu/imgui"
	"github.com/atotto/clipboard"
	"github.com/golang/glog"
)

func swarmLayout(ctx *gigantickContext, review *swarm.Review) {
	if review.ID == 0 {
		return
	}

	if imgui.Button(fmt.Sprintf("Swarm %d", review.ID)) {
		openBrowser(swarm.BuildUrl(&ctx.swarm, fmt.Sprintf("reviews/%d", review.ID)))
	}

	imgui.Text("Reviewers:")

	ballot := review.BallotBuild()
	imgui.SameLine()
	imgui.Text(fmt.Sprintf("[%2d/%2d]", ballot.UpVoteCount, len(ballot.Entries)))
	imgui.SameLine()
	for _, particpant := range ballot.Entries {
		imgui.SameLine()
		if particpant.Vote > 0 {
			imgui.PushStyleColor(imgui.StyleColorButton, imgui.Vec4{0, 0.3, 0, 1})
		}
		if particpant.Vote > 0 {
			imgui.PopStyleColor()
		}
	}
}

func reviewLayout(ctx *gigantickContext, cl *gigantickChange) {
	if imgui.Button("Show in P4V") {
		cmd := exec.Command("p4vc", "change", fmt.Sprintf("%d", cl.changeList))
		cmd.Start()
	}
	imgui.SameLine()
	if cl.review.ID != 0 {
		if imgui.Button(fmt.Sprintf("Swarm %d", cl.review.ID)) {
			openBrowser(swarm.BuildUrl(&ctx.swarm, fmt.Sprintf("reviews/%d", cl.review.ID)))
		}
	} else {
		imgui.Button("Create Review")
	}
	f := cl.getFiles(ctx)
	if len(f) == 0 {
		imgui.Text("loading cl " + animatingDots())
	} else {
		imgui.Text(fmt.Sprintf("%d Files", len(f)))
		imgui.SameLine()
		dt := cl.getDiffTotalCount(ctx)
		imgui.Text(fmt.Sprintf("    %4d Edits     %4d Adds    %4d Deletes", dt.changeCount, dt.addCount, dt.deleteCount))

		allVers := []string{"Base Version for this review", "Current version in depot"}
		for i := 1; i < len(cl.review.Changes); i++ {
			allVers = append(allVers, fmt.Sprintf("#%d by %s - shelved in %d", i, cl.review.Author, cl.review.Changes[i]))
		}

		imgui.ColumnsV(2, "dc", true)

		leftVers := allVers
		if imgui.BeginCombo("###LeftVersion", allVers[cl.leftVersionIndex]) {
			for i := range leftVers {
				isSelected := cl.leftVersionIndex == i
				if imgui.SelectableV(allVers[i], isSelected, 0, imgui.Vec2{X: 0, Y: 0}) {
					cl.leftVersionIndex = i
				}
				if isSelected {
					imgui.SetItemDefaultFocus()
				}
			}
			imgui.EndCombo()
		}
		imgui.NextColumn()

		rightVers := allVers
		if imgui.BeginCombo("###RightVersion", rightVers[cl.rightVersionIndex]) {
			for i := range rightVers {
				isSelected := cl.leftVersionIndex == i
				if imgui.SelectableV(rightVers[i], isSelected, 0, imgui.Vec2{X: 0, Y: 0}) {
					cl.rightVersionIndex = i
				}
				if isSelected {
					imgui.SetItemDefaultFocus()
				}
			}
			imgui.EndCombo()
		}

		imgui.Columns()

		for i := range f {
			if imgui.TreeNode(fmt.Sprintf("[%s] %s", cl.files[i].fstat.Action, cl.files[i].fstat.ClientFile)) {
				diffView(ctx, cl, i)
				imgui.TreePop()
			}
		}
	}
}

var checks [4]bool

func changelistLayout(ctx *gigantickContext, cl *gigantickChange) {
	ctx.focusChange = cl.changeList
	imgui.PushID(fmt.Sprintf("layout %d", cl.changeList))

	if link, err := toLink(ctx); err == nil {
		if imgui.Button("Copy") {
			clipboard.WriteAll(link)
		}
		imgui.SameLine()
		imgui.Text(link)
	}

	popupTitle := fmt.Sprintf("Edit Description %d", cl.changeList)
	imgui.PushStyleColor(imgui.StyleColorChildBg, imgui.Vec4{0.3, 0.3, 0.3, 1})

	size := imgui.WindowSize()
	if imgui.BeginChildV("", imgui.Vec2{X: size.X, Y: 100}, false, 0) {
		dlines := strings.Split(cl.change.Description, "\n")
		for i := range dlines {
			imgui.Text(dlines[i])
		}
		imgui.EndChild()
		if imgui.IsItemHovered() && imgui.IsAnyMouseDown() {
			imgui.OpenPopup(popupTitle)
		}
	} else {
		// seems odd, but asserts without this
		imgui.EndChild()
	}

	imgui.PopStyleColor()

	imgui.SetNextWindowSize(imgui.Vec2{X: 1024, Y: 292})
	if imgui.BeginPopupModal(popupTitle) {
		imgui.PushItemWidth(-1)
		imgui.InputTextMultiline("###desc", &cl.change.Description)
		imgui.PopItemWidth()
		imgui.InvisibleButton("", imgui.Vec2{X: 16, Y: 8})
		if imgui.Button("Update") {
			go func() {
				defer goRoutineUnregister(ctx, goRoutineRegister(ctx, "changeupdate", cl.change.Description))
				ctx.p4.ChangeUpdate(cl.change.Description, cl.changeList)
			}()
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

	if imgui.BeginTabBar("") {
		if imgui.BeginTabItem("Review") {
			reviewLayout(ctx, cl)
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("Tests") {
			testsLayout(ctx, cl)
			imgui.EndTabItem()
		}
		imgui.EndTabBar()
	}

	imgui.PopID()

}

func changelistTree(ctx *gigantickContext, changes []*gigantickChange) {
	for i := range changes {
		if i != 0 {
			imgui.Separator()
		}
		msg := strings.Split(changes[i].change.Description, "\n")[0]
		swarm := "     "
		if changes[i].review.ID != 0 {
			b := changes[i].review.BallotBuild()
			swarm = fmt.Sprintf("S %d/%d", b.UpVoteCount, len(b.Entries))
		}
		open := imgui.TreeNode(fmt.Sprintf("Change %d [%s] %s###penders%d", changes[i].changeList, swarm, msg, changes[i].changeList))
		if open {
			changelistLayout(ctx, changes[i])
			imgui.TreePop()
		}
	}
}

func dashboardPendingColumns() {
	imgui.ColumnsV(6, "dashboard", false)
	imgui.SetColumnOffset(1, 100)
	imgui.SetColumnOffset(2, 250)
	imgui.SetColumnOffset(3, 450)
	imgui.SetColumnOffset(4, 1050)
	imgui.SetColumnOffset(5, 1150)
}

var dashLinePendingMap = make(map[int]bool)

func dashboardPending(ctx *gigantickContext, changes []int) {

	var goColumns = []float32{100, 150, 200, 500, 100}
	imguix.SetColumns(goColumns)

	colHeader := func(title string, fun func(a, b *gigantickChange) bool) {
		imguix.SortableColumnHeader(title, changes, func(i, j int) bool {
			a, ok := ctx.allChanges[changes[i]]
			if !ok {
				return true
			}
			b, ok := ctx.allChanges[changes[j]]
			if !ok {
				return false
			}
			return fun(a, b)
		})
	}

	colHeader("Change", func(a, b *gigantickChange) bool {
		return a.changeList < b.changeList
	})
	colHeader("Author", func(a, b *gigantickChange) bool {
		return a.change.User < b.change.User
	})
	colHeader("Update", func(a, b *gigantickChange) bool {
		return a.change.DateUnix < b.change.DateUnix
	})
	colHeader("Reviewers", func(a, b *gigantickChange) bool {
		return len(a.review.BallotBuild().Entries) < len(b.review.BallotBuild().Entries)
	})
	colHeader("Files", func(a, b *gigantickChange) bool {
		return len(a.files) < len(b.files)
	})
	colHeader("Description", func(a, b *gigantickChange) bool {
		return a.change.Description < b.change.Description
	})

	imgui.Columns()
	imgui.Separator()

	imguix.SetColumns(goColumns)

	selected := -1
	for i, changeNum := range changes {
		ch, ok := ctx.allChanges[changeNum]
		if !ok {
			glog.Warningf("change not found %d", changeNum)
			continue
		}
		if i != 0 {
			imgui.Separator()
		}
		desc := ""
		lines := strings.Split(ch.change.Description, "\n")
		if len(lines) > 0 {
			desc = lines[0]
		}
		clNum := ch.changeList
		if imgui.SelectableV(fmt.Sprintf("%d", clNum), selected == i, imgui.SelectableFlagsSpanAllColumns, imgui.Vec2{X: 0, Y: 0}) {
			selected = i
			dashLinePendingMap[i] = !dashLinePendingMap[i]
			v, _ := dashLinePendingMap[clNum]
			v = !v
			if dashLinePendingMap[clNum] != v {
				changeTabAdd(ctx, clNum)
			}
			dashLinePendingMap[clNum] = v
		}

		imgui.NextColumn()

		imgui.Selectable("")
		imgui.NextColumn()

		if len(ch.change.Date) > 4 {
			imgui.Selectable(fmt.Sprintf("%s", ch.change.Date)[5:])
		}
		imgui.NextColumn()

		b := ch.review.BallotBuild()
		for _, bp := range b.Entries {
			if bp.Vote > 0 {
				imgui.PushStyleColor(imgui.StyleColorText, imgui.Vec4{0, 0.7, 0, 1})
			}
			imgui.Selectable(bp.User)
			if bp.Vote > 0 {
				imgui.PopStyleColor()
			}
			imgui.SameLine()
		}
		imgui.NextColumn()

		fileCount := len(ch.files)
		if 0 == fileCount {
			fileCount = len(ch.description.Files)
		}
		imgui.Selectable(fmt.Sprintf("%d", fileCount))
		imgui.NextColumn()

		imgui.Selectable(desc)
		imgui.NextColumn()
	}

	imgui.Columns()

}

func dashboardColumns() {
	imgui.ColumnsV(6, "dashboard", false)
	imgui.SetColumnOffset(1, 100)
	imgui.SetColumnOffset(2, 250)
	imgui.SetColumnOffset(3, 450)
	imgui.SetColumnOffset(4, 1050)
	imgui.SetColumnOffset(5, 1150)
}

var dashLineSelected [128]bool
var dashLineSelectedMap = make(map[int]bool)

func dashboard(ctx *gigantickContext) {
	dashboardColumns()
	imgui.Text("Change")
	imgui.NextColumn()
	imgui.Text("Author")
	imgui.NextColumn()
	imgui.Text("Update")
	imgui.NextColumn()
	imgui.Text("Reviewers")
	imgui.NextColumn()
	imgui.Text("Files")
	imgui.NextColumn()
	imgui.Text("Description")

	imgui.Columns()
	imgui.Separator()

	dashboardColumns()

	selected := -1
	for i, rid := range ctx.dashboardReviews {
		r, ok := ctx.allReviews[rid]
		if !ok {
			glog.Warningf("can't find review: %d", rid)
			continue
		}
		if i != 0 {
			imgui.Separator()
		}
		desc := ""
		lines := strings.Split(r.Description, "\n")
		if len(lines) > 0 {
			desc = lines[0]
		}
		author := ""
		as := strings.Split(r.Author, " ")
		if len(as) > 0 {
			author = as[0]
		}

		clNum := 0
		if len(r.Changes) > 0 {
			clNum = r.Changes[len(r.Changes)-1]
		}
		if imgui.SelectableV(fmt.Sprintf("%d", clNum), selected == i, imgui.SelectableFlagsSpanAllColumns, imgui.Vec2{X: 0, Y: 0}) {
			selected = i
			dashLineSelected[i] = !dashLineSelected[i]
			v, _ := dashLineSelectedMap[clNum]
			v = !v
			dashLineSelectedMap[clNum] = v
		}

		imgui.NextColumn()

		imgui.Selectable(author)
		imgui.NextColumn()

		update := time.Unix(int64(r.Updated), 0)
		imgui.Selectable(fmt.Sprintf("%s", update.Format("01/02 15:04")))
		imgui.NextColumn()

		b := r.BallotBuild()
		for _, bp := range b.Entries {
			if bp.Vote > 0 {
				imgui.PushStyleColor(imgui.StyleColorText, imgui.Vec4{0, 0.7, 0, 1})
			}
			imgui.Selectable(bp.User)
			if bp.Vote > 0 {
				imgui.PopStyleColor()
			}
			imgui.SameLine()
		}
		imgui.NextColumn()

		fileCount := 0
		if ch, ok := ctx.allChanges[clNum]; ok && clNum > 0 {
			fileCount = len(ch.files)
		}
		imgui.Selectable(fmt.Sprintf("%d", fileCount))
		imgui.NextColumn()

		imgui.Selectable(desc)
		imgui.NextColumn()

		if dashLineSelectedMap[clNum] {
			imgui.Columns()
			if imgui.TreeNodeV("###subnode", imgui.TreeNodeFlagsDefaultOpen) {
				if cl, ok := ctx.allChanges[clNum]; ok {
					changelistLayout(ctx, cl)
				}
				imgui.TreePop()
			}
			dashboardColumns()
		}
	}
	imgui.Columns()
}

func jobsTab(ctx *gigantickContext) error {
	var goColumns = []float32{200, 200, 200, 400}
	imguix.SetColumns(goColumns)

	ctx.goRoutMutex.Lock()
	imguix.SortableColumnHeader("Start Time", ctx.goRoutDetails, func(i, j int) bool {
		return ctx.goRoutDetails[i].startTime.Unix() < ctx.goRoutDetails[j].startTime.Unix()
	})
	imguix.SortableColumnHeader("Duration", ctx.goRoutDetails, func(i, j int) bool {
		return ctx.goRoutDetails[i].dur < ctx.goRoutDetails[j].dur
	})
	imguix.SortableColumnHeader("Completed", ctx.goRoutDetails, func(i, j int) bool {
		a := 0
		b := 0
		if ctx.goRoutDetails[i].completed {
			a = 1
		}
		if ctx.goRoutDetails[j].completed {
			b = 1
		}
		return a < b
	})
	imguix.SortableColumnHeader("Function", ctx.goRoutDetails, func(i, j int) bool {
		return ctx.goRoutDetails[i].name < ctx.goRoutDetails[j].name
	})
	imguix.SortableColumnHeader("Args", ctx.goRoutDetails, func(i, j int) bool {
		return ctx.goRoutDetails[i].args < ctx.goRoutDetails[j].args
	})
	ctx.goRoutMutex.Unlock()

	imgui.Columns()
	imgui.Separator()

	if imgui.BeginChild("jobs") {
		imguix.SetColumns(goColumns)
		for i := range ctx.goRoutDetails {
			imgui.Text(ctx.goRoutDetails[i].startTime.Format("15:04:05.9999"))
			imgui.NextColumn()

			imgui.Text(fmt.Sprintf("%v", ctx.goRoutDetails[i].dur))
			imgui.NextColumn()

			imgui.Text(fmt.Sprintf("%v", ctx.goRoutDetails[i].completed))
			imgui.NextColumn()

			imgui.Text(ctx.goRoutDetails[i].name)
			imgui.NextColumn()

			imgui.Text(ctx.goRoutDetails[i].args)
			imgui.NextColumn()

		}
		imgui.EndChild()
		imgui.Columns()
	}
	return nil
}

func infoTab(ctx *gigantickContext) error {
	imguix.TextCentre(fmt.Sprintf("Gigantick v%s", gigantickVersion))
	imguix.TextCentre("(c) Google 2020")
	imgui.Separator()
	imguix.TextCentre("Perforce Details")
	imguix.TextCentre(fmt.Sprintf("User: %s", ctx.info.User))
	imguix.TextCentre(fmt.Sprintf("Client: %s", ctx.info.Client))
	imguix.TextCentre(fmt.Sprintf("Host: %s", ctx.info.Host))
	imguix.TextCentre(fmt.Sprintf("Root: %s", ctx.info.Root))
	return nil
}
