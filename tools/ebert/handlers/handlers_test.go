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

package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"testing"

	"sge-monorepo/tools/ebert/ebert"
)

func TestWrap(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		pattern string
		handler interface{}
		want    interface{}
		wantErr error
	}{
		{
			name:    "no args",
			url:     "http://test.com/a/b",
			pattern: "/a/b",
			handler: func(*ebert.Context, *http.Request, *struct{}) (interface{}, error) {
				return "function called", nil
			},
			want: "function called",
		},
		{
			name:    "string arg",
			url:     "http://test.com/a/foo",
			pattern: "/a/:b",
			handler: func(ctx *ebert.Context, r *http.Request, args *struct {
				b string
			}) (interface{}, error) {
				return args.b, nil
			},
			want: "foo",
		},
		{
			name:    "int arg",
			url:     "http://test.com/a/13",
			pattern: "/a/:b",
			handler: func(ctx *ebert.Context, r *http.Request, args *struct {
				b int
			}) (interface{}, error) {
				return args.b, nil
			},
			want: 13,
		},
		{
			name:    "invalid int arg",
			url:     "http://test.com/a/foo",
			pattern: "/a/:b",
			handler: func(ctx *ebert.Context, r *http.Request, args *struct {
				b int
			}) (interface{}, error) {
				return args.b, nil
			},
			wantErr: strconv.ErrSyntax,
		},
		{
			name:    "invalid arg type",
			url:     "http://test.com/a/foo",
			pattern: "/a/:b",
			handler: func(ctx *ebert.Context, r *http.Request, args *struct {
				b *int
			}) (interface{}, error) {
				return args.b, nil
			},
			wantErr: ErrHandlerSig,
		},
		{
			name:    "multiple args",
			url:     "http://test.com/a/foo/baz",
			pattern: "/a/:b/:c",
			handler: func(ctx *ebert.Context, r *http.Request, args *struct {
				b string
				c string
			}) (interface{}, error) {
				return args.b + args.c, nil
			},
			want: "foobaz",
		},
		{
			name:    "trailing path",
			url:     "http://test.com/a/foo/baz",
			pattern: "/a/:pattern",
			handler: func(ctx *ebert.Context, r *http.Request, args *struct {
				pattern string
			}) (interface{}, error) {
				return args.pattern, nil
			},
			want: "foo/baz",
		},
		{
			name:    "url params",
			url:     "http://test.com/a/b?a=1&b=2",
			pattern: "/a/b",
			handler: func(ctx *ebert.Context, r *http.Request, args *struct {
				a int
				b int
			}) (interface{}, error) {
				return args.a*10 + args.b, nil
			},
			want: 12,
		},
		{
			name:    "url bool param, true",
			url:     "http://test.com/a/b?a=true",
			pattern: "/a/b",
			handler: func(ctx *ebert.Context, r *http.Request, args *struct {
				a bool
			}) (interface{}, error) {
				return args.a, nil
			},
			want: true,
		},
		{
			name:    "url bool param, false",
			url:     "http://test.com/a/b?a=nottrue",
			pattern: "/a/b",
			handler: func(ctx *ebert.Context, r *http.Request, args *struct {
				a bool
			}) (interface{}, error) {
				return args.a, nil
			},
			want: false,
		},
		{
			name:    "url bool param, empty(true)",
			url:     "http://test.com/a/b?a",
			pattern: "/a/b",
			handler: func(ctx *ebert.Context, r *http.Request, args *struct {
				a bool
			}) (interface{}, error) {
				return args.a, nil
			},
			want: true,
		},
		{
			name:    "url bool param, absent(false)",
			url:     "http://test.com/a/b",
			pattern: "/a/b",
			handler: func(ctx *ebert.Context, r *http.Request, args *struct {
				a bool
			}) (interface{}, error) {
				return args.a, nil
			},
			want: false,
		},
		{
			// verify that url params override pattern params
			name:    "pattern and url params",
			url:     "http://test.com/a/3?a=1&b=2",
			pattern: "/a/:b",
			handler: func(ctx *ebert.Context, r *http.Request, args *struct {
				a int
				b int
			}) (interface{}, error) {
				return args.a*10 + args.b, nil
			},
			want: 12,
		},
	}

	for _, test := range tests {
		wrapped, err := Wrap(test.pattern, test.handler)
		if err != nil {
			t.Errorf("test '%s' couldn't wrap handler: %v", test.name, err)
			continue
		}
		r, err := http.NewRequest("GET", test.url, nil)
		if err != nil {
			t.Errorf("test '%s' couldn't create request", test.name)
			continue
		}
		got, err := wrapped.Serve(nil, r)
		if !errors.Is(err, test.wantErr) && fmt.Sprintf("%v", err) != fmt.Sprintf("%v", test.wantErr) {
			t.Errorf("test '%s' error mismatch: want %v, got %v", test.name, test.wantErr, err)
		}
		if !reflect.DeepEqual(test.want, got) {
			t.Errorf("test '%s' result mismatch: want %v, got %v", test.name, test.want, got)
		}
	}
}
