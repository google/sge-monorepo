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
	"testing"

	"sge-monorepo/tools/ebert/ebert"
)

func TestMux(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		handlers map[string]interface{}
		want     interface{}
		wantErr  error
	}{
		{
			name:    "empty mux",
			url:     "http://test.com/a/b",
			wantErr: ErrRouteNotFound,
		},
		{
			name: "one handler",
			url:  "http://test.com/a/b",
			handlers: map[string]interface{}{
				"/a/b": func(*ebert.Context, *http.Request, *struct{}) (interface{}, error) {
					return 1, nil
				},
			},
			want: 1,
		},
		{
			name: "two handlers",
			url:  "http://test.com/a/b",
			handlers: map[string]interface{}{
				"/a/b": func(*ebert.Context, *http.Request, *struct{}) (interface{}, error) {
					return 1, nil
				},
				"/a": func(*ebert.Context, *http.Request, *struct{}) (interface{}, error) {
					return 2, nil
				},
			},
			want: 1,
		},
		{
			name: "three handlers with arg (don't match arg)",
			url:  "http://test.com/a/b",
			handlers: map[string]interface{}{
				"/a/b": func(*ebert.Context, *http.Request, *struct{}) (interface{}, error) {
					return 1, nil
				},
				"/a/:b": func(ctx *ebert.Context, r *http.Request, args *struct{ b string }) (interface{}, error) {
					return args.b, nil
				},
				"/a": func(*ebert.Context, *http.Request, *struct{}) (interface{}, error) {
					return 2, nil
				},
			},
			want: 1,
		},
		{
			name: "three handlers with arg (match arg)",
			url:  "http://test.com/a/foo",
			handlers: map[string]interface{}{
				"/a/b": func(*ebert.Context, *http.Request, *struct{}) (interface{}, error) {
					return 1, nil
				},
				"/a/:b": func(ctx *ebert.Context, r *http.Request, args *struct{ b string }) (interface{}, error) {
					return args.b, nil
				},
				"/a": func(*ebert.Context, *http.Request, *struct{}) (interface{}, error) {
					return 2, nil
				},
			},
			want: "foo",
		},
	}

	for _, test := range tests {
		mux := &Mux{}
		for pattern, handler := range test.handlers {
			if err := mux.Handle(pattern, handler); err != nil {
				t.Errorf("test '%s' adding handler failed: %v", test.name, err)
				continue
			}
		}
		r, err := http.NewRequest("GET", test.url, nil)
		if err != nil {
			t.Errorf("test '%s' couldn't create request: %v", test.name, err)
			continue
		}
		got, err := mux.Serve(nil, r)
		if !errors.Is(err, test.wantErr) && fmt.Sprintf("%v", err) != fmt.Sprintf("%v", test.wantErr) {
			t.Errorf("test '%s' error mismatch: want %v, got %v", test.name, test.wantErr, err)
		}
		if !reflect.DeepEqual(test.want, got) {
			t.Errorf("test '%s' result mismatch: want %v, got %v", test.name, test.want, got)
		}
	}
}
