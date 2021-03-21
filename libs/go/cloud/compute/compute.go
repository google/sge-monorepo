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

// Package compute is a convenience wrapper over the GCP compute golang API.

package compute

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"google.golang.org/api/compute/v1"
)

// Compute is an abstract interface to refer to compute APIs.
// The interface is assocaited with a particular GCP project. You can state the GCP project
// explicitly using |New| or you can obtain the gcloud default one with |NewFromDefaultProject|.
// NOTE: |NewFromDefaultProject| requires gcloud to be in PATH.
type Compute interface {
	// InstancesList lists the instances associated with the given |zones|. Note that each specific
	// zone is a new query, which might take some time. If you query many zones, you might want to
	// query without zones. If no zone is given, it list all the instances within the project. This
	// goes through another path which makes one sole query.
	InstancesList(zones ...string) ([]*compute.Instance, error)
}

func New(project string) (Compute, error) {
	ctx := context.Background()
	service, err := compute.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("coult not create compute service: %v", err)
	}
	return &impl{
		project: project,
		ctx:     ctx,
		service: service,
	}, nil
}

// NewFromDefaultProject creates a compute service interface associated with the current project set
// up as "core/project" within gcloud. If you need another way to define the GCP project, use |New|.
// NOTE: This requires for gcloud to be within PATH.
func NewFromDefaultProject() (Compute, error) {
	out, err := exec.Command("gcloud", "config", "get-value", "core/project").Output()
	if err != nil {
		return nil, fmt.Errorf("could not obtain default project from gcloud: %v", err)
	}
	project := strings.TrimSpace(string(out))
	return New(project)
}

// Implementation ----------------------------------------------------------------------------------

type impl struct {
	project string
	ctx     context.Context
	service *compute.Service
}

func (c *impl) InstancesList(zones ...string) ([]*compute.Instance, error) {
	// No zones queries all the zones.
	if len(zones) == 0 {
		result, err := c.service.Instances.AggregatedList(c.project).Do()
		if err != nil {
			return nil, fmt.Errorf("could not get aggregated list for project %q: %v", c.project, err)
		}
		var instances []*compute.Instance
		for _, instanceList := range result.Items {
			for _, instance := range instanceList.Instances {
				instances = append(instances, instance)
			}
		}
		return instances, nil
	}
	// We do a call per zone.
	// TODO: If this gets too slow, we might want to parallize.
	var instances []*compute.Instance
	for _, zone := range zones {
		result, err := c.service.Instances.List(c.project, zone).Do()
		if err != nil {
			return nil, fmt.Errorf("could not list instances for %q, zone %q: %v", c.project, zone, err)
		}
		for _, instance := range result.Items {
			instances = append(instances, instance)
		}
	}
	return instances, nil
}
