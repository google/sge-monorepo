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

//Package buildtool provides helper functions for build/test/publish tools that interface with sgeb.
// It provides a helper class that assists in loading/writing the protos that sgeb uses to communicate
// with the tools.
package buildtool

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"

	"github.com/golang/protobuf/proto"

	_ "github.com/golang/glog" // Ensure we the binary registers the glog flags
)

var _ Helper = (*helper)(nil)
var toolInvocationFlag = flag.String("tool-invocation", "", "path to build invocation proto")
var toolInvocationResultFlag = flag.String("tool-invocation-result", "", "path to write the invocation result to")

// Helper assists the between the presubmit system and the checker tool.
type Helper interface {
	// Invocation returns the loaded invocation proto.
	Invocation() *buildpb.ToolInvocation

	// ResolveBuildPath resolves a path into a monorepo-relative one.
	// Relative paths are resolved relative to the BUILDUNIT file.
	// Can be used to resolve paths in tool arguments. Since these are opaque
	// strings, the presubmit system is unable to resolve them for you.
	// Input files have already been resolved by the presubmit system.
	ResolveBuildPath(path string) (string, error)

	// ResolvePath resolves path into a monorepo-relative one.
	// This is useful for resolving tool binaries.
	// Relative paths are resolved relative to the monorepo root.
	ResolvePath(path string) (string, error)

	// DeclareOutput returns a (file path, stable path) tuple given a partial output path.
	// The caller is expected to write to the file path, then return an artifact with
	// the given stable path and local file URI.
	// This call is valid only for build invocations.
	DeclareOutput(p string) (filePath string, stablePath string)

	// MustWriteBuildResult writes a build result to --tool-invocation-result.
	MustWriteBuildResult(result *buildpb.BuildInvocationResult)

	// MustWriteTestResult writes a build result to --tool-invocation-result.
	MustWriteTestResult(result *buildpb.TestInvocationResult)

	// MustWritePublishResult writes a publish result to --tool-invocation-result.
	MustWritePublishResult(result *buildpb.PublishInvocationResult)

	// Returns a map of log labels from the invocation.
	LogLabels() map[string]string
}

type helper struct {
	monorepo   monorepo.Monorepo
	invocation *buildpb.ToolInvocation
}

// MustLoad loads a build tool invocation.
// You must call flag.Parse prior to calling this function
// Exits with an error on failure.
func MustLoad() Helper {
	if *toolInvocationFlag == "" {
		fmt.Println("Missing --tool-invocation flag. Did you forget to call flag.Parse?")
		os.Exit(1)
	}
	bytes, err := ioutil.ReadFile(*toolInvocationFlag)
	if err != nil {
		fmt.Printf("failed to load invocation %s: %v\n", *toolInvocationFlag, err)
		os.Exit(1)
	}
	h := helper{}
	h.invocation = &buildpb.ToolInvocation{}
	if err := proto.Unmarshal(bytes, h.invocation); err != nil {
		fmt.Printf("failed to unmarshal invocation proto %s, %v\n", *toolInvocationFlag, err)
		os.Exit(1)
	}
	// pwd is guaranteed to be the monorepo root by the system.
	h.monorepo, _, err = monorepo.NewFromPwd()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return &h
}

func (h *helper) Invocation() *buildpb.ToolInvocation {
	return h.invocation
}

func (h *helper) ResolveBuildPath(p string) (string, error) {
	return h.resolvePath(monorepo.Path(h.invocation.BuildUnitDir), p)
}

func (h *helper) ResolvePath(p string) (string, error) {
	return h.resolvePath("", p)
}

func (h *helper) DeclareOutput(p string) (filePath string, stablePath string) {
	filePath = path.Join(h.invocation.BuildInvocation.OutputDir, p)
	stablePath = path.Join(h.invocation.BuildInvocation.OutputStablePath, p)
	return
}

func (h *helper) resolvePath(relTo monorepo.Path, p string) (string, error) {
	mrp, err := h.monorepo.NewPath(relTo, p)
	if err != nil {
		return "", fmt.Errorf("could not resolve path %s", p)
	}
	return string(mrp), nil
}

func (h *helper) MustWriteBuildResult(result *buildpb.BuildInvocationResult) {
	h.mustWriteResult(result)
}

func (h *helper) MustWriteTestResult(result *buildpb.TestInvocationResult) {
	h.mustWriteResult(result)
}

func (h *helper) MustWritePublishResult(result *buildpb.PublishInvocationResult) {
	h.mustWriteResult(result)
}

func (h *helper) mustWriteResult(result proto.Message) {
	bytes, err := proto.Marshal(result)
	if err != nil {
		fmt.Printf("failed to marshal result proto: %v", err)
		os.Exit(1)
	}
	if err := ioutil.WriteFile(*toolInvocationResultFlag, bytes, 0666); err != nil {
		fmt.Printf("failed to write result proto: %v", err)
		os.Exit(1)
	}
}

func (h *helper) LogLabels() map[string]string {
	labels := map[string]string{}
	for _, l := range h.invocation.LogLabels {
		labels[l.Key] = l.Value
	}
	return labels
}

// ResolveArtifact resolves a file to a local path.
// If the file is inlined or a non-local-file URI ("", false) is returned.
func ResolveArtifact(f *buildpb.Artifact) (string, bool) {
	if f.Uri == "" {
		return "", false
	}
	prefix := "file:///"
	if !strings.HasPrefix(f.Uri, prefix) {
		return "", false
	}
	return f.Uri[len(prefix):], true
}

// LocalFileUri returns a file:/// local URI.
func LocalFileUri(p string) string {
	return fmt.Sprintf("file:///%s", p)
}
