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

// package secretmanager is a convenience wrapper over the GCP Secret Manager go API.

package secretmanager

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

// SecretManager is an abstract interface to refer about the secrets of a project.
// Each secret manager is associated to a particular GCP project. You can state the GCP project
// explicitly using |New| or you can obtain the gcloud default one with |NewFromDefaultProject|.
// NOTE: |NewFromDefaultProject| requires gcloud to be in PATH.
//
// Usage:
//      secrets, err := secretmanager.NewFromDefaultProject()
//      ...
//      secret, err := secrets.AccessLatest("some_secret")
//      ...
type SecretManager interface {
	// Returns the associated project of this client.
	Project() string

	// AccessLatest gives the latest version of |secret|. If you need a particular explicit version,
	// refer to |Access|. The boolean indicates whether the secret exists or not, which is different
	// from a runtime error (eg. Could not connect).
	AccessLatest(secret string) (string, bool, error)

	// Access gives the value of |secret| for the given |version|. The boolean indicates whether the
	// secret (and version) exists or not, which is different from a runtime error (eg. Could not
	// connect).
	Access(secret string, version int) (string, bool, error)
}

// New creates a secret manager interface associated with the given |project|.
func New(project string) (SecretManager, error) {
	ctx := context.Background()
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not create secretmanager client: %v", err)
	}
	return &impl{
		project: project,
		ctx:     ctx,
		client:  client,
	}, nil
}

// NewFromDefaultProject creates a secret manager interface associated with the current project set
// up as "core/project" within gcloud. If you need another way to define the GCP project, use |New|.
// NOTE: This requires for gcloud to be within PATH.
func NewFromDefaultProject() (SecretManager, error) {
	out, err := exec.Command("gcloud", "config", "get-value", "core/project").Output()
	if err != nil {
		return nil, fmt.Errorf("could not obtain default project from gcloud: %v", err)
	}
	project := strings.Trim(string(out), "\r\n")
	return New(project)
}

// Implementation ----------------------------------------------------------------------------------

type impl struct {
	project string
	ctx     context.Context
	client  *secretmanager.Client
}

func (s *impl) Project() string {
	return s.project
}

func (s *impl) AccessLatest(secret string) (string, bool, error) {
	secretName := fmt.Sprintf("projects/%s/secrets/%s/versions/latest", s.project, secret)
	return s.accessSecret(secretName)
}

func (s *impl) Access(secret string, version int) (string, bool, error) {
	secretName := fmt.Sprintf("projects/%s/secrets/%s/versions/%d", s.project, secret, version)
	return s.accessSecret(secretName)
}

func (s *impl) accessSecret(secretName string) (string, bool, error) {
	request := &secretmanagerpb.AccessSecretVersionRequest{
		Name: secretName,
	}
	response, err := s.client.AccessSecretVersion(s.ctx, request)
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return "", false, nil
		}
		return "", false, err
	}
	return string(response.Payload.Data), true, nil
}
