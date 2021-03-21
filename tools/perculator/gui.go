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
	"net/http"
	"os"
	"sort"

	"sge-monorepo/libs/go/files"

	"github.com/AllenDang/giu"
	"github.com/AllenDang/giu/imgui"
	"github.com/atotto/clipboard"
	"github.com/golang/glog"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

func imguiFrameBegin(title string) bool {
	return imgui.BeginV(title, nil,
		imgui.WindowFlagsNoTitleBar|
			imgui.WindowFlagsNoCollapse|
			imgui.WindowFlagsNoScrollbar|
			imgui.WindowFlagsNoMove|
			imgui.WindowFlagsNoResize,
	)
}

func cursorTextRight(titles ...string) {
	var width float32
	for _, t := range titles {
		width += imgui.CalcTextSize(t, false, 0).X
		width += imgui.CurrentStyle().FramePadding().X * 2
	}

	cp := imgui.CursorPos()
	ws := imgui.WindowSize()
	imgui.SetCursorPos(imgui.Vec2{X: ws.X - width, Y: cp.Y})
}

func periodButtons(ctx *perculatorContext) {
	if imgui.RadioButton("Day", ctx.period == periodDay) {
		ctx.period = periodDay
	}
	imgui.SameLine()
	if imgui.RadioButton("Week", ctx.period == periodWeek) {
		ctx.period = periodWeek
	}
	imgui.SameLine()
	if imgui.RadioButton("Month", ctx.period == periodMonth) {
		ctx.period = periodMonth
	}
	imgui.SameLine()
	if imgui.RadioButton("Quarter", ctx.period == periodQuarter) {
		ctx.period = periodQuarter
	}
	imgui.SameLine()
	if imgui.RadioButton("Year", ctx.period == periodYear) {
		ctx.period = periodYear
	}
	imgui.SameLine()
	if imgui.RadioButton("Life", ctx.period == periodLifetime) {
		ctx.period = periodLifetime
	}
}

func urlBar(ctx *perculatorContext) {
	imgui.Separator()
	link, err := toLink(ctx)
	if err != nil {
		return
	}
	if imgui.Button("Copy") {
		clipboard.WriteAll(link)
	}
	imgui.SameLine()
	imgui.Text(link)
	imgui.Separator()
}

func statsTab2(ctx *perculatorContext, statIndex int, _ int) error {
	return statsTab(ctx, statIndex)
}

func statsTab(ctx *perculatorContext, statIndex int) error {
	periodButtons(ctx)
	urlBar(ctx)

	max := int64(0)
	umax := 0

	indices := make([]int, len(ctx.users))
	for i := range indices {
		indices[i] = i
		if ctx.users[indices[i]].accs[statIndex].counter[ctx.period] > max {
			max = ctx.users[indices[i]].accs[statIndex].counter[ctx.period]
		}
	}
	// ascertain length of longest user name
	for i := range ctx.users {
		lu := len(ctx.users[i].userName)
		if lu > umax {
			umax = lu
		}
	}

	sort.Slice(indices, func(i, j int) bool {
		a := ctx.users[indices[i]].accs[statIndex].counter[ctx.period]
		b := ctx.users[indices[j]].accs[statIndex].counter[ctx.period]
		if a != b {
			return a > b
		}
		// Alpha sort elements where values are the same
		return ctx.users[indices[i]].userName < ctx.users[indices[j]].userName
	})

	p := message.NewPrinter(language.English)

	// create a formatter to optimally pack user name and stat
	pm := p.Sprintf("%d", max)
	formatter := fmt.Sprintf("%%%ds %%%dd", umax+2, len(pm))

	for _, u := range indices {
		imgui.Text(p.Sprintf(formatter, ctx.users[u].userName, ctx.users[u].accs[statIndex].counter[ctx.period]))
		imgui.SameLine()
		ic := imgui.CursorScreenPos()
		if ctx.users[u].accs[statIndex].counter[ctx.period] > 0 {
			perc := float32(ctx.users[u].accs[statIndex].counter[ctx.period]) / float32(max)
			barWidth := float32(256) * perc
			bar0 := imgui.Vec2{X: ic.X + 8, Y: ic.Y + 4}
			bar1 := imgui.Vec2{X: ic.X + 8 + barWidth, Y: ic.Y + 20}
			imgui.GetWindowDrawList().AddRectFilled(bar0, bar1, imgui.Vec4{0.3, 0.4, 0.3, 1}, 0, 0)
		}
		imgui.InvisibleButton("", imgui.Vec2{1, 1})
	}
	return nil
}

