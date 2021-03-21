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

// binary Poogle is a perforce search engine that allows us to perform textual searches across our entire server
package main

import (
	"encoding/json"
    _ "embed"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/imguix"
	"sge-monorepo/libs/go/p4lib"

	"github.com/AllenDang/giu"
	"github.com/AllenDang/giu/imgui"
	"github.com/atotto/clipboard"
)

const poogleVersion = "0.0.2"

// PoogleSearchQuery contains details about a search invokation
type PoogleSearchQuery struct {
	Root      string
	Term      string
	TimeStamp int64
}

type poogleSearchResult struct {
	grepStatus     p4lib.GrepStatus
	results        []p4lib.Grep
	searchDuration time.Duration
	searchRoot     string
	searchTerm     string
}

type poogleContext struct {
	results          []p4lib.Grep
	resultIndex      int
	resultCollection []poogleSearchResult
	history          []PoogleSearchQuery

	grepStatus       p4lib.GrepStatus
	grepCompleted    chan bool
	resultsChan      chan []p4lib.Grep
	uriChan          chan string
	searchDuration   time.Duration
	googleSansSmall  imgui.Font
	googleSansBig    imgui.Font
	googleSansConfig imgui.FontConfig
	tabSelectName    string

	caseSensitivity bool
	searchRoot      string
	searchTerm      string
	searching       bool
	useSwarmView    bool // launch result view via swarm
	useLocalView    bool // launch result view via local app
	swarmPath       string
	localAppPath    string
	localAppArgs    string
	p4              p4lib.P4
}

var gContext poogleContext

func (ctx *poogleContext) init() error {
	ctx.searchRoot = "//"
	ctx.useSwarmView = true
	ctx.useLocalView = false
	ctx.localAppPath = "code"
	ctx.localAppArgs = "-g $(file_name):$(line)"
	ctx.swarmPath = "INSERT_HOST"

	ctx.uriChan = make(chan string)
	ctx.p4 = p4lib.New()
	historyLoad(ctx)

	return nil
}

func (ctx *poogleContext) deinit() error {
	historySave(ctx)
	return nil
}

func (ctx *poogleContext) update() error {
	if !ctx.searching {
		select {
		case u := <-ctx.uriChan:
			fromLink(ctx, u)
			if len(ctx.searchTerm) > 0 {
				searchBegin(ctx)
			}
		default:
		}
	}
	return nil
}

func timeFunction(startTime time.Time, duration *time.Duration) {
	*duration = time.Now().Sub(startTime)
}

// begin a p4 grep. This will run in the background on multiple threads, and indicate completion via grepCompleted channel
func searchBegin(ctx *poogleContext) {
	if len(ctx.searchTerm) == 0 {
		return
	}
	if !ctx.searching {
		ctx.grepCompleted = make(chan bool)
		ctx.resultsChan = make(chan []p4lib.Grep)
		ctx.results = nil
		ctx.resultIndex = 0
		go func() {
			defer timeFunction(time.Now(), &ctx.searchDuration)
			root := ctx.searchRoot
			if strings.HasSuffix(root, "...") {
				root = root[:len(root)-3]
			}
			if strings.HasSuffix(root, "/") || strings.HasSuffix(root, "\\") {
				root = root[:len(root)-1]
			}

			ctx.grepStatus.GrepsChan = make(chan []p4lib.Grep)
			if err := ctx.p4.GrepLarge(ctx.searchTerm, root, ctx.caseSensitivity, &ctx.grepStatus); err != nil {
				fmt.Println("search error", err)
			}
			ctx.grepCompleted <- true
		}()
		ctx.searching = true
		ctx.tabSelectName = "Search"

		ctx.history = append(ctx.history,
			PoogleSearchQuery{
				Root:      ctx.searchRoot,
				Term:      ctx.searchTerm,
				TimeStamp: time.Now().Unix(),
			})
	}
}

func toLink(ctx *poogleContext) (string, error) {
	base := "sge://poogle"
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("couldn't parse url: %s: %v", base, err)
	}
	// if the search term is empty, we don't need to construct arguments for URL
	if len(ctx.searchTerm) == 0 {
		return u.String(), nil
	}
	u.Path += "/search"
	args := url.Values{}
	args.Add("q", ctx.searchTerm)
	if len(ctx.searchRoot) > 0 {
		args.Add("r", ctx.searchRoot)
	}
	u.RawQuery = args.Encode()
	return u.String(), nil
}

