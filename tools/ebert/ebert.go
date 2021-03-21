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

// Ebert is SG&E's code/content review tool.
//
// Building and running Ebert.
// On Windows, Ebert must be built with the windows-gnu toolchain.
// `bazel build --config=windows-gnu tools/ebert`
//
// * running locally
//   `ebert --dev`
// This will start Ebert in dev mode on the default port 8088.
// Running Ebert locally requires the --dev flag.
//
// * running as Swarm
//   `ebert --dev --user=swarm --passwd=<swarm password>`
// When deployed to GCP Ebert runs as the Swarm user, and I often find
// it useful to force Ebert to use the Swarm user when running
// locally, to ensure that there are no hidden assumptions.  Ebert
// will honor the P4USER and P4PASSWD environment variables, and I
// often find it easier to just `set P4USER=swarm` and `set
// P4PASSWD=<swarm password>` in my shell.  If you are an admin, the
// swarm password can be retrived with `p4 login -a -p swarm`.
//
// * running with SSL
//   `ebert --dev --cert=<path to cert.pem> --key=<path to cert.key>`
// Mostly useful for testing SSL
//
// General structure:
// Ebert is an HTTP server that generally serves two types of data.
// * HTML pages (dashboard, reviews, browser)
// * REST handlers (used by the above)
//
// The HTML handlers are defined via 'dotfns' and Go templates.  The dotfns
// are functions which take an Ebert context and http.Request as arguments,
// and return a 'dot' and any error.  A 'dot' is simply a
// map[string]interface{}, and is so named because it will be used as the "dot"
// when expanding a Go template.  Each dotfn has a corresponding template,
// and the expanded contents of that template will be returned by the handler.
// Most of the machinery around dotfns is found in webui.go, though actual
// handlers tend to be distributed by function (i.e. the dashboard handler
// is in dashboard.go, the review handler is in review.go, etc.)
//
// The REST handlers generally return JSON data, but more specifically return
// raw data, not HTML.  They are generally prefixed by /ebert and are usually
// invoked by Javascript running in the user's browser.  The common code for
// REST handlers is in rest.go, but the actual handlers are distributed by
// function, and may reside in the same file as the HTML handler (for example,
// many REST handlers used by the review page are defined in review.go).

package main

import (
	"context"

	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/log/cloudlog"
	"sge-monorepo/tools/ebert/ebert"
	"sge-monorepo/tools/ebert/flags"
	"sge-monorepo/tools/ebert/handlers/browse"
	"sge-monorepo/tools/ebert/handlers/comments"
	"sge-monorepo/tools/ebert/handlers/dashboard"
	"sge-monorepo/tools/ebert/handlers/files"
	"sge-monorepo/tools/ebert/handlers/project"
	"sge-monorepo/tools/ebert/handlers/review"
	"sge-monorepo/tools/ebert/handlers/trigger"
	"sge-monorepo/tools/ebert/watcher"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

const ebertVersion = "0.0.1"

const swarmHost = "INSERT_HOST"
const swarmPort = 9000

func main() {
	flags.Parse()

	var logSinks = []log.Sink{log.NewGlog()}
	if flags.CloudLogID != "" {
		cloudLogger, err := cloudlog.New(flags.CloudLogID)
		if err != nil {
			log.Errorf("failed to create cloud logger: %v", err)
			return
		}
		logSinks = append(logSinks, cloudLogger)
	}

	log.AddSink(logSinks...)
	defer log.Shutdown()

	creds, err := google.FindDefaultCredentials(context.Background(), compute.ComputeScope)
	if err == nil && creds != nil && creds.ProjectID != "" {
		exporter, err := stackdriver.NewExporter(stackdriver.Options{})
		if err != nil {
			log.Errorf("failed to create stackdriver exporter: %v", err)
			return
		}
		defer exporter.Flush()

		if err := exporter.StartMetricsExporter(); err != nil {
			log.Errorf("failed to start metrics exporter: %v", err)
			return
		}
		defer exporter.StopMetricsExporter()
		log.Infof("metrics exporter started")
	} else {
		project := "<nil>"
		if creds != nil {
			project = creds.ProjectID
		}
		log.Infof("skipping stackdriver exporter: err = %v, project = '%s'", err, project)
	}
	trace.ApplyConfig(trace.Config{DefaultSampler: trace.AlwaysSample()})
	view.Register(ochttp.DefaultClientViews...)
	view.Register(ochttp.DefaultServerViews...)

	dotfns["browse/:path"] = browse.Handle
	dotfns["files/:path$browse"] = browse.Handle
	dotfns["dashboard"] = dashboard.Handle
	dotfns["project/:name"] = project.Handle
	dotfns["projects"] = project.HandleProjects
	dotfns["review/:suffix"] = review.Handle
	restfns["/file/:path"] = files.Handle
	restfns["/ebert/approve/:rid"] = review.Approve
	restfns["/ebert/browse/history/:path"] = browse.History
	restfns["/ebert/comments/:rid"] = comments.Handle
	restfns["/ebert/comments/:rid/:cid"] = comments.Handle
	restfns["/ebert/comments/read/:cid"] = comments.MarkRead
	restfns["/ebert/diff"] = review.Diff
	restfns["/ebert/pairs"] = review.Pairs
	restfns["/ebert/review/:rid"] = review.HandleRest
	restfns["/ebert/testruns/:rid"] = review.TestRuns
	restfns["/ebert/users"] = review.Users
	restfns["/trigger/:trigger"] = trigger.Handle

	ectx, err := ebert.NewContext()
	if err != nil {
		log.Errorf("%v", err)
		return
	}

	bgctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go watcher.Watch(bgctx, ectx)

	done := make(chan struct{})
	ui, err := newWebui(ectx, flags.Port, done)
	if err != nil {
		log.Errorf("%v", err)
		return
	}

	go handleSigTerm(ui, done)

	if flags.Crt == "" && flags.Key == "" {
		log.Errorf("%v", ui.ListenAndServe())
	} else {
		log.Errorf("%v", ui.ListenAndServeTLS(flags.Crt, flags.Key))
	}

	<-done
}