func ratioTab(ctx *perculatorContext, numIndex int, denomIndex int) error {
	type ratioUser struct {
		ratio    float32
		userName string
	}

	periodButtons(ctx)
	urlBar(ctx)

	var max float32
	ratios := make([]ratioUser, len(ctx.users))

	for i, u := range ctx.users {
		ratios[i].userName = u.userName
		denom := u.accs[denomIndex].counter[ctx.period]
		if 0 != denom {
			ratios[i].ratio = float32(u.accs[numIndex].counter[ctx.period]) / float32(denom)
		}
		if ratios[i].ratio > 1 {
			ratios[i].ratio = 1
		}
		if ratios[i].ratio > max {
			max = ratios[i].ratio
		}
	}

	sort.Slice(ratios, func(i, j int) bool {
		if ratios[i].ratio != ratios[j].ratio {
			return ratios[i].ratio > ratios[j].ratio
		}
		return ratios[i].userName < ratios[j].userName

	})

	for _, r := range ratios {
		imgui.Text(fmt.Sprintf("%16s %6.2f%%", r.userName, r.ratio*100))
		imgui.SameLine()
		ic := imgui.CursorScreenPos()
		if r.ratio > 0 {
			perc := float32(r.ratio) / float32(max)
			barWidth := float32(256) * perc
			bar0 := imgui.Vec2{X: ic.X + 8, Y: ic.Y + 4}
			bar1 := imgui.Vec2{X: ic.X + 8 + barWidth, Y: ic.Y + 20}
			imgui.GetWindowDrawList().AddRectFilled(bar0, bar1, imgui.Vec4{0.3, 0.4, 0.3, 1}, 0, 0)
		}
		imgui.InvisibleButton("", imgui.Vec2{1, 1})
	}
	return nil
}

func dayTab2(ctx *perculatorContext, dayStatIndex int, _ int) error {
	return dayTab(ctx, dayStatIndex)
}

func dayTab(ctx *perculatorContext, dayStatIndex int) error {

	var lines []float32

	var min int64
	var max int64
	if len(ctx.days) > 0 {
		min = ctx.days[0].counts[dayStatIndex]
		max = ctx.days[0].counts[dayStatIndex]
	}
	var total int64
	var meds []int64
	for _, d := range ctx.days {
		lines = append(lines, float32(d.counts[dayStatIndex]))
		if d.counts[dayStatIndex] > max {
			max = d.counts[dayStatIndex]
		}
		if d.counts[dayStatIndex] < min {
			min = d.counts[dayStatIndex]
		}
		total += d.counts[dayStatIndex]
		meds = append(meds, d.counts[dayStatIndex])
	}

	var median int64
	var average int64
	sort.Slice(meds, func(i, j int) bool {
		return meds[i] > meds[j]
	})
	if len(meds) > 0 {
		median = meds[len(meds)/2]
		average = total / int64(len(meds))
	}

	if len(lines) > 0 {
		imgui.PlotLines("##days", lines)
	}

	p := message.NewPrinter(language.English)
	pm := p.Sprintf("%d", max)

	formatter := fmt.Sprintf("%%8s %%%dd", len(pm))
	imgui.Text(p.Sprintf(formatter, "min", min))
	imgui.Text(p.Sprintf(formatter, "max", max))
	imgui.Text(p.Sprintf(formatter, "average", average))
	imgui.Text(p.Sprintf(formatter, "median", median))

	// Sort days chronologically
	sort.Slice(ctx.days, func(i, j int) bool {
		return ctx.days[i].date.After(ctx.days[j].date)
	})

	formatter = fmt.Sprintf("%%14v   %%%dd", len(pm))
	for _, d := range ctx.days {
		df := d.date.Format("2006-01-02 Mon")
		imgui.Text(p.Sprintf(formatter, df, d.counts[dayStatIndex]))
	}

	return nil
}

type tabFunc func(ctx *perculatorContext, stat0 int, stat1 int) error

type tabDetails struct {
	name  string
	desc  string
	fun   tabFunc
	stat0 int
	stat1 int
}

