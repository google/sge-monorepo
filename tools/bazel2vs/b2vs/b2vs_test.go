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

package b2vs

import (
	// "io/ioutil"
	// "path/filepath"
	// "testing"

	// "sge-monorepo/build/cicd/bep"
	// "sge-monorepo/libs/go/sgetest"

	// "github.com/google/go-cmp/cmp"
)


// TODO: Commented out because golden file could not be open sourced.
//       Re-enable this once a suitable bep file has been generated.
// func TestGetBuildResults(t *testing.T) {
// 	testCases := []struct {
// 		desc    string
// 		bepFile string
// 		want    []string
// 		wantErr string
// 	}{
// 		{
// 			desc:    "Success case",
// 			bepFile: "hellobep.out",
// 			want: []string{
// 				"C:/bzl/jlidm6zv/execroot/sge/bazel-out/x64_windows-fastbuild/bin/experimental/users/graphics-guy/hellovulkan/hellovulkan.msbuild",
// 				"C:/bzl/jlidm6zv/execroot/sge/bazel-out/x64_windows-fastbuild/bin/external/flatbuffers/runtime_cc.msbuild",
// 				"C:/bzl/jlidm6zv/execroot/sge/bazel-out/x64_windows-fastbuild/bin/external/glfw/windows/glfwlib.msbuild",
// 				"C:/bzl/jlidm6zv/execroot/sge/bazel-out/x64_windows-fastbuild/bin/external/glfw/windows/glfwlib_ext.msbuild",
// 				"C:/bzl/jlidm6zv/execroot/sge/bazel-out/x64_windows-fastbuild/bin/external/glm/glmheaders.msbuild",
// 				"C:/bzl/jlidm6zv/execroot/sge/bazel-out/x64_windows-fastbuild/bin/external/lc/lcheaders.msbuild",
// 				"C:/bzl/jlidm6zv/execroot/sge/bazel-out/x64_windows-fastbuild/bin/external/vulkan/vulkan.msbuild",
// 				"C:/bzl/jlidm6zv/execroot/sge/bazel-out/x64_windows-fastbuild/bin/external/vulkan/vulkan_headers.msbuild",
// 				"C:/bzl/jlidm6zv/execroot/sge/bazel-out/x64_windows-fastbuild/bin/external/vulkan/vulkan_import.msbuild",
// 				"C:/bzl/jlidm6zv/execroot/sge/bazel-out/x64_windows-fastbuild/bin/tools/shaderpacker_rust/shader_headers.msbuild",
// 			},
// 		},
// 		{
// 			desc:    "No msbuild_outputs",
// 			bepFile: "bazel2vsbep.out",
// 			want:    nil,
// 			wantErr: "",
// 		},
// 	}

// 	for _, tc := range testCases {
// 		content, err := ioutil.ReadFile(filepath.Join(".\\testdata", tc.bepFile))
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		bepStream, err := bep.Parse(content)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		got, err := buildInvocationResult(bepStream)
// 		if err := sgetest.CmpErr(err, tc.wantErr); err != nil {
// 			t.Errorf("[%s]: %v", tc.desc, err)
// 		}
// 		if diff := cmp.Diff(got, tc.want); diff != "" {
// 			t.Errorf("[%s] TestGetBuildResults wrong have. Diff (-want, +got):\n%s", tc.desc, diff)
// 		}
// 	}
// }
