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

// Binary docker_publisher pushes the output of a docker_push_config rule to a remote repository.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"sge-monorepo/build/cicd/sgeb/buildtool"

	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
	"sge-monorepo/build/publishers/docker_publisher/protos/dockerpushconfigpb"

	"github.com/golang/glog"
	"github.com/golang/protobuf/proto"
)

var flags = struct {
	dest string
}{}

func main() {
	flag.StringVar(&flags.dest, "dest", "", "registry path to push the image to")
	flag.Parse()
	glog.Infof("%v", os.Args)

	err := publish()

	if err != nil {
		glog.Errorf("%v", err)
	}
	glog.Flush()

	if err != nil {
		os.Exit(1)
	}
}

func publish() error {
	helper := buildtool.MustLoad()
	pc, err := loadManifest(helper)
	if err != nil {
		return err
	}
	pushedAnything, err := runPusher(pc)
	if err != nil {
		return fmt.Errorf("could not push image: %v", err)
	}
	if !pushedAnything {
		helper.MustWritePublishResult(&buildpb.PublishInvocationResult{})
		return nil
	}
	result := &buildpb.PublishResult{
		Name: flags.dest,
		Files: []*buildpb.PublishedFile{
			{Size: 0},
		},
	}
	helper.MustWritePublishResult(&buildpb.PublishInvocationResult{
		PublishResults: []*buildpb.PublishResult{result},
	})
	return nil
}

func loadManifest(helper buildtool.Helper) (*dockerpushconfigpb.DockerPushConfig, error) {
	// Locate push config textpb among inputs.
	// The other inputs are files pointed to by the push config.
	var p string
	for _, artifacts := range helper.Invocation().Inputs {
		for _, a := range artifacts.Artifacts {
			if strings.HasSuffix(a.Uri, ".pushconfig.textpb") {
				p, _ = buildtool.ResolveArtifact(a)
				break
			}
		}
	}
	if p == "" {
		return nil, errors.New("could not find .pushconfig.textpb in inputs. Point to a docker_push_config rule")
	}
	content, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("could not load push config file: %v", err)
	}
	pc := &dockerpushconfigpb.DockerPushConfig{}
	if err := proto.UnmarshalText(string(content), pc); err != nil {
		return nil, err
	}
	return pc, nil
}

func bazelExecRoot() (string, error) {
	cmd := exec.Command("bin/windows/bazel.exe", "info", "execution_root")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(buf.String()), nil
}

func runPusher(pc *dockerpushconfigpb.DockerPushConfig) (bool, error) {
	workspaceDir, err := os.Getwd()
	if err != nil {
		return false, err
	}
	args := []string{
		"-format=Docker",
		fmt.Sprintf("-config=%s", pc.Config),
		fmt.Sprintf("-manifest=%s", pc.Manifest),
		fmt.Sprintf("-dst=%s", flags.dest),
		fmt.Sprintf("-client-config-dir=%s", scrubClientConfigDir(workspaceDir, pc.ClientConfigDir)),
		"-skip-unchanged-digest",
	}
	for _, l := range pc.Layers {
		args = append(args, fmt.Sprintf("-layer=%s", l))
	}
	execRoot, err := bazelExecRoot()
	if err != nil {
		return false, err
	}
	cmd := exec.Command(pc.PushTool, args...)
	cmd.Dir = execRoot
	var logs bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &logs)
	cmd.Stderr = io.MultiWriter(os.Stderr, &logs)
	glog.Info(cmd.String())
	err = cmd.Run()
	if err != nil {
		return false, err
	}
	pushedNothing := strings.Contains(logs.String(), "Skipping push of unchanged digest")
	return !pushedNothing, nil
}

func scrubClientConfigDir(workspaceDir, c string) string {
	return strings.ReplaceAll(c, "%BUILD_WORKSPACE_DIRECTORY%", workspaceDir)
}
