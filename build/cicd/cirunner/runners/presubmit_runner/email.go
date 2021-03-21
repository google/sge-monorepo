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
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"

	"sge-monorepo/build/cicd/cirunner/ciemail"
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
	var emailClient email.Client
	if creds.Email != nil {
		emailClient = email.NewClientWithPlainAuth(
			creds.Email.Host,
			int(creds.Email.Port),
			creds.Email.Username,
			creds.Email.Password,
		)
	}
	return &PresubmitContext{
		presubmitpb:  presubmitpb,
		swarmContext: swarmContext,
		swarmReview:  swarmReview,
		emailClient:  emailClient,
	}, nil
}

func (ctx *PresubmitContext) SendSwarmPass() error {
	return ctx.SendSwarmRequest(swarm.TestRunPass)
}

func (ctx *PresubmitContext) SendSwarmFail() error {
	return ctx.SendSwarmRequest(swarm.TestRunFail)
}

func (ctx *PresubmitContext) SendSwarmRequest(t swarm.TestRunResponseType) error {
	update := ctx.presubmitpb.UpdateUrl
	results := ctx.presubmitpb.ResultsUrl
	if _, err := swarm.SendTestRunRequest(ctx.swarmContext, t, update, results); err != nil {
		return err
	}
	return nil
}

func toEmailCheckResults(results []CheckResult) []ciemail.CheckResult {
	var emailResults []ciemail.CheckResult
	for _, r := range results {
		emailResults = append(emailResults, ciemail.CheckResult{
			Name:   r.Check.Name(),
			Check:  r.Check,
			Result: r.Result,
		})
	}
	return emailResults
}

func (ctx *PresubmitContext) SendPassEmail(results []CheckResult) error {
	return ctx.sendEmail(true, results)
}

func (ctx *PresubmitContext) SendFailEmail(results []CheckResult) error {
	return ctx.sendEmail(false, results)
}

func (ctx *PresubmitContext) sendEmail(success bool, results []CheckResult) error {
	if ctx.emailClient == nil {
		return fmt.Errorf("no email client provided")
	}
	data := &ciemail.PresubmitEmailData{
		Author:     ctx.swarmReview.Author,
		ReviewID:   int(ctx.presubmitpb.Review),
		ChangeID:   int(ctx.presubmitpb.Change),
		ReviewURL:  formatReviewUrl("", ctx.presubmitpb.UpdateUrl, int(ctx.presubmitpb.Review)),
		EbertURL:   formatReviewUrl(ebertHost, ctx.presubmitpb.UpdateUrl, int(ctx.presubmitpb.Review)),
		ResultsURL: ctx.presubmitpb.ResultsUrl,
		Success:    success,
		Results:    toEmailCheckResults(results),
	}
	e := ciemail.NewPresubmitEmail(data)
	if err := ctx.emailClient.Send(e); err != nil {
		return fmt.Errorf("could not send email: %v", err)
	}
	return nil
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
