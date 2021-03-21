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

// Package jenkins exposes Jenkins endpoints.
// Note that the provided credentials are mostly intented to be used in the CI machines context, so
// explicit attention should be given to this credentials in order to enable request into Jenkins
// from a local workstation.

package jenkins

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"sge-monorepo/libs/go/cloud/secretmanager"

	"sge-monorepo/build/cicd/cirunner/protos/cirunnerpb"

	"github.com/golang/protobuf/proto"
)

const (
	presubmitUrl = "job/presubmits/job/presubmit/buildWithParameters"
	unitUrl      = "job/unit/job/unit_runner/buildWithParameters"
)

// UnitOption is a function that modifies a UnitOptions in the context of a request.
type UnitOption func(*UnitOptions)

// UnitOptions contains the options for a unit_request.
type UnitOptions struct {
	// BaseCl is the CL that the runner should sync to.
	BaseCl int

	// Change represents a CL the unit_runner should unshelve before executing.
	Change int

	// TaskKey is the P4 Key that the runner will communicate information back in.
	TaskKey string

	// LogLevel will be forwarded to the build.
	LogLevel string

	// Invoker is who is issuing the request.
	Invoker string

	// InvokerUrl is a information URL related to the |Invoker|. The canonical example is that when
	// the invoker is `Jenkins`, the |InvokerUrl| could be the logs of the job run that triggered
	// this request.
	InvokerUrl string

	// Args will be forwarded to the build.
	// Will be semicolon-separated.
	// Currently ignored for any action except publish actions.
	Args []string
}

// Remote can perform remote build and presubmit commands.
type Remote interface {
	// SendBuildRequest sends a request for Jenkins to build a build_unit represented by |label|.
	SendBuildRequest(label string, opts ...UnitOption) error

	// SendPublishRequest sends a request for Jenkins to publish a publish_unit represented by |label|.
	SendPublishRequest(label string, opts ...UnitOption) error

	// SendTestRequest sends a request for Jenkins to test a test_unit represented by |label|.
	SendTestRequest(label string, opts ...UnitOption) error

	// SendTaskRequest sends a request for Jenkins to run a task_unit represented by |label|.
	SendTaskRequest(label string, opts ...UnitOption) error

	// SendPresubmitRequest sends a presubmit request to the select jenkins endpoint.
	SendPresubmitRequest(presubmitpb *cirunnerpb.RunnerInvocation_Presubmit) error
}

func NewRemote(creds *cirunnerpb.JenkinsCredentials) Remote {
	return &remote{creds}
}

type remote struct {
	creds *cirunnerpb.JenkinsCredentials
}

func (r *remote) SendBuildRequest(label string, opts ...UnitOption) error {
	options := &UnitOptions{}
	for _, opt := range opts {
		opt(options)
	}
	params := map[string]string{
		"buildUnit": label,
	}
	if err := addParamsFromOptions(options, params); err != nil {
		return err
	}
	body, err := SendJenkinsRequest(r.creds, "POST", unitUrl, params)
	if err != nil {
		return fmt.Errorf("Could not send build request: %v. Response:\n%s", err, body)
	}
	return nil
}

func (r *remote) SendPresubmitRequest(presubmitpb *cirunnerpb.RunnerInvocation_Presubmit) error {
	params := map[string]string{
		"token":    r.creds.BuildToken,
		"review":   strconv.Itoa(int(presubmitpb.Review)),
		"change":   strconv.Itoa(int(presubmitpb.Change)),
		"swarmURL": presubmitpb.UpdateUrl,
	}
	body, err := SendJenkinsRequest(r.creds, "POST", presubmitUrl, params)
	if err != nil {
		return fmt.Errorf("Could not send presubmit request: %v. Response:\n%s", err, body)
	}
	return nil
}

func (r *remote) SendPublishRequest(label string, opts ...UnitOption) error {
	options := &UnitOptions{}
	for _, opt := range opts {
		opt(options)
	}
	params := map[string]string{
		"publishUnit": label,
	}
	if err := addParamsFromOptions(options, params); err != nil {
		return err
	}
	body, err := SendJenkinsRequest(r.creds, "POST", unitUrl, params)
	if err != nil {
		return fmt.Errorf("Could not send publish request: %v. Response:\n%s", err, body)
	}
	return nil
}

