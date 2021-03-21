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

// Package launcher launches common SG&E as a shadow copy to allow syncing of new versions of exe
// to happen from p4.
package main

import (
    _ "embed"
    "errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/libs/go/files"

	"github.com/AllenDang/giu"
	"github.com/AllenDang/giu/imgui"
	"github.com/golang/glog"
)

const appVersion = "0.0.1"
const appName = "launcher"

type launcherContext struct {
	googleSansSmall  imgui.Font
	googleSansBig    imgui.Font
	googleSansConfig imgui.FontConfig
	applications     []application
	root             string
}

type application struct {
	name     string
	fileName string
}

var gContext launcherContext

var gApps = []application{
	{name: "Gigantick", fileName: "bin/windows/gigantick.exe"},
	{name: "Poogle", fileName: "bin/windows/poogle.exe"},
	{name: "Urika", fileName: "bin/windows/urika.exe"},
}

var monorepoDebugPath = flag.String("monorepo_debug_path", "", "Path for debugging in intellij")
var appButtonWidth = 512
var appButtonHeight = 96

func (ctx *launcherContext) init() error {
	ctx.applications = gApps
	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return err
	}
	mr, err := monorepo.NewFromDir(dir)
	if err != nil {
		glog.Warningf("couldn't find monorepo at %s", dir)
        if *monorepoDebugPath == "" {
            return errors.New("no monorepo_debug_path set")
        }
		if mr, err = monorepo.NewFromDir(*monorepoDebugPath); err != nil {
			return err
		}
	}
	ctx.root = mr.Root
	return nil
}

func (ctx *launcherContext) update() error {
	return nil
}

func launch(ctx *launcherContext, app *application) error {
	file_name_src := path.Join(ctx.root, app.fileName)
	appDir, err := files.GetAppDir("sge", appName)
	if err != nil {
		return err
	}
	file_name_dst := path.Join(appDir, "apps", app.fileName)
	if err := os.MkdirAll(path.Dir(file_name_dst), os.ModeDir); err != nil {
		return err
	}
	if err := files.Copy(file_name_src, file_name_dst); err != nil {
		return err
	}
	cmd := exec.Command(file_name_dst, "")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Start()
}

func windowBuild(ctx *launcherContext) error {
	size := giu.Context.GetPlatform().DisplaySize()
	imgui.SetNextWindowPos(imgui.Vec2{X: 0, Y: 0})
	imgui.SetNextWindowSize(imgui.Vec2{X: size[0], Y: size[1]})
	if !imgui.BeginV("main window", nil,
		imgui.WindowFlagsNoTitleBar|
			imgui.WindowFlagsNoCollapse|
			imgui.WindowFlagsNoScrollbar|
			imgui.WindowFlagsNoMove|
			imgui.WindowFlagsNoResize,
	) {
		return fmt.Errorf("couldn't open window")
	}
	bv := imgui.Vec2{float32(appButtonWidth), float32(appButtonHeight)}
	imgui.PushFont(ctx.googleSansBig)
	for _, a := range ctx.applications {
		if imgui.ButtonV(a.name, bv) {
			if err := launch(ctx, &a); err != nil {
				glog.Infof("couldn't launch %s %v", a.name, err)
			}
		}
	}
	imgui.PopFont()
	imgui.End()
	return nil
}

func loop() {
	if err := gContext.update(); err != nil {
		return
	}
	windowBuild(&gContext)
}

//go:embed GoogleSans-Regular.ttf
var fd []byte

func loadFonts() {
	gContext.googleSansConfig = imgui.NewFontConfig()
	fonts := imgui.CurrentIO().Fonts()
	gContext.googleSansSmall = fonts.AddFontFromMemoryTTFV(fd, 12, gContext.googleSansConfig, imgui.EmptyGlyphRanges)
	gContext.googleSansBig = fonts.AddFontFromMemoryTTFV(fd, 48, gContext.googleSansConfig, imgui.EmptyGlyphRanges)
}

func run() error {
	if err := gContext.init(); err != nil {
		return err
	}
	wnd := giu.NewMasterWindow("SG&E Launcher", appButtonWidth+16, len(gApps)*appButtonHeight+40, 0, loadFonts)
	wnd.Main(loop)
	return nil
}

func main() {
	// glog to both stderr and to file
	flag.Set("alsologtostderr", "true")
	if ad, err := files.GetAppDir("sge", appName); err == nil {
		// set directory for glog to %APPDATA%/sge/launcher
		flag.Set("log_dir", ad)
	}
	flag.Parse()
	glog.Infof("application start: %s v:%s", appName, appVersion)
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
