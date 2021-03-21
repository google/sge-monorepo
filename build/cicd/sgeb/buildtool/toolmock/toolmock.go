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

package toolmock

import (
	"fmt"
	"os"
	"path"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/sgeb/buildtool"

	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
)

var _ buildtool.Helper = (*Helper)(nil)

type Helper struct {
	monorepo   monorepo.Monorepo
	invocation *buildpb.ToolInvocation
}

func NewHelper(invocation *buildpb.ToolInvocation) *Helper {
	mr := monorepo.New("", map[string]monorepo.Path{})
	return &Helper{
		monorepo:   mr,
		invocation: invocation,
	}
}

func (h *Helper) Invocation() *buildpb.ToolInvocation {
	return h.invocation
}

func (h *Helper) ResolveBuildPath(path string) (string, error) {
	panic("implement me")
}

func (h *Helper) ResolvePath(p string) (string, error) {
	return h.resolvePath("", p)
}

func (h *Helper) DeclareOutput(p string) (filepath, stablePath string) {
    panic("implement me")
}

func (h *Helper) MustWriteBuildResult(*buildpb.BuildInvocationResult) {
}

func (h *Helper) MustWriteTestResult(*buildpb.TestInvocationResult) {
}

func (h *Helper) MustWritePublishResult(*buildpb.PublishInvocationResult) {
}

func (h *Helper) LogLabels() map[string]string {
	return nil
}

func (h *Helper) resolvePath(relTo monorepo.Path, p string) (string, error) {
	runFiles, ok := os.LookupEnv("TEST_SRCDIR")
	if !ok {
		return "", fmt.Errorf("could not find runfiles dir env var $TEST_SRCDIR")
	}
	workspaceName, ok := os.LookupEnv("TEST_WORKSPACE")
	if !ok {
		return "", fmt.Errorf("could not find workspace name $TEST_WORKSPACE")
	}
	mrp, err := h.monorepo.NewPath(relTo, p)
	if err != nil {
		return "", err
	}
	return path.Join(runFiles, workspaceName, string(mrp)), nil
}
