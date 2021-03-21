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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/golang/glog"
)

func handler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		fmt.Fprintf(w, "gigantick post")
		var uri string
		if err := json.NewDecoder(r.Body).Decode(&uri); err != nil {
			glog.Errorf("error decoding posted json: %v", err)
		} else {
			// send this in a go func to effectively have unbounded channel capacity
			go func(uri string) {
				gContext.uriChan <- uri
			}(uri)
		}
	}
}

func toLink(ctx *gigantickContext) (string, error) {
	base := "sge://gigantick"
	u, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("couldn't parse url: %s: %v", base, err)
	}
	args := url.Values{}
	if ctx.focusChange != 0 {
		args.Add("c", fmt.Sprintf("%d", ctx.focusChange))
	}
	u.RawQuery = args.Encode()
	return u.String(), nil
}

func fromLink(ctx *gigantickContext, link string) error {
	url, err := url.Parse(link)
	if err != nil {
		return fmt.Errorf("couldn't parse url: %s: %v", link, err)
	}
	args := url.Query()
	if c, ok := args["c"]; ok && len(c) > 0 {
		if v, err := strconv.Atoi(c[0]); err == nil {
			ctx.focusChangeNew = v
			changeTabAdd(ctx, v)
		}
	}
	return nil
}

func serverInit(ctx *gigantickContext) error {
	http.HandleFunc("/gigantick", handler)
	go http.ListenAndServe(":8080", nil)

	return nil
}
