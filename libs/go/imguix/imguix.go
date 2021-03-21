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

// Library imguix provides extra pieces of shared functionality for imgui users
package imguix

import (
	"reflect"
	"sort"

	"github.com/AllenDang/giu/imgui"
)

// SetCursorTextCentre moves cursor to centre of available region based on text sizes
func SetCursorTextCentre(titles ...string) {
	var width float32
	for _, t := range titles {
		width += imgui.CalcTextSize(t, false, 0).X
		width += imgui.CurrentStyle().FramePadding().X * 2
	}

	cp := imgui.CursorPos()
	ws := imgui.ContentRegionAvail()
	imgui.SetCursorPos(imgui.Vec2{X: (ws.X / 2) - (width / 2), Y: cp.Y})
}

// SetCursorTextRight moves cursor to right of available region based on text sizes
func SetCursorTextRight(titles ...string) {
	var width float32
	for _, t := range titles {
		width += imgui.CalcTextSize(t, false, 0).X
		width += imgui.CurrentStyle().FramePadding().X * 2
	}

	cp := imgui.CursorPos()
	ws := imgui.ContentRegionAvail()
	imgui.SetCursorPos(imgui.Vec2{X: ws.X - width, Y: cp.Y})
}

// TextCentre renders text in the centre of the available region
func TextCentre(title string) {
	SetCursorTextCentre(title)
	imgui.Text(title)
}

// SetColumns sets a series of colums specified by the widths slice
func SetColumns(widths []float32) {
	imgui.ColumnsV(len(widths)+1, "", false)
	var off float32
	for i := range widths {
		off += widths[i]
		imgui.SetColumnOffset(i+1, off)
	}
}

// SortToggle will invert the sorting each time it is called
// it checks current sort by looking at first and last elements in slice
// depend on ordering will invoke a less or greater sort
func SortToggle(s interface{}, less func(i, j int) bool) {
	rv := reflect.ValueOf(s)
	if rv.Len() == 0 {
		return
	}
	if less(0, rv.Len()-1) {
		sort.Slice(s, func(i, j int) bool {
			return less(j, i)
		})
	} else {
		sort.Slice(s, func(i, j int) bool {
			return less(i, j)
		})
	}
}

// SortableColumn creates a displays a column header than will sort contents on click
func SortableColumnHeader(title string, s interface{}, less func(i, j int) bool) {
	if imgui.Selectable(title) {
		SortToggle(s, less)
	}
	imgui.NextColumn()
}
