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

// Package flags contains all the global flags for Ebert.
package flags

import (
	"flag"
	"os"
	"strconv"
)

var (
	Crt        string
	Key        string
	ApiHost    string
	ApiAddr    string
	ApiPort    int
	P4User     string
	P4Passwd   string
	Port       int
	CloudLogID string
	DevMode    bool
	Jenkins    string
)

// Parse parses the flags contained in this package, including default values derived from the environment.
func Parse() {
	flag.StringVar(&Crt, "cert", "", "Path to certificate file.")
	flag.StringVar(&Key, "key", "", "Path to key file.")
	flag.StringVar(&ApiHost, "apiHost", "", "Hostname for Swarm API backend.")
	flag.StringVar(&ApiAddr, "apiAddr", "", "IP for Swarm API backend.  If empty, apiHost will be used.")
	flag.IntVar(&ApiPort, "apiPort", 9000, "Port for Swarm API backend.")
	flag.StringVar(&P4User, "user", "swarm", "Username for Swarm API/P4.")
	flag.StringVar(&P4Passwd, "passwd", "", "Password for Swarm API/P4.")
	flag.IntVar(&Port, "port", 8088, "Serving port for Ebert.")
	flag.StringVar(&CloudLogID, "cloud_log_id", "", "If set, uses Cloud Logging with the given ID")
	flag.BoolVar(&DevMode, "dev", false, "If enabled, relax authentication.")
	flag.StringVar(&Jenkins, "jenkins", "", "Jenkins Host")

	if v, ok := os.LookupEnv("P4USER"); ok {
		P4User = v
	}
	if v, ok := os.LookupEnv("P4PASSWD"); ok {
		P4Passwd = v
	}
	if v, ok := os.LookupEnv("EBERT_CERT"); ok {
		Crt = v
	}
	if v, ok := os.LookupEnv("EBERT_KEY"); ok {
		Key = v
	}
	if v, ok := os.LookupEnv("SWARM_HOST"); ok {
		ApiHost = v
	}
	if v, ok := os.LookupEnv("SWARM_PORT"); ok {
		if i, err := strconv.Atoi(v); err == nil {
			ApiPort = i
		}
	}
	flag.Parse()
}