func fromLink(ctx *poogleContext, link string) error {
	url, err := url.Parse(link)
	if err != nil {
		return fmt.Errorf("couldn't parse url: %s: %v", link, err)
	}
	args := url.Query()
	if s, ok := args["q"]; ok && len(s) > 0 {
		ctx.searchTerm = s[0]
	}
	if r, ok := args["r"]; ok && len(r) > 0 {
		ctx.searchRoot = r[0]
	}
	return nil
}

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

func imguiTextCentre(title string) {
	cursorTextCentre(title)
	imgui.Text(title)
}

func showFile(ctx *poogleContext, depotFile string, line int) error {
	if ctx.useSwarmView {
		openBrowser(ctx.swarmPath + "/files/" + depotFile[2:])
	}
	if ctx.useLocalView {
		localFile, err := ctx.p4.Where(depotFile)
		fmt.Println("localFile", localFile)
		fmt.Println("e", err)
		if err != nil {
			return err
		}
		args := strings.Split(ctx.localAppArgs, " ")
		for i, a := range args {
			ar := strings.ReplaceAll(a, "$(file_name)", localFile)
			args[i] = strings.ReplaceAll(ar, "$(line)", fmt.Sprintf("%d", line))
		}
		cmd := exec.Command(ctx.localAppPath, args...)
		cmd.Start()
	}
	return nil
}

func toolsTab(ctx *poogleContext) error {
	cp := imgui.CursorPos()
	x1 := cp.X + 32
	imgui.SetCursorPos(imgui.Vec2{X: x1, Y: cp.Y})
	imgui.InputText("Root Search Path", &ctx.searchRoot)
	imgui.Checkbox("Case Sensitive", &ctx.caseSensitivity)
	imgui.Checkbox("Launch Swarm View Of File", &ctx.useSwarmView)
	imgui.Checkbox("Launch Local View Of File", &ctx.useLocalView)
	cp = imgui.CursorPos()
	imgui.SetCursorPos(imgui.Vec2{X: x1, Y: cp.Y})
	imgui.InputText("App Path", &ctx.localAppPath)
	cp = imgui.CursorPos()
	imgui.SetCursorPos(imgui.Vec2{X: x1, Y: cp.Y})
	imgui.InputText("App Args", &ctx.localAppArgs)
	return nil
}

func infoTab(ctx *poogleContext) error {
	imguiTextCentre(fmt.Sprintf("Poogle v%s", poogleVersion))
	imguiTextCentre("(c) Google 2020")
	imgui.Separator()
	return nil
}

func poogleLogo(ctx *poogleContext) {
	glyphs := []struct {
		s string
		c imgui.Vec4
	}{
		{"P", imgui.Vec4{66.0 / 255.0, 133.0 / 255.0, 244.0 / 255.0, 1}},
		{"o", imgui.Vec4{234.0 / 255.0, 67.0 / 255.0, 53.0 / 255.0, 1}},
		{"o", imgui.Vec4{251.0 / 255, 188.0 / 255.0, 5.0 / 255.0, 1}},
		{"g", imgui.Vec4{66.0 / 255.0, 133.0 / 255.0, 244.0 / 255.0, 1}},
		{"l", imgui.Vec4{52.0 / 255.0, 168.0 / 255.0, 83.0 / 255.0, 1}},
		{"e", imgui.Vec4{234.0 / 255.0, 68.0 / 255.0, 54.0 / 255.0, 1}},
	}

	imgui.PushFont(ctx.googleSansBig)
	imgui.PushStyleVarVec2(imgui.StyleVarItemSpacing, imgui.Vec2{X: 0, Y: 0})

	var width float32
	for _, g := range glyphs {
		width += imgui.CalcTextSize(g.s, false, 0).X
	}

	cp := imgui.CursorPos()
	ws := imgui.ContentRegionAvail()
	imgui.SetCursorPos(imgui.Vec2{X: (ws.X / 2) - (width / 4), Y: cp.Y})

	for _, g := range glyphs {
		imgui.PushStyleColor(imgui.StyleColorText, g.c)
		imgui.Text(g.s)
		imgui.SameLine()
		imgui.PopStyleColor()
	}
	imgui.PopStyleVar()
	imgui.PopFont()
	imgui.Text("")
}

