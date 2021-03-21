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

package runnertool

import (
	"fmt"

	"sge-monorepo/libs/go/cloud/secretmanager"

	"sge-monorepo/build/cicd/cirunner/protos/cirunnerpb"

	"github.com/golang/protobuf/proto"
)

// Credentials are all the credentials used by cirunner.
type Credentials struct {
	// Environment is the configuration proto that informs about the environment cirunner is
	// executing one. It is required to be present.
	Environment *cirunnerpb.Environment
	// Email are credentials for communicating with the Email server.
	Email *cirunnerpb.Connection
	// Swarm are the credentials for communicating with the Swarm server.
	Swarm *cirunnerpb.Connection
	// ShadowJenkins are credentials for communication with the shadow jenkins server.
	// This is mostly used for having a secondary server mimic jobs.
	ShadowJenkins *cirunnerpb.JenkinsCredentials
}

// NewCredentials loads the credentials from the secret manager.
func NewCredentials() (*Credentials, error) {
	secrets, err := secretmanager.NewFromDefaultProject()
	if err != nil {
		return nil, fmt.Errorf("could not create secretmanager client: %v", err)
	}
	credentials := &Credentials{}
	// Load the environment proto.
	if secret, _, err := secrets.AccessLatest("cirunner_environment"); err != nil {
		return nil, fmt.Errorf("could not load cirunner_environment secret (nil == not found): %v", err)
	} else {
		environment := &cirunnerpb.Environment{}
		if err := proto.UnmarshalText(secret, environment); err != nil {
			return nil, fmt.Errorf("could not unmarshal environment proto %s: %v", secret, err)
		}
		credentials.Environment = environment
	}
	// Attempt to load the email credentials.
	if secret, ok, err := secrets.AccessLatest("cirunner_email_credentials"); err != nil {
		return nil, fmt.Errorf("could not load cirunner_email_credentials: %v", err)
	} else if ok {
		email := &cirunnerpb.Connection{}
		if err := proto.UnmarshalText(secret, email); err != nil {
			return nil, fmt.Errorf("could not unmarshal email credentials %s: %v", secret, err)
		}
		credentials.Email = email
	}
	// Attemp to load the swarm credentials.
	if secret, ok, err := secrets.AccessLatest("cirunner_swarm_credentials"); err != nil {
		return nil, fmt.Errorf("could not load cirunner_swarm_credentials: %v", err)
	} else if ok {
		swarm := &cirunnerpb.Connection{}
		if err := proto.UnmarshalText(secret, swarm); err != nil {
			return nil, fmt.Errorf("could not unmarshal swarm credentials %s: %v", secret, err)
		}
		credentials.Swarm = swarm
	}
	// Attempt to load the Jenkins credentials.
	if secret, ok, err := secrets.AccessLatest("cirunner_shadow_jenkins_credentials"); err != nil {
		return nil, fmt.Errorf("could not load cirunner_shadow_jenkins_credentials: %v", err)
	} else if ok {
		jenkins := &cirunnerpb.JenkinsCredentials{}
		if err := proto.UnmarshalText(secret, jenkins); err != nil {
			return nil, fmt.Errorf("could not unmarshal shadow jenkins creds %s: %v", secret, err)
		}
		credentials.ShadowJenkins = jenkins
	}
	return credentials, nil
}
