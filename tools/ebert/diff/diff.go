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

package diff

import (
	"fmt"
	"strings"
)

// Use Myers algorithm to compute diffs.
func Compute(from, to []byte) (string, error) {
	fromLines := []string{}
	toLines := []string{}
	if len(from) > 0 {
		fromLines = strings.Split(string(from), "\n")
	}
	if len(to) > 0 {
		toLines = strings.Split(string(to), "\n")
	}
	prefix := findCommonPrefix(fromLines, toLines)
	suffix := findCommonSuffix(fromLines[prefix:], toLines[prefix:])

	middle, err := myersDiff(fromLines[prefix:len(fromLines)-suffix], toLines[prefix:len(toLines)-suffix])
	if err != nil {
		return "", err
	}
	diffs := []string{}
	for i := 0; i < prefix; i++ {
		diffs = append(diffs, "="+fromLines[i])
	}
	diffs = append(diffs, middle...)
	for i := len(fromLines) - suffix; i < len(fromLines); i++ {
		diffs = append(diffs, "="+fromLines[i])
	}
	return strings.Join(diffs, "\n"), nil
}

func findCommonPrefix(from, to []string) int {
	min := len(from)
	if len(to) < min {
		min = len(to)
	}
	for i := 0; i < min; i++ {
		if from[i] != to[i] {
			return i
		}
	}
	return min
}

func findCommonSuffix(from, to []string) int {
	min := len(from)
	if len(to) < min {
		min = len(to)
		from = from[len(from)-min:]
	} else {
		to = to[len(to)-min:]
	}
	for i := min; i > 0; i-- {
		if from[i-1] != to[i-1] {
			return min - i
		}
	}
	return min
}

type diffPath struct {
	x    int
	path string
}

func myersDiff(from, to []string) ([]string, error) {
	m := len(from)
	n := len(to)
	max := m + n
	if max == 0 {
		return []string{}, nil
	}
	v := make([]diffPath, 2*max+1)

	for d := 0; d <= max; d++ {
		for k := -d; k <= d; k = k + 2 {
			var x int
			var path string
			if k == -d || (k != d && v[k+max-1].x < v[k+max+1].x) {
				x = v[k+max+1].x
				path = v[k+max+1].path + "-"
			} else {
				x = v[k+max-1].x + 1
				path = v[k+max-1].path + "+"
			}
			y := x - k
			same := strings.Builder{}
			for x < n && y < m && to[x] == from[y] {
				x++
				y++
				same.WriteByte('=')
			}
			if x >= n && y >= m {
				if len(path) >= 1 {
					path = path[1:]
				}
				return buildDiffs(from, to, path+same.String())
			}
			v[k+max].x = x
			v[k+max].path = path + same.String()
		}
	}
	return nil, fmt.Errorf("Failed to find minimal diff")
}

func buildDiffs(from, to []string, edits string) ([]string, error) {
	diffs := make([]string, 0, len(edits))
	fi := 0
	ti := 0
	for _, e := range edits {
		if e == '=' {
			if from[fi] != to[ti] {
				return nil, fmt.Errorf("Expected '%s' to match '%s'", from[fi], to[ti])
			}
			diffs = append(diffs, string(e)+from[fi])
			fi++
			ti++
		} else if e == '-' {
			diffs = append(diffs, string(e)+from[fi])
			fi++
		} else if e == '+' {
			diffs = append(diffs, string(e)+to[ti])
			ti++
		} else {
			return nil, fmt.Errorf("Unknown edit type '%s'", string(e))
		}
	}
	return diffs, nil
}