func searchTab(ctx *poogleContext) error {
	ws := imgui.WindowSize()

	imgui.PushID("search")

	poogleLogo(ctx)
	imgui.PushItemWidth(512)
	cp := imgui.CursorPos()
	imgui.SetCursorPos(imgui.Vec2{X: (ws.X / 2) - 256, Y: cp.Y})
	if !imgui.IsAnyItemActive() && !imgui.IsMouseClicked(0) {
		imgui.SetKeyboardFocusHere()
	}
	iflags := imgui.InputTextFlagsEnterReturnsTrue
	// If we have started searching, we may be using an externally sourced searc term.
	// We need to set this control to readonly to ensure contents are refreshed.
	if ctx.searching {
		iflags |= imgui.InputTextFlagsReadOnly
	}
	if imgui.InputTextV("###IT", &ctx.searchTerm, iflags, nil) {
		searchBegin(ctx)
	}
	imgui.PopItemWidth()

	ps := imgui.CalcTextSize("Poogle Search", false, 0)
	ls := imgui.CalcTextSize("I'm Feeling Lucky", false, 0)
	sw := imgui.CurrentStyle().FramePadding().X * 4

	cp = imgui.CursorPos()

	imgui.SetCursorPos(imgui.Vec2{(ws.X / 2) - ((ps.X + ls.X + sw) / 2), cp.Y})

	imgui.PopID()
	if imgui.Button("Poogle Search") {
		searchBegin(ctx)
	}
	imgui.SameLine()
	if imgui.Button("I'm Feeling Lucky") {
		openBrowser("https://www.youtube.com/watch?v=oHg5SJYRHA0")
	}

	detailsX := imgui.CursorPosX()
	detailsY := imgui.CursorPosY()
	if !ctx.searching {
		dur := fmt.Sprintf("%d milliseconds", ctx.searchDuration.Milliseconds())
		if ctx.searchDuration.Milliseconds() < 2 {
			dur = fmt.Sprintf("%d milliseconds", ctx.searchDuration.Nanoseconds())
		}
		if ctx.searchDuration.Milliseconds() > 500 {
			dur = fmt.Sprintf("%f seconds", ctx.searchDuration.Seconds())
		}
		imgui.Text(fmt.Sprintf("%d result(s) in %s", len(gContext.results), dur))
		imgui.Separator()
	}
	colx := imgui.WindowSize().X - 512
	imgui.SetCursorPos(imgui.Vec2{X: colx, Y: detailsY})

	if ctx.searching {
		select {
		case res := <-ctx.grepStatus.GrepsChan:
			ctx.results = append(ctx.results, res...)
		default:
		}

		select {
		case _ = <-ctx.grepCompleted:
			ctx.resultIndex = len(ctx.resultCollection)
			ctx.resultCollection = append(ctx.resultCollection, poogleSearchResult{
				grepStatus:     ctx.grepStatus,
				results:        ctx.results,
				searchDuration: ctx.searchDuration,
				searchRoot:     ctx.searchRoot,
				searchTerm:     ctx.searchTerm,
			})

			ctx.searching = false
		default:
			bars := "|/-\\"
			index := ((time.Now().Second() * 4) % 3)
			imgui.Text(fmt.Sprintf("%s %16d files %16d bytes", bars[index:index+1], ctx.grepStatus.FilesChecked, ctx.grepStatus.BytesChecked))
			ts := imgui.GetItemRectSize()
			ic := imgui.CursorPos()
			bar0 := imgui.Vec2{X: colx, Y: ic.Y}
			bar1 := imgui.Vec2{X: colx + ts.X, Y: ic.Y + 4}
			imgui.GetWindowDrawList().AddRectFilled(bar0, bar1, imgui.Vec4{0.2, 0.2, 0.2, 1}, 0, 0)
			var progress float64
			if ctx.grepStatus.Total.FileSize > 0 {
				progress = float64(ctx.grepStatus.BytesChecked) / float64(ctx.grepStatus.Total.FileSize)
			}
			bar1.X = bar0.X + ((bar1.X - bar0.X) * float32(progress))
			imgui.GetWindowDrawList().AddRectFilled(bar0, bar1, imgui.Vec4{0.2, 0.2, 1, 1}, 0, 0)
		}
	} else {
		imgui.Text(fmt.Sprintf("  %16d files %16d bytes", ctx.grepStatus.FilesChecked, ctx.grepStatus.BytesChecked))
		imgui.GetWindowDrawList().AddRectFilled(imgui.Vec2{}, imgui.Vec2{}, imgui.Vec4{0.2, 0.2, 1, 0}, 0, 0)
	}
	imgui.SetCursorPos(imgui.Vec2{X: detailsX, Y: detailsY + 32})

	if link, err := toLink(ctx); err == nil {
		if imgui.Button("Copy") {
			clipboard.WriteAll(link)
		}
		imgui.SameLine()
		imgui.Text(link)
	}
	imgui.Separator()

	imgui.BeginChild("results")
	var lc imgui.ListClipper
	lc.Begin(len(gContext.results))
	for lc.Step() {
		for i := lc.DisplayStart; i < lc.DisplayEnd; i++ {
			grep := gContext.results[i]
			imgui.PushID(fmt.Sprintf("grep_%d", i))
			imgui.PushStyleColor(imgui.StyleColorText, imgui.Vec4{0, 1, 0, 1})
			imgui.Text(fmt.Sprintf("%s (%d)", grep.DepotPath, grep.LineNumber))

			if imgui.IsItemHovered() {
				imin := imgui.GetItemRectMin()
				imax := imgui.GetItemRectMax()
				imgui.GetWindowDrawList().AddLine(imgui.Vec2{imin.X, imax.Y + 1}, imgui.Vec2{imax.X, imax.Y + 1}, imgui.Vec4{0, 0.5, 1, 1}, 2)
			}

			if imgui.IsItemHovered() && imgui.IsMouseClicked(0) {
				showFile(ctx, grep.DepotPath, grep.LineNumber)
			}
			imgui.PopStyleColor()
			imgui.Text(grep.Contents)
			imgui.PopID()
			imgui.Separator()
		}
	}
	imgui.EndChild()

	return nil
}

