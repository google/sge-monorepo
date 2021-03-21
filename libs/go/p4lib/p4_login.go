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

package p4lib

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// P4 API callbacks for Login command.
type logincb struct {
	ticket     string
	expiration int
}

// outputInfo gets the ticket
func (cb *logincb) outputInfo(level int, info string) error {
	cb.ticket = strings.TrimSpace(info)
	return nil
}

// outputStat gets the expiration
func (cb *logincb) outputStat(stats map[string]string) error {
	if value, ok := stats["TicketExpiration"]; ok {
		expiration, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("can't convert TicketExpiration(%s) to int: %v", value, err)
		}
		cb.expiration = expiration
	}
	return nil
}

// Implement tagProtocol to trigger outputStat
func (cb *logincb) tagProtocol() {}

// Login returns the ticket and expiration for the specified user, or an error.
func (p4 *impl) Login(user string) (string, time.Time, error) {
	cb := logincb{}
	// The -a argument ensures that the ticket is valid for all host machines.
	// The -p argument prints the resulting ticket.
	err := p4.runCmdCb(&cb, "login", "-a", "-p", user)
	if err != nil {
		return "", time.Time{}, err
	}
	return cb.ticket, time.Now().Add(time.Duration(cb.expiration) * time.Second), nil
}

