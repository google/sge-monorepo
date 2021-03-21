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

package ciemail

import (
	"bytes"
	"fmt"
	"sort"

	"sge-monorepo/build/cicd/presubmit"
	"sge-monorepo/libs/go/email"

	"sge-monorepo/build/cicd/presubmit/protos/presubmitpb"

	"github.com/julvo/htmlgo"
	"github.com/julvo/htmlgo/attributes"
)

// CheckResult represents a simple check result.
type CheckResult struct {
	Name   string
	Check  presubmit.Check
	Result *presubmitpb.CheckResult
}

// PresbumitEmail holds the information needed for sending a presubmit email.
type PresubmitEmailData struct {
	Author   string
	ReviewID int
	ChangeID int

	ReviewURL  string
	EbertURL   string
	ResultsURL string

	Success bool
	Results []CheckResult
}

const (
	bigPassIcon = "https://fonts.gstatic.com/s/i/googlematerialicons/check_circle_filled/v6/white-36dp/2x/gm_check_circle_filled_white_36dp.png"
	bigFailIcon = "https://fonts.gstatic.com/s/i/googlematerialicons/error/v8/white-36dp/2x/gm_error_white_36dp.png"

	smallPassIcon = "https://fonts.gstatic.com/s/i/materialiconsextended/check_circle_outline/v6/white-24dp/1x/baseline_check_circle_outline_white_24dp.png"
	smallFailIcon = "https://fonts.gstatic.com/s/i/materialiconsextended/error_outline/v6/white-24dp/1x/baseline_error_outline_white_24dp.png"
)

const (
	// We unify the subject line for all emails so that gmail will then chain them.
	presubmitSubjectFmt = "[sge-ci] Test run for Review %d"
)

func NewPresubmitEmail(data *PresubmitEmailData) *email.Email {
	to := []string{toCompanyEmail(data.Author)}
	subject := fmt.Sprintf(presubmitSubjectFmt, data.ReviewID)
	html := EmailHtml(
		emailHead(),
		emailBody(data),
	)
	htmlBuf := new(bytes.Buffer)
	htmlgo.WriteTo(htmlBuf, html)
	// Create the email.
	e := &email.Email{
		Subject:     subject,
		To:          to,
		EmailBody:   htmlBuf.String(),
		ContentType: email.ContentTypeHTML,
	}
	return e
}

func toCompanyEmail(username string) string {
	return fmt.Sprintf("%s@foo.com", username)
}

func emailHead() htmlgo.HTML {
	return htmlgo.Head_(
		htmlgo.Title_(htmlgo.Text("Presubmit Email")),
		htmlgo.Meta(htmlgo.Attr(
			attributes.HttpEquiv_("Content-Type"),
			attributes.Content_("text/html; charset=UTF-8"))),
		htmlgo.Meta(htmlgo.Attr(
			attributes.Name_("viewport"),
			attributes.Content_("width=device-width, initial-scale=1.0"))),
		htmlgo.Link(htmlgo.Attr(
			attributes.Href_("https://fonts.googleapis.com/css2?family=Roboto&display=swap"),
			attributes.Rel_("stylesheet"),
		)),
        htmlgo.Style_(htmlgo.Text(cssContent)),
	)
}

func emailBody(data *PresubmitEmailData) htmlgo.HTML {
	return htmlgo.Body(
		htmlgo.Attr(attributes.Style_("margin: 0; padding: 0; font-family:'Roboto', sans-serif;")),
		htmlgo.Table(
			TableAttr(
				attributes.Align_("center"),
				attributes.Border_("0"),
				attributes.Class_("main-table")),
			TrTd_(banner(data)),
			TrTd_(results(data)),
			TrTd_(actions(data)),
		),
	)
}

func banner(data *PresubmitEmailData) htmlgo.HTML {
	iconSrc := bigPassIcon
	class := "background-pass"
	title := fmt.Sprintf("Presubmit Success for Review %d", data.ReviewID)
	if !data.Success {
		iconSrc = bigFailIcon
		class = "background-fail"
		title = fmt.Sprintf("Presubmit Failure for Review %d", data.ReviewID)
	}

	return htmlgo.Table(
		TableAttr(attributes.Align_("center"), attributes.Class_("banner-table")),
		TrTd(htmlgo.Attr(attributes.Align_("center"), attributes.Class_("banner", "banner-icon", class)),
			htmlgo.Img(htmlgo.Attr(attributes.Src_(iconSrc))),
		),
		TrTd(
			htmlgo.Attr(attributes.Align_("center"), attributes.Class_("banner", "banner-title", class)),
			htmlgo.Text(title),
		),
	)
}

