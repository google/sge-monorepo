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
    "embed"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"sge-monorepo/libs/go/imguix"
	"sge-monorepo/libs/go/swarm"

	"github.com/AllenDang/giu"
	"github.com/AllenDang/giu/imgui"
	"github.com/golang/glog"
)

//go:embed *.ttf
var fontData embed.FS

func readFontData(font string) ([]byte, error) {
    fd, err := fontData.ReadFile(font)
    if err != nil {
        return nil, fmt.Errorf("could not read font %q", font)
    }
    return fd, nil
}

type gigantickChangeTab struct {
	cl int
	id int
}

type gigantickGui struct {
	changeTabID      int
	changeTabs       []gigantickChangeTab
	googleSansSmall  imgui.Font
	googleSansBig    imgui.Font
	googleSansConfig imgui.FontConfig
	consolasSmall    imgui.Font
	consolasConfig   imgui.FontConfig
}

var gGui gigantickGui

func openBrowser(url string) bool {
	var args []string
	switch runtime.GOOS {
	case "darwin":
		args = []string{"open"}
	case "windows":
		args = []string{"cmd", "/c", "start"}
	default:
		args = []string{"xdg-open"}
	}
	cmd := exec.Command(args[0], append(args[1:], url)...)
	return cmd.Start() == nil
}

func showFile(ctx *gigantickContext, depotFile string, line int) error {
	if len(depotFile) < 2 {
		return fmt.Errorf("depot path too short")
	}
	openBrowser(swarm.BuildUrl(&ctx.swarm, "files/"+depotFile[2:]))
	return nil
}

func showFileLocal(ctx *gigantickContext, depotFile string, line int) error {
	localFile, err := ctx.p4.Where(depotFile)
	if err != nil {
		return err
	}
	args := strings.Split(ctx.localAppArgs, " ")
	for i, a := range args {
		ar := strings.ReplaceAll(a, "$(file_name)", localFile)
		args[i] = strings.ReplaceAll(ar, "$(line)", fmt.Sprintf("%d", line))
	}
	cmd := exec.Command(ctx.localAppPath, args...)
	return cmd.Start()
}

var animatingDotsStart = time.Now()

func animatingDots() string {
	dots := ".........."
	secs := time.Now().Sub(animatingDotsStart).Seconds()
	secs = secs - float64(int64(secs))
	index := int(secs * float64(len(dots)))
	return dots[:index]
}