func (r *remote) SendTestRequest(label string, opts ...UnitOption) error {
	options := &UnitOptions{}
	for _, opt := range opts {
		opt(options)
	}
	params := map[string]string{
		"testUnit": label,
	}
	if err := addParamsFromOptions(options, params); err != nil {
		return err
	}
	body, err := SendJenkinsRequest(r.creds, "POST", unitUrl, params)
	if err != nil {
		return fmt.Errorf("Could not send test request: %v. Response:\n%s", err, body)
	}
	return nil
}

func (r *remote) SendTaskRequest(label string, opts ...UnitOption) error {
	options := &UnitOptions{}
	for _, opt := range opts {
		opt(options)
	}
	params := map[string]string{
		"taskUnit": label,
	}
	if err := addParamsFromOptions(options, params); err != nil {
		return err
	}
	body, err := SendJenkinsRequest(r.creds, "POST", unitUrl, params)
	if err != nil {
		return fmt.Errorf("Could not send task request: %v. Response:\n%s", err, body)
	}
	return nil
}

func addParamsFromOptions(options *UnitOptions, params map[string]string) error {
	if options.Change != 0 {
		params["change"] = strconv.Itoa(options.Change)
	}
	if options.BaseCl != 0 {
		params["baseCl"] = strconv.Itoa(options.BaseCl)
	}
	if options.TaskKey != "" {
		params["taskKey"] = options.TaskKey
	}
	if options.LogLevel != "" {
		params["logLevel"] = options.LogLevel
	}
	if options.Invoker != "" {
		params["invoker"] = options.Invoker
	}
	if options.InvokerUrl != "" {
		params["invokerUrl"] = options.InvokerUrl
	}
	if len(options.Args) != 0 {
		for _, arg := range options.Args {
			if strings.Contains(arg, ";") {
				return fmt.Errorf("args cannot currently contain semicolons, found %q", arg)
			}
		}
		params["args"] = strings.Join(options.Args, ";")
	}
	return nil
}

// SendJenkinsRequest sends an arbitrary HTTP command to Jenkins. Most of the other calls within
// this lib are made using this.
// |verb| is an HTTP verb in caps: "GET", "POST", etc.
// |path| is the path within the host.
// |params| will be encoded into the request.
// Returns the body of the request in all cases.
func SendJenkinsRequest(creds *cirunnerpb.JenkinsCredentials, verb, path string, params map[string]string) (string, error) {
	hostname := fmt.Sprintf("https://%s:%d/", creds.Host, creds.Port)
	baseUrl, err := url.Parse(hostname)
	if err != nil {
		return "", fmt.Errorf("could not parse hostname %s: %v", hostname, err)
	}
	baseUrl.Path += path
	urlParams := url.Values{}
	urlParams.Add("token", creds.BuildToken)
	for k, v := range params {
		urlParams.Add(k, v)
	}
	baseUrl.RawQuery = urlParams.Encode()
	// We create the HTTP request.
	req, err := http.NewRequest(verb, baseUrl.String(), nil)
	if err != nil {
		return "", fmt.Errorf("could not create HTTP request: %v", err)
	}
	req.SetBasicAuth(creds.Username, creds.ApiKey)
	// Sadly, we cannot make a correct call to jenkins because the DNS entry will attempt to connect
	// through the external IP, which Jenkins won't have access to. Connecting via the internal ip
	// will work, but we need to ignore the certificate.
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	// Sending the shadow message is best effort.
	r, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("could not send HTTP request: %v", err)
	}
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r.Body)
	body := buf.String()
	if !(r.StatusCode == 200 || r.StatusCode == 201) {
		return body, fmt.Errorf("jenkins unexpected status code %d", r.StatusCode)
	}
	return body, nil
}

// CredentialsForProject returns the jenkins credentials associated with a project.
// Requires the authenticated user to have access to those secrets.
// If |project| is empty, it will try the default project set with gcloud.
func CredentialsForProject(project string) (*cirunnerpb.JenkinsCredentials, error) {
	secrets, err := func() (secretmanager.SecretManager, error) {
		if project == "" {
			return secretmanager.NewFromDefaultProject()
		} else {
			return secretmanager.New(project)
		}
	}()
	if err != nil {
		return nil, fmt.Errorf("could not create secretmanager client: %w", err)
	}
	secret, ok, err := secrets.AccessLatest("jenkins_credentials")
	if err != nil || !ok {
		return nil, fmt.Errorf("could not get jenkins_credentials secret for project %q (nil == not found): %v", project, err)
	}
	creds := &cirunnerpb.JenkinsCredentials{}
	if err := proto.UnmarshalText(secret, creds); err != nil {
		return nil, fmt.Errorf("could not unmarshal jenkins credentials proto: %w", err)
	}
	return creds, nil
}