func historyPath() (string, error) {
	return files.GetAppDirFileName("sge", "poogle", "history.json")
}

func historyLoad(ctx *poogleContext) error {
	filename, err := historyPath()
	if err != nil {
		return err
	}
	return files.JsonLoad(filename, &ctx.history)
}

func historySave(ctx *poogleContext) error {
	filename, err := historyPath()
	if err != nil {
		return err
	}
	return files.JsonSave(filename, &ctx.history)
}

func historyTab(ctx *poogleContext) error {
	cursorTextRight("Clear Cache")
	if imgui.Button("Clear Cache") {
		ctx.history = nil
	}

	colWidths := []float32{200, 500}
	imguix.SetColumns(colWidths)

	imguix.SortableColumnHeader("Date", ctx.history, func(i, j int) bool {
		return ctx.history[i].TimeStamp < ctx.history[j].TimeStamp
	})
	imguix.SortableColumnHeader("Root", ctx.history, func(i, j int) bool {
		return ctx.history[i].Root < ctx.history[j].Root
	})
	imguix.SortableColumnHeader("Term", ctx.history, func(i, j int) bool {
		return ctx.history[i].Term < ctx.history[j].Term
	})

	imgui.Columns()

	imgui.Separator()
	imguix.SetColumns(colWidths)
	for _, h := range ctx.history {
		c := time.Unix(h.TimeStamp, 0)
		if imgui.SelectableV(c.Format("2006-01-02 15:04:05"), false, imgui.SelectableFlagsSpanAllColumns, imgui.Vec2{X: 0, Y: 0}) {
			ctx.searchTerm = h.Term
			ctx.searchRoot = h.Root
			searchBegin(ctx)
		}
		imgui.NextColumn()

		imgui.Text(h.Root)
		imgui.NextColumn()

		imgui.Text(h.Term)
		imgui.NextColumn()
	}
	imgui.Columns()
	return nil
}

func retrieveResultFromCollection(ctx *poogleContext) {
	ctx.results = ctx.resultCollection[ctx.resultIndex].results
	ctx.searchRoot = ctx.resultCollection[ctx.resultIndex].searchRoot
	ctx.searchTerm = ctx.resultCollection[ctx.resultIndex].searchTerm
	ctx.searchDuration = ctx.resultCollection[ctx.resultIndex].searchDuration
}

