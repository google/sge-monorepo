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
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

// slnProject generates the project Id in a .sln file.
func slnProject(info *projectInfo) string {
	return fmt.Sprintf(
		`Project("%s") = "%s", "%s", "%s"\nEndProject`,
		projectTypeGUID,
		info.output.projectName,
		filepath.Join(projOutputDir(info.targetInfo.Label), info.targetInfo.Label.Name+".vcxproj"),
		info.output.uuid,
	)
}

// slnProjects generates project Ids for each project of a .sln file.
func slnProjects(infos []*projectInfo) string {
	var slnProjectIds []string
	for _, info := range infos {
		slnProjectIds = append(slnProjectIds, slnProject(info))
	}
	return strings.Join(slnProjectIds, "\r\n")
}

// slnCfgs generates the solution configuration content.
func slnCfgs(ctx *context) string {
	var lines []string
	for _, buildCfg := range ctx.buildCfg {
		for _, platform := range ctx.platforms {
			lines = append(lines, fmt.Sprintf("%[1]s|%[2]s = %[1]s|%[2]s", buildCfg.msbuildName, platform.msbuildName))
		}
	}
	return strings.Join(lines, "\r\n\t\t")
}

// slnProjectCfg generates the solution project configuration.
func slnProjectCfg(ctx *context, infos []*projectInfo) string {
	var lines []string
	for _, buildCfg := range ctx.buildCfg {
		for _, platform := range ctx.platforms {
			for _, info := range infos {
				lines = append(lines, fmt.Sprintf("%[1]s.%[2]s|%[3]s.ActiveCfg = %[2]s|%[3]s", info.output.uuid, buildCfg.msbuildName, platform.msbuildName))
				lines = append(lines, fmt.Sprintf("%[1]s.%[2]s|%[3]s.Build.0 = %[2]s|%[3]s", info.output.uuid, buildCfg.msbuildName, platform.msbuildName))
			}
		}
	}
	return strings.Join(lines, "\r\n\t\t")
}

// generateSolutionFile mofifies a template to generate the content of the .sln file
func generateSolutionFile(template string, slnName string, ctx *context, infos []*projectInfo) (string, error) {
	// Use the base of the path to get an UUID that depends only on the name of solution files.
	uuid, err := generateUUIDFromString(filepath.Base(slnName))
	if err != nil {
		return "", err
	}
	r := strings.NewReplacer(
		"{projects}", slnProjects(infos),
		"{cfgs}", slnCfgs(ctx),
		"{project_cfgs}", slnProjectCfg(ctx, infos),
		"{guid}", uuid,
	)
	return r.Replace(template), nil
}

// generateSolution writes to disk a .sln file. a template .sln is modify accordingly.
func generateSolution(ctx *context, infos []*projectInfo) (string, error) {
	slnFilename := filepath.Join(ctx.outputSlnDir, ctx.solutionName+".sln")
	slnContent, err := generateSolutionFile(readTemplate("templates/solution.sln"), slnFilename, ctx, infos)
	if err != nil {
		return "", err
	}
	err = ioutil.WriteFile(slnFilename, []byte(slnContent), os.ModePerm)
	return slnFilename, err
}