func cursorTextCentre(titles ...string) {
	var width float32
	for _, t := range titles {
		width += imgui.CalcTextSize(t, false, 0).X
		width += imgui.CurrentStyle().FramePadding().X * 2
	}

	cp := imgui.CursorPos()
	ws := imgui.WindowSize()
	imgui.SetCursorPos(imgui.Vec2{X: (ws.X / 2) - (width / 2), Y: cp.Y})
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

func columnTextRight(titles ...string) {
	var width float32
	for _, t := range titles {
		width += imgui.CalcTextSize(t, false, 0).X
		width += imgui.CurrentStyle().FramePadding().X * 2
	}

	cp := imgui.CursorPos()
	cx := imgui.WindowSize().X / 4
	imgui.SetCursorPos(imgui.Vec2{X: cx - width, Y: cp.Y})
}

func timeFromString(t string) (time.Time, error) {
	v, err := strconv.ParseInt(t, 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(v, 0), nil
}

func imguiTextCentre(title string) {
	cursorTextCentre(title)
	imgui.Text(title)
}

func imguiFrameBegin(title string) bool {
	return imgui.BeginV(title, nil,
		imgui.WindowFlagsNoTitleBar|
			imgui.WindowFlagsNoCollapse|
			imgui.WindowFlagsNoScrollbar|
			imgui.WindowFlagsNoMove|
			imgui.WindowFlagsNoResize,
	)
}

func mainWindow(ctx *gigantickContext) error {
	for len(ctx.focusChanges) > 0 {
		changeTabAdd(ctx, ctx.focusChanges[0])
		ctx.focusChanges = ctx.focusChanges[1:]
	}

	size := giu.Context.GetPlatform().DisplaySize()
	imgui.SetNextWindowPos(imgui.Vec2{X: 0, Y: 0})
	imgui.SetNextWindowSize(imgui.Vec2{X: size[0], Y: size[1] - 64})

	if !imguiFrameBegin("main_frame") {
		return fmt.Errorf("couldn't create main window")
	}
	ws := imgui.WindowSize()

	if imgui.BeginTabBarV("tabs", imgui.TabBarFlagsAutoSelectNewTabs) {

		if imgui.BeginTabItem("Pending") {
			if imgui.BeginChild("Pending") {
				dashboardPending(ctx, ctx.pendingChanges)
				imgui.EndChild()
			}
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("Submitted") {
			if imgui.BeginChild("submitted") {
				dashboardPending(ctx, ctx.submittedChanges)
				imgui.EndChild()
			}
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("Dashboard") {
			if imgui.BeginChild("dashboard") {
				dashboard(ctx)
				imgui.EndChild()
			}
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("Info") {
			infoTab(ctx)
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("Log") {
			jobsTab(ctx)
			imgui.EndTabItem()
		}

		var closers []int
		for ti, ct := range gGui.changeTabs {
			if gc, ok := ctx.allChanges[ct.cl]; ok {
				rtext := fmt.Sprintf("CL %d###", ct.cl)
				if imgui.BeginTabItem(rtext) {
					if imgui.BeginPopupContextItemV("Tab Option", 1) {
						if imgui.MenuItem("Close Tab") {
							closers = append(closers, (ct.cl))
						}
						if imgui.MenuItem("Close Other Tabs") {
							for tj, cc := range gGui.changeTabs {
								if ti != tj {
									closers = append(closers, cc.cl)
								}
							}
						}
						imgui.EndPopup()
					}
					if imgui.BeginChild(rtext) {
						changelistLayout(ctx, gc)
						imgui.EndChild()
					}
					imgui.EndTabItem()
				}
			}
		}

		for _, c := range closers {
			changeTabClose(c)
		}

		imgui.EndTabBar()
	}
	imgui.End()

	imgui.SetNextWindowPos(imgui.Vec2{X: 0, Y: ws.Y})
	imgui.SetNextWindowSize(imgui.Vec2{X: ws.X, Y: 64})
	if imguiFrameBegin("bottom") {
		imguix.SetCursorTextRight("EXIT")
		if imgui.Button("EXIT") {
			os.Exit(1)
		}
		imgui.End()
	}
	return nil
}

func changeTabClose(cl int) {
	for i := range gGui.changeTabs {
		if gGui.changeTabs[i].cl == cl {
			copy(gGui.changeTabs[i:], gGui.changeTabs[i+1:])
			gGui.changeTabs = gGui.changeTabs[:len(gGui.changeTabs)-1]
			break
		}
	}
}

func loadFonts() {
	// fd, ok := FontData["GoogleSans-Regular.ttf"]
	// if !ok {
	// 	glog.Errorf("could not find font: GoogleSans-Regular.ttf")
	// 	return
	// }
    fd, err := readFontData("GoogleSans-Regular.ttf")
    if err != nil {
        glog.Error(err)
        return
    }
	gGui.googleSansConfig = imgui.NewFontConfig()
	fonts := imgui.CurrentIO().Fonts()
	gGui.googleSansSmall = fonts.AddFontFromMemoryTTFV(fd, 12, gGui.googleSansConfig, imgui.EmptyGlyphRanges)

    fm, err := readFontData("consolas-l1x.ttf")
    if err != nil {
        glog.Error(err)
        return
    }
	gGui.consolasConfig = imgui.NewFontConfig()
	gGui.consolasSmall = fonts.AddFontFromMemoryTTFV(fm, 12, gGui.consolasConfig, imgui.EmptyGlyphRanges)
}

func changeTabAdd(ctx *gigantickContext, cl int) {
	changeTabClose(cl)
	gGui.changeTabID++
	gGui.changeTabs = append(gGui.changeTabs, gigantickChangeTab{cl: cl, id: gGui.changeTabID})
	prefetchDiffs(ctx, cl)
}

func loop() {
	gContext.update()
	mainWindow(&gContext)
	if gContext.focusChangeNew != 0 {
		changeTabAdd(&gContext, gContext.focusChangeNew)
		gContext.focusChangeNew = 0
	}
}
