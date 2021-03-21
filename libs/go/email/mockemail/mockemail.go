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

// Package mockemail provides facilities to mock the email package.
package mockemail

import (
	"sge-monorepo/libs/go/email"
)

var _ email.Client = (*Client)(nil)

// NewClient returns a new mock email client.
func NewClient() *Client {
	return &Client{}
}

type Client struct {
	sendCount int
}

func (c *Client) Send(*email.Email) error {
	c.sendCount = c.sendCount + 1
	return nil
}

// SendCount returns the number of sent emails.
func (c *Client) SendCount() int {
	return c.sendCount
}
