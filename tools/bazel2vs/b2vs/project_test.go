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
	// "os"
	// "path/filepath"
	// "testing"

	// "sge-monorepo/build/cicd/monorepo"
	// "sge-monorepo/libs/go/sgetest"
	// "sge-monorepo/tools/bazel2vs/protos/msbuildpb"

	// "github.com/golang/protobuf/proto"
	// "github.com/google/go-cmp/cmp"
)

// TODO: The test had to be disabled because some of the golden files could not be open sourced.
//       Re-enable when this situation is fixed.
// func TestNewProjectInfo(t *testing.T) {
// 	monorepoRoot := `D:\P4\sge`
// 	mr := monorepo.New(monorepoRoot, make(map[string]monorepo.Path))
// 	testcases := []struct {
// 		desc          string
// 		ctx           *context
// 		protoFilename string
// 		want          *projectInfo
// 		wantErr       string
// 	}{
// 		{
// 			desc: "Success case",
// 			ctx: &context{
// 				execRoot:     "C:/bzl/random/execroot/sge",
// 				monorepo:     &mr,
// 				outputSlnDir: "D:\\P4\\sge\\msbuild",
// 			},
// 			protoFilename: ".\\testdata\\my_lib.msbuild",
// 			want: &projectInfo{
// 				targetInfo: &msbuildpb.TargetInfo{
// 					Cc: &msbuildpb.CcInfo{
// 						QuoteIncludeDirs: []string{
// 							".",
// 							"bazel-out/x64_windows-fastbuild/bin",
// 						},
// 					},
// 					Files: &msbuildpb.SrcFiles{
// 						Hdrs: []*msbuildpb.File{
// 							{
// 								Path:      "experimental/users/bazel-guy/hellobazel/my_lib.h",
// 								ShortPath: "experimental/users/bazel-guy/hellobazel/my_lib.h",
// 							},
// 						},
// 						Srcs: []*msbuildpb.File{
// 							{
// 								Path:      "experimental/users/bazel-guy/hellobazel/my_lib.cpp",
// 								ShortPath: "experimental/users/bazel-guy/hellobazel/my_lib.cpp",
// 							},
// 						},
// 					},
// 					Kind: "cc_library",
// 					Outputs: []*msbuildpb.File{
// 						{
// 							Path:      "bazel-out/x64_windows-fastbuild/bin/experimental/users/bazel-guy/hellobazel/my_lib.lib",
// 							RootPath:  "bazel-out/x64_windows-fastbuild/bin",
// 							ShortPath: "experimental/users/bazel-guy/hellobazel/my_lib.lib",
// 						},
// 					},
// 					Label: &msbuildpb.Label{
// 						Name:  "my_lib",
// 						Pkg:   "experimental/users/bazel-guy/hellobazel",
// 						Label: "//experimental/users/bazel-guy/hellobazel:my_lib",
// 					},
// 				},
// 				output: projectOutput{
// 					execPath:      "",
// 					execName:      "",
// 					workspaceRoot: "D:/P4/sge",
// 					projectName:   "my_lib",
// 					vcxprojDir:    `D:\P4\sge\msbuild\experimental\users\bazel-guy\hellobazel`,
//                     uuid:        "{15F9E62B-D1A0-5695-7168-938BA7DD01D7}",
// 				},
// 			},
// 		},
// 	}

// 	for _, tc := range testcases {
// 		targetInfo, err := readTargetInfo(tc.protoFilename)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		got, err := newProjectInfo(tc.ctx, targetInfo)
// 		if cmpErr := sgetest.CmpErr(err, tc.wantErr); cmpErr != nil {
// 			t.Errorf("newProjectInfo error: %v", cmpErr)
// 		}
// 		if err == nil {
// 			if !proto.Equal(got.targetInfo, tc.want.targetInfo) {
// 				t.Errorf("[%s] newProjectInfo targetInfo got (%s)\n, want(%s)", tc.desc, got.targetInfo, tc.want.targetInfo)
// 			}
// 			if diff := cmp.Diff(got.output, tc.want.output, cmp.AllowUnexported(projectOutput{})); diff != "" {
// 				t.Errorf("[%s] newProjectInfo output wrong have. Diff (+want, -got):\n%s", tc.desc, diff)
// 			}
// 		}
// 	}
// }

