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
	"math"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"sge-monorepo/tools/ebert/ebert"
)

var (
	ErrRouteNotFound = errors.New("route not found")
)

type route struct {
	matcher     *regexp.Regexp
	specificity float64
	handler     Handler
}

// Mux routes http requests to an appropriate handler.  Handlers are functions
// with the signature:
//   func(*ebert.Context, *http.Request, *Args) (interface{}, error)
// where *Args is a pointer to a structure of strings, ints, or bools defining
// the arguments the function expects.  These arguments are extracted from
// the request path or URL params.
//
// Handler's are ordered from most to least specific, and the first handler
// that matches the request is selected.  Specificity is defined such that
// paths with more slashes are more specific and paths with more parameters
// are less specific, with parameters earlier in the path being less specific
// than parameters later in the path.  For example, the following are ordered
// from most specific to least specific.
//   /a/b/c/d
//   /a/b/c/:d
//   /:a/b/c/d
//   /a/b/c
//   /a/:b/:c
//   /:a/b/c
//   /a/b
type Mux struct {
	Prefix string
	routes []route
}

// Serve routes a http.Request to the correct implementation.
func (m *Mux) Serve(ctx *ebert.Context, r *http.Request) (interface{}, error) {
	for _, route := range m.routes {
		if route.matcher.MatchString(r.URL.Path) {
			return route.handler.Serve(ctx, r)
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrRouteNotFound, r.URL.Path)
}

// Add a handler to the mux.  The handler must be a function with the
// signature func (*ebert.Context, *http.Request, *Args) (interface{},
// error) where *Args must be a pointer to a struct of ints, strings,
// and bools, defining the handlers arguments.
func (m *Mux) Handle(pattern string, h interface{}) error {
	handler, err := Wrap(pattern, h)
	if err != nil {
		return err
	}

	var matchers []string
	parts := strings.Split(pattern, "/")
	specificity := float64(len(parts))
	for i, part := range parts {
		if part != "" && part[0] == ':' {
			penalty := math.Pow(0.5, float64(i+1))
			specificity = specificity - penalty
			if i == len(parts)-1 {
				matchers = append(matchers, ".*")
			} else {
				matchers = append(matchers, "[^/]+")
			}
		} else {
			matchers = append(matchers, part)
		}
	}
	joined := strings.Join(matchers, "/")
	matcher, err := regexp.Compile(joined)
	if err != nil {
		return fmt.Errorf("regexp.Compile(%s) error: %w", joined, err)
	}
	idx := sort.Search(len(m.routes), func(i int) bool {
		return m.routes[i].specificity < specificity
	})
	m.routes = append(m.routes, route{})
	copy(m.routes[idx+1:], m.routes[idx:])
	m.routes[idx] = route{
		matcher:     matcher,
		specificity: specificity,
		handler:     handler,
	}
	return nil
}