func mainWindow(ctx *perculatorContext) error {
	size := giu.Context.GetPlatform().DisplaySize()
	imgui.SetNextWindowPos(imgui.Vec2{X: 0, Y: 0})
	imgui.SetNextWindowSize(imgui.Vec2{X: size[0], Y: size[1] - 64})

	if !imguiFrameBegin("main_frame") {
		return fmt.Errorf("couldn't create main window")
	}
	ws := imgui.WindowSize()

	tabz := []tabDetails{
		tabDetails{name: "Submits", desc: "Number of CLs submitted", fun: statsTab2, stat0: statClCount},
		tabDetails{name: "Files", desc: "Number of files submitted", fun: statsTab2, stat0: statFileCount},
		tabDetails{name: "Edits", desc: "Number of files edited", fun: statsTab2, stat0: statFileEditCount},
		tabDetails{name: "Sizes", desc: "Size of files submitted", fun: statsTab2, stat0: statFileSize},
		tabDetails{name: "Reviews", desc: "Number of reviews created", fun: statsTab2, stat0: statReviewsAuthored},
		tabDetails{name: "Upvotes", desc: "Number of upvotes given", fun: statsTab2, stat0: statReviewsUpvotesGiven},
		tabDetails{name: "Downvotes", desc: "Number of downvotes given", fun: statsTab2, stat0: statReviewsDownvotesGiven},
		tabDetails{name: "No Votes", desc: "Reviews where users was requested reviewer but didn't cast a vote", fun: statsTab2, stat0: statReviewsNovotesGiven},
		tabDetails{name: "Comments", desc: "Number of comments written by user", fun: statsTab2, stat0: statCommentsGiven},
		tabDetails{name: "Voted %", desc: "Percentage of requested reviews user actually voted on", fun: ratioTab, stat0: statReviewsVotesGiven, stat1: statReviewsParticipant},
		tabDetails{name: "Review %", desc: "Percentage of user's CLs that a review was created for", fun: ratioTab, stat0: statReviewsAuthored, stat1: statClCount},
		tabDetails{name: "CL/day", desc: "CLs submmited per day", fun: dayTab2, stat0: dayClCount},
		tabDetails{name: "Size/day", desc: "Size of data submmited per day", fun: dayTab2, stat0: daySize},
		tabDetails{name: "Files/day", desc: "Number of files submmited per day", fun: dayTab2, stat0: dayFileCount},
		tabDetails{name: "DAUs", desc: "Daily active users", fun: dayTab2, stat0: dayUserCount},
	}

	if imgui.BeginTabBar("tabs") {
		for i, t := range tabz {
			tflags := 0
			if t.name == ctx.tabSelectName {
				tflags = imgui.TabItemFlagsSetSelected
				ctx.tabSelectName = ""
			}
			if imgui.BeginTabItemV(t.name, nil, tflags) {
				ctx.tabName = tabz[i].name
				if imgui.BeginChild(t.name) {
					imgui.Text(t.desc)
					t.fun(ctx, t.stat0, t.stat1)
					imgui.EndChild()
				}
				imgui.EndTabItem()
			}
		}
		imgui.EndTabBar()
	}
	imgui.End()

	imgui.SetNextWindowPos(imgui.Vec2{X: 0, Y: ws.Y})
	imgui.SetNextWindowSize(imgui.Vec2{X: ws.X, Y: 64})
	if imguiFrameBegin("bottom") {
		cursorTextRight("EXIT")
		if imgui.Button("EXIT") {
			os.Exit(1)
		}
		imgui.End()
	}
	return nil
}

func loop() {
	gContext.update()
	mainWindow(&gContext)
}

func guiMain() {
	wnd := giu.NewMasterWindow("Perculator", 1024, 800, 0, nil)
	wnd.Main(loop)
}

func run() error {
	if err := gContext.init(); err != nil {
		return fmt.Errorf("couldn't init: %v", err)
	}

	http.HandleFunc("/perculator", handler)
	go http.ListenAndServe(":8080", nil)

	guiMain()
	gContext.deinit()

	return nil
}

func main() {
	// glog to both stderr and to file
	flag.Set("alsologtostderr", "true")
	if ad, err := files.GetAppDir("sge", appName); err == nil {
		// set directory for glog to %APPDATA%/sge/perculator
		flag.Set("log_dir", ad)
	}
	flag.Parse()
	glog.Infof("application start: %s v:%s", appName, perculatorVersion)
	glog.Infof("%v", os.Args)

	err := run()
	if err != nil {
		glog.Error(err)
	}

	glog.Info("application exit")
	glog.Flush()

	if err != nil {
		os.Exit(1)
	}
}
