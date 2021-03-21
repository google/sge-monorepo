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

// Package cloudlog contains a log.Logger implementation that logs to google cloud logs.
package cloudlog

import (
	"context"
	"errors"
	"runtime"

	"sge-monorepo/libs/go/log"

	"cloud.google.com/go/logging"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
	logpb "google.golang.org/genproto/googleapis/logging/v2"
)

// Settings represents the parameters that options can modify about a new Cloud Logger.
type Settings struct {
	// labels represents the group of labels that the logger will add to its entries.
	labels map[string]string
}

func (s *Settings) AddLabels(labels map[string]string) {
	for k, v := range labels {
		s.labels[k] = v
	}
}

// Option represents a configuration of the |Settings| struct.
type Option func(*Settings)

// WithLabels is a shorthand to creating an option that adds labels to the logger.
func WithLabels(labels map[string]string) Option {
	return func(settings *Settings) {
		settings.AddLabels(labels)
	}
}

// CloudLogger represents a running logger. It can be a working version or there is also a no-op
// sink for some cases (eg. running locally).
type CloudLogger interface {
	// Valid is true when this logger will actually log against a cloud project.
	Valid() bool

	// AddLabels adds additional labels to the cloud logger.
	AddLabels(labels map[string]string)

	// CloudLogger implements the log.Sink interface.
	log.Sink
}

// New returns a new cloud logger. If no error is returning, a working CloudLogger is returned,
// which can be used as a log.Logger. Depending on the working environemnt, this logger might be a
// working logger or a sink (eg. when running locally, there is no cloud project to log to).
// Most cases don't care about the difference so you can use the logger seamlessly, but you can
// consult |Valid| to check which version you have if needed.
func New(logId string, options ...Option) (CloudLogger, error) {
	ctx := context.Background()
	creds, err := google.FindDefaultCredentials(ctx, compute.ComputeScope)
	if err != nil {
		return nil, err
	}
	// Running on a local machine?
	if creds.ProjectID == "" {
		return nil, errors.New("unable to create cloud logger: cannot find cloud project id")
	}
	client, err := logging.NewClient(ctx, creds.ProjectID)
	if err != nil {
		return nil, err
	}
	logger := client.Logger(logId)
	c := &impl{
		client: client,
		logger: logger,
	}
	// Apply the options.
	settings := Settings{
		labels: map[string]string{},
	}
	for _, option := range options {
		option(&settings)
	}
	c.labels = settings.labels
	return c, nil
}

// Impl --------------------------------------------------------------------------------------------

type impl struct {
	client *logging.Client
	logger *logging.Logger
	labels map[string]string
}

// Valid returns whether this logging is able to actually log into cloud logging.
// It might be false in the case of running in a local environment.
func (c *impl) Valid() bool {
	return true
}

func (c *impl) AddLabels(labels map[string]string) {
	for k, v := range labels {
		c.labels[k] = v
	}
}

func (c *impl) DebugDepth(depth int, msg string) {
	c.log(logging.Debug, msg)
}

func (c *impl) InfoDepth(depth int, msg string) {
	c.log(logging.Info, msg)
}

func (c *impl) WarningDepth(depth int, msg string) {
	c.log(logging.Warning, msg)
}

func (c *impl) ErrorDepth(depth int, msg string) {
	c.log(logging.Error, msg)
}

func (c *impl) Close() {
	_ = c.client.Close()
}

func (c *impl) log(severity logging.Severity, payload string) {
	var location *logpb.LogEntrySourceLocation
	_, file, line, ok := runtime.Caller(4)
	if ok {
		location = &logpb.LogEntrySourceLocation{
			File: file,
			Line: int64(line),
		}
	}
	// Copy the labels, as Logger.Log takes ownership of the map.
	labels := make(map[string]string)
	for k, v := range c.labels {
		labels[k] = v
	}
	entry := logging.Entry{
		Severity:       severity,
		Labels:         labels,
		Payload:        payload,
		SourceLocation: location,
	}
	c.logger.Log(entry)
}