func results(data *PresubmitEmailData) htmlgo.HTML {
	msg := "CI performed a successful run on your CL."
	if !data.Success {
		msg = "CI detected presubmit errors on your CL."
	}
	return htmlgo.Table(
		TableAttr(),
		TrTd(htmlgo.Attr(attributes.Class_("results-body")),
			htmlgo.P_(htmlgo.Text(fmt.Sprintf("Hello %s,", data.Author))),
			htmlgo.P(htmlgo.Attr(attributes.Class_("no-margin")), htmlgo.Text(msg)),
			htmlgo.P(htmlgo.Attr(attributes.Class_("no-margin")), htmlgo.Text(fmt.Sprintf("Review: %d", data.ReviewID))),
			htmlgo.P(htmlgo.Attr(attributes.Class_("no-margin")), htmlgo.Text(fmt.Sprintf("Change: %d", data.ChangeID))),
		),
		TrTd_(
			htmlgo.Table(TableAttr(attributes.Class_("results-table")),
				checkResults(data)...,
			),
		),
	)
}

func checkResults(data *PresubmitEmailData) []htmlgo.HTML {
	// Sort alphabetically.
	sort.Slice(data.Results, func(i, j int) bool {
		return data.Results[i].Name < data.Results[j].Name
	})
	var fail []CheckResult
	var pass []CheckResult
	for _, r := range data.Results {
		if r.Result.OverallResult.Success {
			pass = append(pass, r)
		} else {
			fail = append(fail, r)
		}
	}
	// Output the fail checks first.
	var rows []htmlgo.HTML
	for _, check := range fail {
		rows = append(rows, checkRow(check.Name, smallFailIcon, "background-fail"))
	}
	for _, check := range pass {
		rows = append(rows, checkRow(check.Name, smallPassIcon, "background-pass"))
	}
	return rows
}

func checkRow(name, icon, backgroundClass string) htmlgo.HTML {
	return htmlgo.Tr_(
		htmlgo.Td_(
			htmlgo.Img(htmlgo.Attr(attributes.Class_(backgroundClass), attributes.Src_(icon))),
		),
		htmlgo.Td(htmlgo.Attr(attributes.Class_("check-result-name")),
			htmlgo.Text(name),
		),
	)
}

func actions(data *PresubmitEmailData) htmlgo.HTML {
	return htmlgo.Table(
		TableAttr(attributes.Class_("actions-table")),
		htmlgo.Tr_(
			htmlgo.Td(htmlgo.Attr(attributes.Align_("center"), attributes.Class_("btn btn-primary")),
				htmlgo.A(htmlgo.Attr(attributes.Href_(data.ReviewURL)), htmlgo.Text("See Review")),
			),
			htmlgo.Td(htmlgo.Attr(attributes.Align_("center"), attributes.Class_("btn btn-primary")),
				htmlgo.A(htmlgo.Attr(attributes.Href_(data.EbertURL)), htmlgo.Text("Ebert Review")),
			),
			htmlgo.Td(htmlgo.Attr(attributes.Align_("center"), attributes.Class_("btn btn-primary")),
				htmlgo.A(htmlgo.Attr(attributes.Href_(data.ResultsURL)), htmlgo.Text("See Logs")),
			),
		),
	)
}

// -------------------------------------------------------------------------------------------------

// Embedded content of build/cicd/ciruuner/css/email.css
var cssContent = `
.main-table {
  width: 600px;
}

.background-pass {
  background-color: #4ed34e;
  color: white;
}
.background-fail {
  background-color: #ee4c40;
  color: white;
}

.banner {
  font-size: 30px;
  color: white;
}
.banner-table {
  width: 100%;
}
.banner-icon {
  padding: 40px 0 20px 0;
}
.banner-icon img {
  display: block;
}
.banner-title {
  padding-bottom: 20px;
}

.results-body {
  padding: 10px;
}
.no-margin {
  margin: 0;
}
.results-table {
  border: 0;
  width: 100%;
}

.actions-table {
  padding-top: 20px;
  width: 100%;
}
.btn a {
  background-color: #ffffff;
  border: solid 1px #3498db;
  border-radius: 5px;
  box-sizing: border-box;
  color: #3498db;
  cursor: pointer;
  display: inline-block;
  font-size: 14px;
  font-weight: bold;
  margin: 0;
  padding: 12px 25px;
  text-decoration: none;
  text-transform: capitalize;
  width: 50%;
}
.btn-primary table td {
  background-color: #3498db;
}
.btn-primary a {
  background-color: #3498db;
  border-color: #3498db;
  color: #ffffff;
}
.check-result-name {
  width: 100%;
  padding-left: 4px;
}
`