func windowBuild(ctx *poogleContext) error {
	size := giu.Context.GetPlatform().DisplaySize()
	imgui.SetNextWindowPos(imgui.Vec2{X: 0, Y: 0})
	imgui.SetNextWindowSize(imgui.Vec2{X: size[0], Y: size[1] - 64})

	if !imgui.BeginV("main window", nil,
		imgui.WindowFlagsNoTitleBar|
			imgui.WindowFlagsNoCollapse|
			imgui.WindowFlagsNoScrollbar|
			imgui.WindowFlagsNoMove|
			imgui.WindowFlagsNoResize,
	) {
		return fmt.Errorf("couldn't open window")
	}

	ws := imgui.WindowSize()

	tabz := []struct {
		name string
		fun  func(ctx *poogleContext) error
	}{
		{"Search", searchTab},
		{"Tools", toolsTab},
		{"History", historyTab},
		{"Info", infoTab},
	}
	if imgui.BeginTabBar("tabs") {
		for _, t := range tabz {
			tflags := 0
			if t.name == ctx.tabSelectName {
				tflags = imgui.TabItemFlagsSetSelected
				ctx.tabSelectName = ""
			}
			if imgui.BeginTabItemV(t.name, nil, tflags) {
				t.fun(ctx)
				imgui.EndTabItem()
			}
		}
		imgui.EndTabBar()
	}
	imgui.End()

	imgui.SetNextWindowPos(imgui.Vec2{X: 0, Y: ws.Y})
	imgui.SetNextWindowSize(imgui.Vec2{X: ws.X, Y: 64})
	if imgui.BeginV("buttons2", nil,
		imgui.WindowFlagsNoTitleBar|
			imgui.WindowFlagsNoCollapse|
			imgui.WindowFlagsNoScrollbar|
			imgui.WindowFlagsNoMove|
			imgui.WindowFlagsNoResize,
	) {
		if !ctx.searching {
			if imgui.Button("<") {
				if ctx.resultIndex > 0 {
					ctx.resultIndex--
					retrieveResultFromCollection(ctx)
				}
			}
			imgui.SameLine()
			if imgui.Button(">") {
				if ctx.resultIndex+1 < len(ctx.resultCollection) {
					ctx.resultIndex++
					retrieveResultFromCollection(ctx)
				}
			}
			imgui.SameLine()
		}
		cursorTextRight("EXIT")
		if imgui.Button("EXIT") {
			os.Exit(0)
		}
		imgui.End()
	} else {
		fmt.Println("failed to open buttons panel")
	}
	return nil
}

func loop() {
	if err := gContext.update(); err != nil {
		return
	}
	windowBuild(&gContext)
}

func handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		fmt.Fprintf(w, "poogle post")
		var uri string
		if err := json.NewDecoder(r.Body).Decode(&uri); err != nil {
			fmt.Println(err)
		} else {
			// send this in a go func to effectively have unbounded channel capacity
			go func(uri string) {
				gContext.uriChan <- uri
			}(uri)
		}
	}
}

//go:embed GoogleSans-Regular.ttf
var fd []byte

func loadFonts() {
	gContext.googleSansConfig = imgui.NewFontConfig()
	fonts := imgui.CurrentIO().Fonts()
	gContext.googleSansSmall = fonts.AddFontFromMemoryTTFV(fd, 12, gContext.googleSansConfig, imgui.EmptyGlyphRanges)
	gContext.googleSansBig = fonts.AddFontFromMemoryTTFV(fd, 48, gContext.googleSansConfig, imgui.EmptyGlyphRanges)
}

func main() {
	uriArg := flag.String("uri", "", "uri argument")
	flag.Parse()

	if err := gContext.init(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fromLink(&gContext, *uriArg)
	if len(gContext.searchTerm) > 0 {
		searchBegin(&gContext)
	}

	http.HandleFunc("/poogle", handler)
	go http.ListenAndServe(":8080", nil)

	wnd := giu.NewMasterWindow("Poogle", 1920, 1080, 0, loadFonts)
	wnd.Main(loop)

	gContext.deinit()
}
