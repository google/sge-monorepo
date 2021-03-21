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
    "embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"mime"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/tools/ebert/ebert"
	"sge-monorepo/tools/ebert/flags"
	"sge-monorepo/tools/ebert/handlers"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/zpages"
)

// Assets
//go:embed *.css *.ico *.png
var assetsFS embed.FS

const (
	avatarUrlFmt = "https://picsum.photos/200"
)

var (
	dotfns  = map[string]interface{}{}
	restfns = map[string]interface{}{}
	funcMap = template.FuncMap{
		"json": func(x interface{}) (template.JS, error) {
			bytes, err := json.Marshal(x)
			return template.JS(bytes), err
		},
	}
)

func handleError(w http.ResponseWriter, err error) {
	var e *ebert.Error
	code := http.StatusInternalServerError
	if errors.As(err, &e) {
		log.Errorf("underlying error: %v", e.Unwrap())
		code = e.Code
	} else {
		log.Errorf("error: %v", err)
	}
	http.Error(w, err.Error(), code)
}

func errorMsg(w http.ResponseWriter, code int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	log.Errorf(msg)
	http.Error(w, msg, code)
}

type statuszMap map[string]view.DistributionData

// ExportView implements the view.Exporter interface for statuszMap.
// It is invoked by OpenCensus to export a view of a stat, and simply copies
// the DistributionData for the stat.
// Currently only handles Distribution stat views.
func (s *statuszMap) ExportView(viewData *view.Data) {
	for _, row := range viewData.Rows {
		suffix := ""
		if len(row.Tags) > 0 {
			var builder strings.Builder
			builder.WriteString(":")
			for _, tag := range row.Tags {
				builder.WriteString(fmt.Sprintf(" %s=%s", tag.Key.Name(), tag.Value))
			}
			suffix = builder.String()
		}
		key := fmt.Sprintf("%s%s", viewData.View.Name, suffix)
		switch r := row.Data.(type) {
		case *view.DistributionData:
			(*s)[key] = *r
		}
	}
}

var statusz = statuszMap{}

var patternRE = regexp.MustCompile(`^([^\/]+)(?:[^\$]*)(\$\w+)?`)

func newWebui(ctx *ebert.Context, port int, done chan struct{}) (*http.Server, error) {
	ui := &http.Server{
		Addr: fmt.Sprintf(":%d", port),
	}

	http.HandleFunc("/quitquitquit", func(w http.ResponseWriter, r *http.Request) {
		close(done)
		ui.Shutdown(r.Context())
	})
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok\n")
	})
	zpages.Handle(nil, "/debug")
	view.RegisterExporter(&statusz)
	http.HandleFunc("/statusz", func(w http.ResponseWriter, r *http.Request) {
		keys := make([]string, 0, len(statusz))
		for k := range statusz {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, name := range keys {
			stats := statusz[name]
			fmt.Fprintf(w, "%s: %f (%f) [%f %f]\n", name, stats.Mean, stats.SumOfSquaredDev, stats.Min, stats.Max)
		}
		fmt.Fprintf(w, "%v", p4lib.Stats)
	})
	// Handle /versionz requests by reporting the suffix of the executable.
	// This is used by the puppet automation.
	http.HandleFunc("/versionz", func(w http.ResponseWriter, r *http.Request) {
		version := ""
		exe, _ := os.Executable()
		idx := strings.LastIndex(exe, "_")
		if idx >= 0 {
			version = exe[idx+1:]
		}
		fmt.Fprintf(w, "%s", version)
	})
	swarmHostPort := fmt.Sprintf("%s:%d", flags.ApiHost, flags.ApiPort)
	if !strings.HasPrefix(swarmHostPort, "http") {
		swarmHostPort = fmt.Sprintf("https://%s", swarmHostPort)
	}
	swarmUrl, err := url.Parse(swarmHostPort)
	if err != nil {
		return nil, fmt.Errorf("failed to build Swarm URL: %w", err)
	}
	// Reverse proxy all "/api/" requests to Swarm.  This will allow us to
	// swap Ebert in for Swarm with tools like p4v.  From there we can
	// determine how various tools use the Swarm API so that we can reverse
	// engineer the functionality in a way that lets Ebert stand in for Swarm
	// when Swarm is eventually retired.
	http.Handle("/api/", &httputil.ReverseProxy{
		Director: func(r *http.Request) {
			log.Infof("proxying %s to %v", r.URL.Path, swarmUrl)
			r.URL.Scheme = swarmUrl.Scheme
			r.URL.Host = swarmUrl.Host
			r.Header.Set("Host", flags.ApiHost)
			// The assumption here is that nobody is hitting Ebert without
			// going through a proxy (except in development) and it looks
			// like the proxy may strip BasicAuth headers, so if we don't
			// find any auth headers, add them.
			if _, _, ok := r.BasicAuth(); !ok {
				log.Infof("Adding auth headers to Swarm API request.")
				r.SetBasicAuth(flags.P4User, flags.P4Passwd)
			}
			if _, ok := r.Header["User-Agent"]; !ok {
				// explicitly disable User-Agent so it's not set to default.
				r.Header.Set("User-Agent", "")
			}
		},
	})

	mux := &handlers.Mux{}
	// for path, h := range dotfns {
	// 	matches := patternRE.FindStringSubmatchIndex(path)
	// 	name := path[:matches[3]]
	// 	if matches[4] < matches[5] {
	// 		name = path[matches[4]+1 : matches[5]]
	// 	}
	// 	pattern := path
	// 	if matches[4] > 0 {
	// 		pattern = path[:matches[4]]
	// 	}
	// 	pattern = fmt.Sprintf("/%s", pattern)

	// 	data, ok := EmbeddedData["tmpl-"+name+".html"]
	// 	if !ok {
	// 		log.Warningf("no template found for '%s'", path)
	// 		continue
	// 	}
	// 	tmpl := template.Must(template.New(name).Delims("[[", "]]").Funcs(funcMap).Parse(string(data)))
	// 	handler, err := handlers.Wrap(pattern, h)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("couldn't wrap handler for %s: %w", path, err)
	// 	}
	// 	mux.Handle(pattern, func(ctx *ebert.Context, r *http.Request) (interface{}, error) {
	// 		dot, err := handler.Serve(ctx, r)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		return func(w io.Writer) error {
	// 			return tmpl.Execute(w, dot)
	// 		}, nil
	// 	})
	// }
    assets, err := assetsFS.ReadDir(".")
    if err != nil {
        return nil, fmt.Errorf("could not find embedded assets")
    }
    for _, asset := range assets {
        path := asset.Name()
        data, err := assetsFS.ReadFile(path)
        if err != nil {
            return nil, fmt.Errorf("could not read asset %q", err)
        }
		http.HandleFunc("/"+path, serveData(data, path))
    }
	for pattern, handler := range restfns {
		if err := mux.Handle(pattern, handler); err != nil {
			return nil, fmt.Errorf("couldn't install handler for %s: %w", pattern, err)
		}
	}

	// The mux is used at the root handler -- any path that doesn't match
	// another handler is first checked against the mux, and if that fails,
	// show the not found page.  Everything using the mux is authenticated and
	// instrumented
	http.Handle("/", authenticate(servePages(ctx, mux)))

	return ui, nil
}

