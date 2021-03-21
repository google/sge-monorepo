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

// Package ebert contains the fundamental Ebert types.
package ebert

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"os/user"
	"strings"
	"sync"
	"time"

	"sge-monorepo/build/cicd/jenkins"
	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/swarm"
	"sge-monorepo/tools/ebert/flags"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

// Context is the Ebert context.
type Context struct {
	Ctx       context.Context
	Swarm     swarm.Context
	P4        p4lib.P4
	Jenkins   jenkins.Remote
}

// UserContext returns a login Context for the user making the request.
func (ctx *Context) UserContext(r *http.Request) (*Context, error) {
	user, err := UserFromRequest(r)
	if err != nil {
		return nil, err
	}
	return ctx.Login(user)
}

func (ctx *Context) Login(user string) (*Context, error) {
	// If the context is already for this user, just use the existing context.
	// This is especially useful for local development.
	if user == ctx.Swarm.Username {
		return ctx, nil
	}
	userCtx := *ctx
	userCtx.Swarm.Username = user
	ticket, err := ctx.GetTicket(user)
	if err != nil {
		return nil, err
	}
	userCtx.Swarm.Password = ticket
	userCtx.P4 = p4lib.NewForUser(user, ticket)

	return &userCtx, nil
}

// Trace returns a *Context which will record statistics about P4 and Swarm
// operations, and link to the trace for the original request, similar to a
// dapper trace.
func (ctx *Context) Trace(r *http.Request) *Context {
	rctx := r.Context()
	span := trace.FromContext(rctx)
	if span == nil {
		log.Warningf("no span associated with request context")
		return ctx
	}
	tracer := func(stat string) func() {
		measure := stats.Float64(stat, stat, stats.UnitMilliseconds)
		name := fmt.Sprintf("p4/%s", stat)
		v := view.Find(name)
		if v == nil {
			v := &view.View{
				Name:        name,
				Description: fmt.Sprintf("time spent executing p4 %s", stat),
				Measure:     measure,
				Aggregation: view.Distribution(0, 25, 100, 400, 2000),
			}
			if err := view.Register(v); err != nil {
				log.Errorf("failed to register stats view: %v", err)
				return func() {}
			}
		}
		tctx, span := trace.StartSpan(rctx, name)
		start := time.Now()
		return func() {
			stats.Record(tctx, measure.M(float64(time.Since(start))/float64(time.Millisecond)))
			span.End()
		}
	}
	// Create a Swarm context based on the Ebert context and request context.
	sctx := ctx.Swarm
	sctx.Ctx = rctx
	return &Context{
		Ctx:       rctx,
		P4:        p4lib.WithTracer(ctx.P4, tracer),
		Swarm:     sctx,
		Jenkins:   ctx.Jenkins,
	}
}

var ticketsMutex sync.Mutex

type ticketInfo struct {
	ticket     string
	expiration time.Time
}

var tickets = map[string]ticketInfo{}

func (ctx *Context) GetTicket(user string) (string, error) {
	const expirationThreshold = 5 * time.Minute

	ticketsMutex.Lock()
	defer ticketsMutex.Unlock()
	if info, ok := tickets[user]; ok {
		if time.Until(info.expiration) > expirationThreshold {
			return info.ticket, nil
		}
		delete(tickets, user)
	}
	ticket, expiration, err := ctx.P4.Login(user)
	if err != nil {
		return "", err
	}
	tickets[user] = ticketInfo{
		ticket:     ticket,
		expiration: expiration,
	}
	return ticket, nil
}

func NewContext() (*Context, error) {
	var remote jenkins.Remote
	creds, err := jenkins.CredentialsForProject("INSERT_PROJECT")
	if err != nil {
		log.Errorf("could not get jenkins credentials: %v", err)
	} else {
		// Ebert never runs on a Jenkins instance, so we always set the host.
		creds.Host = flags.Jenkins
		remote = jenkins.NewRemote(creds)
	}

	if flags.P4User == "" {
		return nil, fmt.Errorf("invalid P4 user")
	}
	passwd := flags.P4Passwd
	// If no password, try p4 tickets.
	p4 := p4lib.New()
	if passwd == "" {
		tickets, err := p4.Tickets()
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve p4 tickets: %v", err)
		}
		for _, ticket := range tickets {
			if ticket.User == flags.P4User {
				passwd = ticket.ID
			}
		}
	}
	// If still no password, give up.
	if passwd == "" {
		return nil, fmt.Errorf("No ticket for API user %s", flags.P4User)
	}

	// Ensure future p4 commands are run as the api user.
	if err := os.Setenv("P4USER", flags.P4User); err != nil {
		return nil, fmt.Errorf("Failed to set P4USER=%s: %v", flags.P4User, err)
	}
	if err := os.Setenv("P4PASSWD", passwd); err != nil {
		return nil, fmt.Errorf("Failed to set P4PASSWD: %v", err)
	}

	ctx := &Context{
		Ctx: context.Background(),
		Swarm: swarm.Context{
			Host:     flags.ApiHost,
			Port:     flags.ApiPort,
			Username: flags.P4User,
			Password: passwd,
			Ctx:      context.Background(),
			Client: &http.Client{
				Transport: &ochttp.Transport{},
			},
		},
		P4:        p4,
		Jenkins:   remote,
	}
	if flags.ApiAddr != "" {
		ctx.Swarm.Host = flags.ApiHost
		ctx.Swarm.Client = &http.Client{
			Transport: &ochttp.Transport{
				Base: &http.Transport{
					TLSClientConfig: &tls.Config{
						ServerName: flags.ApiHost,
					},
				},
			},
		}
	}
	return ctx, nil
}

func UserFromRequest(r *http.Request) (string, error) {
	// Fallback to the user the process is running as.  This is really only
	// useful during development.
	current, err := user.Current()
	if err != nil {
		return "", err
	}
	lastSlash := strings.LastIndex(current.Username, "\\") + 1
	return string(current.Username[lastSlash:]), nil
}

func NewError(err error, msg string, code int) error {
	return &Error{
		error:   err,
		Code:    code,
		Message: msg,
	}
}

type Error struct {
	error
	Code    int
	Message string
}

func (e Error) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return e.error.Error()
}

func (e Error) Unwrap() error {
	return e.error
}