// func TestAppendDirs(t *testing.T) {
// 	testcases := []struct {
// 		desc string
// 		dir  string
// 		got  map[string]bool
// 		want map[string]bool
// 	}{
// 		{
// 			desc: "Succes case, append to a empty set",
// 			dir:  "src/some/path/to/some/dir",
// 			got:  make(map[string]bool),
// 			want: map[string]bool{
// 				`src\some\path\to\some\dir`: true,
// 				`src\some\path\to\some`:     true,
// 				`src\some\path\to`:          true,
// 				`src\some\path`:             true,
// 				`src\some`:                  true,
// 				`src`:                       true,
// 			},
// 		},
// 		{
// 			desc: "Succes case, append to a non empty set",
// 			dir:  "src/some/path/to/some/dir",
// 			got: map[string]bool{
// 				`src\some`: true,
// 				`src`:      true,
// 			},
// 			want: map[string]bool{
// 				`src\some\path\to\some\dir`: true,
// 				`src\some\path\to\some`:     true,
// 				`src\some\path\to`:          true,
// 				`src\some\path`:             true,
// 				`src\some`:                  true,
// 				`src`:                       true,
// 			},
// 		},
// 		{
// 			desc: "Empty dir",
// 			dir:  "",
// 			got:  make(map[string]bool),
// 			want: map[string]bool{},
// 		},
// 	}

// 	for _, tc := range testcases {
// 		appendDirs(tc.got, tc.dir)
// 		if diff := cmp.Diff(tc.got, tc.want); diff != "" {
// 			t.Errorf("[%s] appendDirs wrong have. Diff (+want, -got):\n%s", tc.desc, diff)
// 		}
// 	}
// }

// func TestResolveFilePath(t *testing.T) {
// 	monorepoRoot := `D:\P4\sge`
// 	execroot := "C:/bzl/random/execroot/sge"
// 	repoMap := make(map[string]monorepo.Path)
// 	repoMap["glm"] = "third_party/glm"
// 	mr := monorepo.New(monorepoRoot, repoMap)

// 	testcases := []struct {
// 		desc string
// 		ctx  *context
// 		info *projectInfo
// 		file *msbuildpb.File
// 		want string
// 	}{
// 		{
// 			desc: "generated file",
// 			ctx: &context{
// 				execRoot: execroot,
// 				monorepo: &mr,
// 			},
// 			info: &projectInfo{
// 				targetInfo: &msbuildpb.TargetInfo{
// 					Label: &msbuildpb.Label{
// 						WorkspaceName: "sge",
// 					},
// 				},
// 			},
// 			file: &msbuildpb.File{
// 				Path:               "bazel-out/x64_windows-fastbuild/bin/experimental/users/bazel-guy/hellobazel/app_a.exe",
// 				RootPath:           "bazel-out/x64_windows-fastbuild/bin",
// 				ShortPath:          "experimental/users/bazel-guy/hellobazel/app_a.exe",
// 				OwnerWorkspaceName: "",
// 			},
// 			want: `C:\bzl\random\execroot\sge\bazel-out\$(BazelCfgDirname)\bin\experimental\users\bazel-guy\hellobazel\app_a.exe`,
// 		},
// 		{
// 			desc: "external file",
// 			ctx: &context{
// 				execRoot: execroot,
// 				monorepo: &mr,
// 			},
// 			info: &projectInfo{
// 				targetInfo: &msbuildpb.TargetInfo{
// 					Label: &msbuildpb.Label{
// 						WorkspaceName: "sge",
// 					},
// 				},
// 			},
// 			file: &msbuildpb.File{
// 				Path:               "external/glm/glm/gtx/dual_quaternion.hpp",
// 				RootPath:           "",
// 				ShortPath:          "../glm/glm/gtx/dual_quaternion.hpp",
// 				OwnerWorkspaceName: "glm",
// 			},
// 			want: `D:\P4\sge\third_party\glm\glm\gtx\dual_quaternion.hpp`,
// 		},
// 		{
// 			desc: "Local file",
// 			ctx: &context{
// 				execRoot: execroot,
// 				monorepo: &mr,
// 			},
// 			info: &projectInfo{
// 				targetInfo: &msbuildpb.TargetInfo{
// 					Label: &msbuildpb.Label{
// 						WorkspaceName: "sge",
// 					},
// 				},
// 			},
// 			file: &msbuildpb.File{
// 				Path:               "experimental/users/graphics-guy/hellovulkan/src/BitmapImage.h",
// 				RootPath:           "",
// 				ShortPath:          "experimental/users/graphics-guy/hellovulkan/src/BitmapImage.h",
// 				OwnerWorkspaceName: "",
// 			},
// 			want: `D:\P4\sge\experimental\users\graphics-guy\hellovulkan\src\BitmapImage.h`,
// 		},
// 	}

// 	for _, tc := range testcases {
// 		got := resolveFilePath(tc.ctx, tc.info, tc.file)
// 		if diff := cmp.Diff(got, tc.want); diff != "" {
// 			t.Errorf("[%s] resolveFilePath wrong have. Diff (+want, -got):\n%s", tc.desc, diff)
// 		}
// 	}
// }

