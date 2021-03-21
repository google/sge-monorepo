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

// Binary test_publisher is a publisher that simply prints back the inputs, basically a no-op.
// Useful for testing integrations.

package main

import (
	"flag"
	"fmt"
	"os"

	"sge-monorepo/build/cicd/sgeb/buildtool"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
)

func internalMain() int {
	flag.Parse()
	helper := buildtool.MustLoad()

	var publishedFiles []*buildpb.PublishedFile
	for _, input := range helper.Invocation().Inputs {
		for _, artifact := range input.Artifacts {
			path, isFile := buildtool.ResolveArtifact(artifact)
			if !isFile {
				continue
			}
			info, err := os.Stat(path)
			if err != nil {
				fmt.Printf("could not stat %q: %v\n", path, err)
				return 1
			}
			publishedFiles = append(publishedFiles, &buildpb.PublishedFile{
				Size: info.Size(),
			})
		}
	}
	helper.MustWritePublishResult(&buildpb.PublishInvocationResult{
		PublishResults: []*buildpb.PublishResult{
			&buildpb.PublishResult{
				Files: publishedFiles,
			},
		},
	})
	return 0
}

func main() {
	os.Exit(internalMain())
}
