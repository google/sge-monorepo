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
	"testing"
)

func TestCommonPrefix(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want int
	}{
		{name: "empty", a: []string{}, b: []string{}, want: 0},
		{name: "x : x", a: []string{"x"}, b: []string{"x"}, want: 1},
		{name: "x : y", a: []string{"x"}, b: []string{"y"}, want: 0},
		{name: "xx : xx", a: []string{"x", "x"}, b: []string{"x", "x"}, want: 2},
		{name: "xx : xy", a: []string{"x", "x"}, b: []string{"x", "y"}, want: 1},
		{name: "xxx : xx", a: []string{"x", "x", "x"}, b: []string{"x", "x"}, want: 2},
		{name: "xxx : xy", a: []string{"x", "x", "x"}, b: []string{"x", "y"}, want: 1},
		{name: "xx : xxx", a: []string{"x", "x"}, b: []string{"x", "x", "x"}, want: 2},
		{name: "xx : xyx", a: []string{"x", "x"}, b: []string{"x", "y", "x"}, want: 1},
	}

	for _, test := range tests {
		got := findCommonPrefix(test.a, test.b)
		if got != test.want {
			t.Errorf("unexpected prefix for %s, want %d, got %d", test.name, test.want, got)
		}
	}
}

func TestCommonSuffix(t *testing.T) {
	tests := []struct {
		name string
		a    []string
		b    []string
		want int
	}{
		{name: "empty", a: []string{}, b: []string{}, want: 0},
		{name: "x : x", a: []string{"x"}, b: []string{"x"}, want: 1},
		{name: "x : y", a: []string{"x"}, b: []string{"y"}, want: 0},
		{name: "xx : xx", a: []string{"x", "x"}, b: []string{"x", "x"}, want: 2},
		{name: "xx : yx", a: []string{"x", "x"}, b: []string{"y", "x"}, want: 1},
		{name: "xxx : xx", a: []string{"x", "x", "x"}, b: []string{"x", "x"}, want: 2},
		{name: "xxx : yx", a: []string{"x", "x", "x"}, b: []string{"y", "x"}, want: 1},
		{name: "xx : xxx", a: []string{"x", "x"}, b: []string{"x", "x", "x"}, want: 2},
		{name: "xx : yyx", a: []string{"x", "x"}, b: []string{"y", "y", "x"}, want: 1},
	}

	for _, test := range tests {
		got := findCommonSuffix(test.a, test.b)
		if got != test.want {
			t.Errorf("unexpected suffix for %s, want %d, got %d", test.name, test.want, got)
		}
	}
}

func TestDiffs(t *testing.T) {
	tests := []struct {
		from string
		to   string
		want string
	}{
		{from: "hello", to: "hello", want: "=hello"},
		{from: "hello", to: "", want: "-hello"},
		{from: "", to: "hello", want: "+hello"},
		{from: "world", to: "hello\nworld", want: "+hello\n=world"},
		{from: "hello\nworld", to: "hi\nworld", want: "+hi\n-hello\n=world"},
		{from: "goodbye\ncruel\nworld", to: "hello\nworld", want: "+hello\n-goodbye\n-cruel\n=world"},
		{from: "1\n2\n3\n4\n5", to: "1\n2", want: "=1\n=2\n-3\n-4\n-5"},
		{from: "1\n2", to: "1\n2\n3\n4\n5", want: "=1\n=2\n+3\n+4\n+5"},
		{from: "1\n2\na\nb\nc\nd\ne\n3", to: "x\ny\na\nb\n1\nd\nz", want: "+x\n+y\n-1\n-2\n=a\n=b\n+1\n-c\n=d\n+z\n-e\n-3"},
	}

	for _, test := range tests {
		fd, err := Compute([]byte(test.from), []byte(test.to))
		if err != nil {
			t.Errorf("error computing diffs: %v", err)
		}
		if fd != test.want {
			t.Errorf("unexpected diff, want '%s', got '%s'", test.want, fd)
		}
	}
}
