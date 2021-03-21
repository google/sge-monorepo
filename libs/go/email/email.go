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

// Package email has utilities about handling email.

package email

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
)


// HELO identifier. This is used as an identifier for our server.
// TODO: This should be an argument probably.
var heloIdentifier = "ADD_YOUR_IDENTIFIER"

// Client is a connection to an email server.
type Client interface {
	// Send sends a email via the configured email server.
	Send(e *Email) error
}

type client struct {
	username string
	host     string
	port     int
	auth     smtp.Auth
}

func NewClientWithPlainAuth(host string, port int, username, password string) Client {
	return &client{
		username: username,
		host:     host,
		port:     port,
		auth:     smtp.PlainAuth("", username, password, host),
	}
}

func (c *client) Send(e *Email) error {
	fullbody := e.generateFullBody()
	return sendEmail(c.host, c.port, c.auth, c.username, e.To, []byte(fullbody))
}

// ContentType describes the format of the content of the email.
type ContentType string

const (
	ContentTypeHTML ContentType = "text/html"
	ContentTypeText             = "text/plain"
)

// Email represents a simple email message.
type Email struct {
	// Subject of the email.
	Subject string
	// To are a slice of recipients
	To []string
	// EmailBody is the body of the email.
	EmailBody string
	// ContentType is what format the email content is in.
	// If unset, defaults to |ContentTypeHTML|.
	ContentType ContentType
}

// SendEmail does a hands-on implementation of the smtp protocol that permits correct identification
// with gmail. With the vanilla smtp.SendMail would make gmail throttle the messages, making many
// of them fail with a EOF error.
// See step 2 of https://support.google.com/a/answer/2956491?hl=en (the part about HELO of EHLO
// messages and throttling).
func sendEmail(hostname string, port int, auth smtp.Auth, from string, to []string, body []byte) error {
	if from == "" {
		return errors.New(`no "from" provided`)
	}
	if len(to) == 0 {
		return errors.New(`no "to" provided`)
	}
	client, err := smtp.Dial(fmt.Sprintf("%s:%d", hostname, port))
	if err != nil {
		return fmt.Errorf("failed to connect to smtp server: %v", err)
	}
	defer client.Close()
	// Send HELO command. This is used as an identifier for our server. If skipped, the default go
	// implementation would identify itself as "localhost", which could cause the smtp server to
	// sometimes throttle messages.
	if err := client.Hello(heloIdentifier); err != nil {
		return fmt.Errorf("failed to send HELO command to smtp server: %v", err)
	}
	// Check for STARTTLS extension (which upgrades the connection to use tls).
	if ok, _ := client.Extension("STARTTLS"); ok {
		config := &tls.Config{ServerName: hostname}
		if err := client.StartTLS(config); err != nil {
			return fmt.Errorf("failed to setup STARTTLS with smtp server: %v", err)
		}
	}
	// Now that tls is set, we can send the authentication information.
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("failed to setup smtp auth: %v", err)
	}
	// Set FROM address.
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("failed to send MAIL command to smtp server: %v", err)
	}
	// Set TO addresses
	for _, rcpt := range to {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("failed to send RCPT command to smtp server: %v", err)
		}
	}
	// Write the body content to email.
	dataWriter, err := client.Data()
	if err != nil {
		return fmt.Errorf("failed to send DATA command to smtp server: %v", err)
	}
	if _, err := dataWriter.Write(body); err != nil {
		return fmt.Errorf("failed to write data into email: %v", err)
	}
	if err := dataWriter.Close(); err != nil {
		return fmt.Errorf("failed to close email writer: %v", err)
	}
	// Close the connection.
	if err := client.Quit(); err != nil {
		return fmt.Errorf("failed to send QUIT command to smtp server: %v", err)
	}
	return nil
}

// GenerateFullBody creates the complete email body to the sent down the wire.
// This is based on the other members of |Email|. You can skip this step by setting the information
// on |FullBody| directly.
func (e *Email) generateFullBody() string {
	if e.ContentType == "" {
		e.ContentType = ContentTypeHTML
	}
	var buf []byte
	buf = appendHeader(buf, "Subject", e.Subject)
	buf = appendHeader(buf, "MIME-version", "1.0;")
	buf = appendHeader(buf, "Content-Type", fmt.Sprintf("%s; charset=\"UTF-8\";", e.ContentType))
	buf = appendHeader(buf, "To", strings.Join(e.To, ", "))
	buf = append(buf, "\r\n"...)
	buf = append(buf, e.EmailBody...)
	return string(buf)
}

func appendHeader(buf []byte, header string, data string) []byte {
	return append(buf, fmt.Sprintf("%s: %s\r\n", header, data)...)
}
