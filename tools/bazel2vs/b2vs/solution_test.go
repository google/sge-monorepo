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
	// "testing"

	// "sge-monorepo/build/cicd/monorepo"
	// "sge-monorepo/libs/go/sgetest"

	// "github.com/google/go-cmp/cmp"
)

// TODO: Disabling this tests because some of the golden files could not be open sourced.
//       Re-enable this when that situation is fixed.
// func TestGenerateSolution(t *testing.T) {
// 	tempDir, err := ioutil.TempDir("", "test")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	defer os.RemoveAll(tempDir)

// 	mr := monorepo.New(`D:\P4\sge`, make(map[string]monorepo.Path))
// 	ctx := &context{
// 		outputSlnDir: tempDir,
// 		solutionName: "test",
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
// 	}
// 	msbuildFilenames := []string{
// 		".\\testdata\\app_a.msbuild",
// 		".\\testdata\\my_lib.msbuild",
// 	}
// 	var infos []*projectInfo
// 	for _, msbuild := range msbuildFilenames {
// 		ti, err := readTargetInfo(msbuild)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		info, err := newProjectInfo(ctx, ti)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		// Create a folder for the project files
// 		err = os.MkdirAll(info.output.vcxprojDir, os.ModePerm)
// 		if err != nil {
// 			t.Fatal(err)
// 		}
// 		infos = append(infos, info)
// 	}
// 	wantSolution, err := ioutil.ReadFile(".\\testdata\\test.sln")
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	wantErr := ""

// 	gotName, err := generateSolution(ctx, infos)
// 	if err = sgetest.CmpErr(err, wantErr); err != nil {
// 		t.Errorf("generateSolution error: %v", err)
// 	}
// 	gotSolution, err := ioutil.ReadFile(gotName)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// 	if diff := cmp.Diff(gotSolution, wantSolution); diff != "" {
// 		t.Errorf("generateSolution .sln wrong have. Diff (+want, -got):\n%s", diff)
// 		t.Errorf("\ngot:\n%s\nwant:\n%s\n", gotSolution, wantSolution)
// 	}
// }