func servePages(baseCtx *ebert.Context, mux *handlers.Mux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			r.URL.Path = "/dashboard"
		}
		ctx := baseCtx.Trace(r)
		out, err := mux.Serve(ctx, r)
		if err != nil {
			if errors.Is(err, handlers.ErrRouteNotFound) && r.URL.Path != "/" {
				w.WriteHeader(http.StatusNotFound)
				fmt.Fprintf(w, "<h1>Not Found</h1>")
				return
			}
			handleError(w, err)
		}
		if writer, ok := out.(func(io.Writer) error); ok {
			if err := writer(w); err != nil {
				handleError(w, err)
			}
			return
		}
		if raw, ok := out.([]byte); ok {
			// If the mux gives back raw bytes, don't try JSON encoding.
			// Assume things are already encoded and just pass it through.
			w.Header().Set("Content-Type", http.DetectContentType(raw))
			w.Write(raw)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		encoder := json.NewEncoder(w)
		encoder.SetEscapeHTML(true)
		encoder.SetIndent("", "")

		err = encoder.Encode(out)
		if err != nil {
			handleError(w, fmt.Errorf("Couldn't encode response: %v", err))
		}
	})
}
func authenticate(handler http.Handler) http.Handler {
	instrumentedHandler := &ochttp.Handler{
		Handler:          handler,
		IsPublicEndpoint: true,
		IsHealthEndpoint: func(*http.Request) bool { return false },
	}

	if flags.DevMode {
		return instrumentedHandler
	}

	return newHandler(instrumentedHandler, !flags.DevMode, http.StatusUnauthorized)
}

// newHandler wraps an existing http.Handler.
func newHandler(h http.Handler, authRequired bool, notAuthorizedStatus int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if authRequired {
			ip := r.Header.Get("X-Forwarded-For")
			if ip == "" {
				ip = r.RemoteAddr
			}
			ua := r.Header.Get("User-Agent")
			msg := fmt.Sprintf("unauthorized access attempt for %v (%s) from UA %s @ IP %s", r.URL, r.Host, ua, ip)
			if authRequired {
				log.Error(msg)
				http.Error(w, fmt.Sprintf("Not Authorized"), notAuthorizedStatus)
				return
			}
			log.Warning(msg)
		}
		h.ServeHTTP(w, r.WithContext(r.Context()))
	})
}

func serveData(data []byte, path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		contentType := mime.TypeByExtension(filepath.Ext(path))
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.Write(data)
	}
}
