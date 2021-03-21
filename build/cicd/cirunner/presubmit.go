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

// This file contains some presubmit logic that needs to be handled on the cirunner in case the
// presubmit_runner fails to build/run. This is mostly email handling.

package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"net/http"
	"net/url"

	"sge-monorepo/build/cicd/cirunner/runnertool"
	"sge-monorepo/libs/go/email"
	"sge-monorepo/libs/go/swarm"

	"sge-monorepo/build/cicd/cirunner/protos/cirunnerpb"
)

const (
	// We unify the subject line for all emails so that gmail will then chain them.
	presubmitSubjectFmt = "[sge-ci] Test run for Review %d"
	ebertHost           = "<INSERT_EBERT_HOST>"
)

// PresumbitContext holds all the information needed by cinunner for presubmits.
type PresubmitContext struct {
	presubmitpb  *cirunnerpb.RunnerInvocation_Presubmit
	swarmContext *swarm.Context
	swarmReview  *swarm.Review
	emailClient  email.Client
}

// NewPresubmitContext creates a |PresubmitContext| ready to be used by querying swarm.
func NewPresubmitContext(creds *runnertool.Credentials,
	presubmitpb *cirunnerpb.RunnerInvocation_Presubmit) (*PresubmitContext, error) {
	if creds.Swarm == nil {
		return nil, fmt.Errorf("no swarm credentials present")
	}
	sc := creds.Swarm
	swarmContext := swarm.New(sc.Host, int(sc.Port), sc.Username, sc.Password)
	// We create an SSL unaware HTTP client.
	swarmContext.Client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	swarmReview, err := swarm.GetReview(swarmContext, int(presubmitpb.Review))
	if err != nil {
		return nil, fmt.Errorf("could not get swarm review %d: %v", int(presubmitpb.Review), err)
	}
	// Create the email client.
	if creds.Email == nil {
		return nil, fmt.Errorf("no email credentials present")
	}
	emailClient := email.NewClientWithPlainAuth(creds.Email.Host, int(creds.Email.Port),
		creds.Email.Username, creds.Email.Password)
	return &PresubmitContext{
		presubmitpb:  presubmitpb,
		swarmContext: swarmContext,
		swarmReview:  swarmReview,
		emailClient:  emailClient,
	}, nil
}

func (ctx *PresubmitContext) SendStartEmail() error {
	return ctx.sendEmail(startTemplate)
}

func (ctx *PresubmitContext) SendFailEmail() error {
	return ctx.sendEmail(failTemplate)
}

func (ctx *PresubmitContext) sendEmail(t *template.Template) error {
	email, err := ctx.runTemplate(t)
	if err != nil {
		return fmt.Errorf("could not run email HTML template: %v", err)
	}
	if err := ctx.emailClient.Send(email); err != nil {
		return fmt.Errorf("could not send email: %v", err)
	}
	return nil
}

func (ctx *PresubmitContext) runTemplate(t *template.Template) (*email.Email, error) {
	data := templateData{
		Author:     ctx.swarmReview.Author,
		ReviewID:   int(ctx.presubmitpb.Review),
		ReviewURL:  formatReviewUrl("", ctx.presubmitpb.UpdateUrl, int(ctx.presubmitpb.Review)),
		EbertURL:   formatReviewUrl(ebertHost, ctx.presubmitpb.UpdateUrl, int(ctx.presubmitpb.Review)),
		ResultsURL: ctx.presubmitpb.ResultsUrl,
	}
	templateData := new(bytes.Buffer)
	if err := t.Execute(templateData, data); err != nil {
		return nil, err
	}
	to := []string{toCompanyEmail(ctx.swarmReview.Author)}
	subject := fmt.Sprintf(presubmitSubjectFmt, ctx.presubmitpb.Review)
	// Copy the template email and return a copy to that.
	return &email.Email{
		Subject:     subject,
		To:          to,
		EmailBody:   templateData.String(),
		ContentType: email.ContentTypeHTML,
	}, nil
}

func toCompanyEmail(username string) string {
	return fmt.Sprintf("%s@foo.com", username)
}

// formatReviewUrl gives the valid review URL by extracting the hostname from the incoming url.
// This will make the results URL have the same hostname as what the CI system is exposing.
// If |u| is malformed, this function returns an empty link.
func formatReviewUrl(host string, u string, reviewID int) string {
	parsed, err := url.Parse(u)
	if err != nil {
		return ""
	}
	if host == "" {
		host = parsed.Host
	}
	return fmt.Sprintf("https://%s/review/%d", host, reviewID)
}