// func TestGenerateProjects(t *testing.T) {
// 	tempDir, err := ioutil.TempDir("", "test")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer os.RemoveAll(tempDir)

// 	mr := monorepo.New(`D:\P4\sge`, make(map[string]monorepo.Path))
// 	ctx := &context{
// 		outputSlnDir:   tempDir,
// 		solutionName:   "test",
// 		buildCfg: []buildConfig{
// 			{"FastBuild", "fastbuild"},
// 			{"Debug", "dbg"},
// 			{"Release", "opt"},
// 		},
// 		platforms: []platform{
// 			{"x64", "x64_windows", "windows", "WindowsLocalDebugger"},
// 			{"linux", "x64_linux", "ubuntu", "WindowsLocalDebugger"},
// 		},
// 		execRoot: "C:/bzl/random/execroot/sge",
// 		monorepo: &mr,
// 		bazel:    "C:\\bin\\windows\\bazel.exe",
// 	}

// 	testcases := []struct {
// 		desc                   string
// 		msbuildFile            string
// 		wantVcxprojFile        string
// 		wantVcxprojFiltersFile string
// 		wantVcxprojUserFile    string
// 		wantErr                string
// 	}{
// 		{
// 			desc:                   "Cc project",
// 			msbuildFile:            ".\\testdata\\hellovulkan.msbuild",
// 			wantVcxprojFile:        ".\\testdata\\hellovulkan.vcxproj",
// 			wantVcxprojFiltersFile: ".\\testdata\\hellovulkan.vcxproj.filters",
// 			wantVcxprojUserFile:    ".\\testdata\\hellovulkan.vcxproj.user",
// 		},
// 		{
// 			desc:                   "Rust project",
// 			msbuildFile:            ".\\testdata\\shaderpacker_rust.msbuild",
// 			wantVcxprojFile:        ".\\testdata\\shaderpacker_rust.vcxproj",
// 			wantVcxprojFiltersFile: ".\\testdata\\shaderpacker_rust.vcxproj.filters",
// 			wantVcxprojUserFile:    ".\\testdata\\shaderpacker_rust.vcxproj.user",
// 		},
// 	}

// 	for _, tc := range testcases {
// 		msbuildFiles := []string{tc.msbuildFile}
// 		_, err = generateProjects(ctx, msbuildFiles)
// 		if err := sgetest.CmpErr(err, tc.wantErr); err != nil {
// 			t.Errorf("[%s] generateProjects error: %v", tc.desc, err)
// 		}
// 		ti, err := readTargetInfo(tc.msbuildFile)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		{
// 			gotVcxprojFile := filepath.Join(tempDir, ti.Label.Pkg, ti.Label.Name+".vcxproj")
// 			gotVcxproj, err := ioutil.ReadFile(gotVcxprojFile)
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			wantVcxproj, err := ioutil.ReadFile(tc.wantVcxprojFile)
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			if diff := cmp.Diff(gotVcxproj, wantVcxproj); diff != "" {
// 				t.Errorf("[%s] generateProjects .vcxproj wrong have. Diff (+want, -got):\n%s", tc.desc, diff)
// 			}
// 		}
// 		{
// 			gotVcxprojFiltersFile := filepath.Join(tempDir, ti.Label.Pkg, ti.Label.Name+".vcxproj.filters")
// 			gotVcxprojFilters, err := ioutil.ReadFile(gotVcxprojFiltersFile)
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			wantVcxprojFilters, err := ioutil.ReadFile(tc.wantVcxprojFiltersFile)
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			if diff := cmp.Diff(gotVcxprojFilters, wantVcxprojFilters); diff != "" {
// 				t.Errorf("[%s] generateProjects .vcxproj.filters wrong have. Diff (+want, -got):\n%s", tc.desc, diff)
// 			}
// 		}
// 		{
// 			gotVcxprojUserFile := filepath.Join(tempDir, ti.Label.Pkg, ti.Label.Name+".vcxproj.user")
// 			gotVcxprojUser, err := ioutil.ReadFile(gotVcxprojUserFile)
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			wantVcxprojUser, err := ioutil.ReadFile(tc.wantVcxprojUserFile)
// 			if err != nil {
// 				t.Fatal(err)
// 			}
// 			if diff := cmp.Diff(gotVcxprojUser, wantVcxprojUser); diff != "" {
// 				t.Errorf("[%s] generateProjects .vcxproj.user wrong have. Diff (+want, -got):\n%s", tc.desc, diff)
// 			}
// 		}
// 	}
// }
