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

	"sge-monorepo/libs/go/imguix"

	"github.com/AllenDang/giu/imgui"
)

func testsLayout(ctx *gigantickContext, cl *gigantickChange) {
	if cl.sgepState != opStateProcessing {
		if imgui.Button("Run Local Presubmit") {
			cl.buildSgep(ctx)
		}
	} else {
		imgui.Text(fmt.Sprintf("running %d local presubmit test suites...", len(cl.checks)))
	}

	imgui.PushStyleColor(imgui.StyleColorChildBg, imgui.Vec4{0, 0, 0, 1})
	if imgui.BeginChild("sgep") {
		tc := 0
		fc := 0
		for _, r := range cl.results {
			for _, s := range r.SubResults {
				tc++
				if !s.Success {
					fc++
				}
			}
		}
		msg := ""
		if cl.sgepState == opStateProcessing {
			msg = "RUNNING"
		} else if cl.sgepState == opStateDone {
			if fc == 0 {
				msg = "SUCCESS"
			} else {
				msg = "FAILURE"
			}
		}

		imgui.Text(fmt.Sprintf("%s: %d tests, %d failures", msg, tc, fc))
		imgui.SameLine()
		if msg == "FAILURE" {
			if imgui.Button("FIX ALL") {
				imgui.OpenPopup("Fix All Issues")
			}
		} else {
			imgui.InvisibleButton("", imgui.Vec2{2, 2})
		}
		imgui.SetNextWindowSize(imgui.Vec2{X: 1200, Y: 220})
		if imgui.BeginPopupModal("Fix All Issues") {
			for _, r := range cl.results {
				for _, s := range r.SubResults {
					if len(s.Fix) > 0 {
						imgui.Text(s.Fix)
					}
				}
			}
			if imgui.Button("OK") {
				imgui.CloseCurrentPopup()
			}
			imgui.EndPopup()
		}

		testsColumnWidths := []float32{8, 100, 100, 600}

		pop := false
		for i := range cl.results {
			imgui.Text(cl.results[i].OverallResult.Name)
			imguix.SetColumns(testsColumnWidths)
			for j, s := range cl.results[i].SubResults {
				id := fmt.Sprintf("%p", &cl.results[i].SubResults[j])
				imgui.PushID(id)

				imgui.Dummy(imgui.Vec2{2, 26})
				imgui.NextColumn()
				if s.Success {
					imgui.PushStyleColor(imgui.StyleColorText, imgui.Vec4{0, 1, 0, 1})
					imgui.Text("PASSED")
					imgui.PopStyleColor()
					imgui.NextColumn()
				} else {
					imgui.PushStyleColor(imgui.StyleColorText, imgui.Vec4{1, 0, 0, 1})
					imgui.Text("FAILED")
					imgui.PopStyleColor()
					imgui.NextColumn()
					if imgui.Button("FIX") {
						cl.fixer = cl.results[i].SubResults[j]
						pop = true
					}
				}
				imgui.NextColumn()

				imgui.Text(s.Name)
				imgui.NextColumn()

				if len(s.Fix) > 0 {
					if imgui.TreeNodeV("view output", imgui.TreeNodeFlagsCollapsingHeader) {
						imgui.PushFont(gGui.consolasSmall)
						for i := range s.Logs {
							imgui.Text(string(s.Logs[i].Contents))
						}
						imgui.PopFont()
					}
				}
				imgui.NextColumn()
				imgui.PopID()
			}
			imgui.Columns()
		}

		//		imgui.Text(cl.sgepOutput)

		if pop {
			imgui.OpenPopup("fixer")
			cl.fixState = opStateProcessing
			go fetchFix(ctx, cl, cl.fixer.Fix)
		}
		imgui.SetNextWindowSize(imgui.Vec2{X: 1200, Y: 200})
		if imgui.BeginPopupModal("fixer") {
			imgui.Text("Applying fix:")
			imgui.Text(fmt.Sprintf("%s : %s", cl.fixer.Name, cl.fixer.Fix))

			if cl.fixState == opStateDone {
				if cl.fixResult == nil {
					imgui.Text("Done")
				} else {
					imgui.Text(fmt.Sprintf("Error Applying Fix: %v", cl.fixResult))
				}
				if imgui.Button("OK") {
					imgui.CloseCurrentPopup()
				}
			}
			imgui.EndPopup()
		}

		imgui.EndChild()
	}
	imgui.PopStyleColor()
}
