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

// Binary unreal_builder implements the runner tool for building Unreal projects.

package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-version"

	"sge-monorepo/build/cicd/sgeb/buildtool"
	"sge-monorepo/environment/envinstall"
	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/log/cloudlog"
	"sge-monorepo/libs/go/sgeflag"

	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
)

type ubFlags struct {
	action    string
	uproject  string
	editor    string
	platform  string
	config    string
	maps      sgeflag.StringList
}

// executeAction runs the action and returns the list of generated files and logs.
func executeAction(helper buildtool.Helper, c *config) (*buildpb.ArtifactSet, []*buildpb.Artifact, error) {
	switch c.action {
	case "build_editor":
		return executeBuildEditor(helper, c)
	case "build_code":
		return executeBuildCode(helper, c)
	case "package":
		return executePackage(helper, c)
	}
	return nil, nil, fmt.Errorf("action %q not found", c.action)
}

// executeBuildEditor builds the editor for Windows.
func executeBuildEditor(helper buildtool.Helper, c *config) (*buildpb.ArtifactSet, []*buildpb.Artifact, error) {
	args := []string{
		fmt.Sprintf("-Target=%sEditor Win64 Development", c.projectName),
		"-Target=ShaderCompileWorker Win64 Development",
		"-Target=UnrealLightmass Win64 Development",
		"-Target=CrashReportClient Win64 Shipping",
		"-Target=CrashReportClientEditor Win64 Shipping",
	}
	logs, err := executeBuild(helper, c, args)
	if err != nil {
		return nil, logs, err
	}
	var outputs []*buildpb.Artifact
	for tag, exe := range map[string]string{"editor": "UE4Editor.exe", "editor-cmd": "UE4Editor-Cmd.exe"} {
		stablePath := filepath.Join(c.editor, "Engine", "Binaries", "Win64", exe)
		absPath, err := filepath.Abs(stablePath)
		if err != nil {
			return nil, logs, fmt.Errorf("could not resolve output: %v", err)
		}
		if !files.FileExists(absPath) {
			return nil, logs, fmt.Errorf("expected output file %s could not be found", absPath)
		}
		outputs = append(outputs, &buildpb.Artifact{
			Tag:        tag,
			StablePath: stablePath,
			Uri:        buildtool.LocalFileUri(absPath),
		})
	}
	artifacts := &buildpb.ArtifactSet{
		Tag:       "UE4Editor",
		Artifacts: outputs,
	}
	return artifacts, logs, nil
}

func executeBuildCode(helper buildtool.Helper, c *config) (*buildpb.ArtifactSet, []*buildpb.Artifact, error) {
	args := []string{
		fmt.Sprintf("-Target=%s %s %s", c.projectName, c.platform, c.config),
	}
	logs, err := executeBuild(helper, c, args)
	return nil, logs, err
}

func executeBuild(helper buildtool.Helper, c *config, args []string) ([]*buildpb.Artifact, error) {
	// Unreal Editor builds need to ensure UnrealBuildTool is built.
	var logs []*buildpb.Artifact
	toolingLogs, err := buildRequiredTooling(helper, c)
	if err != nil {
		logs = append(logs, toolingLogs...)
		return logs, fmt.Errorf("could not build UnrealBuildTool: %w", err)
	}
	buildBat := filepath.Join("Engine", "Build", "BatchFiles", "Build.bat")
	// Create the log file for stdout.
	stdoutLog, err := ioutil.TempFile(helper.Invocation().LogsDir, "build_editor_stdout_*.log")
	if err != nil {
		return logs, fmt.Errorf("could not create stdout logs file: %w", err)
	}
	defer stdoutLog.Close()
	logs = append(logs, &buildpb.Artifact{
		Tag: "build_editor_stdout",
		Uri: buildtool.LocalFileUri(stdoutLog.Name()),
	})
	// Create the log file for full logs.
	fullLog, err := ioutil.TempFile(helper.Invocation().LogsDir, "build_editor_full_*.log")
	if err != nil {
		return logs, fmt.Errorf("could not create full logs file: %w", err)
	}
	// We close it because unreal is going to open this one.
	_ = fullLog.Close()
	logs = append(logs, &buildpb.Artifact{
		Tag: "build_editor_full",
		Uri: buildtool.LocalFileUri(fullLog.Name()),
	})
	// Run the build.
	cmdArgs := []string{
		fmt.Sprintf("-Project=%s", c.uproject),
		"-WaitMutex",
		fmt.Sprintf("-log=%s", fullLog.Name()),
	}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(buildBat, cmdArgs...)
	cmd.Dir = c.editor
	// Log to file and log.
	cmd.Stdout = io.MultiWriter(stdoutLog, log.NewInfoLogger(log.Get()))
	cmd.Stderr = io.MultiWriter(stdoutLog, log.NewInfoLogger(log.Get()))
	log.Infof("Running: %s", cmd.Args)
	if err := cmd.Run(); err != nil {
		return logs, fmt.Errorf("could not build editor: %w", err)
	}
	return logs, nil
}

// buildRequiredTooling builds all the required tooling for building.
func buildRequiredTooling(helper buildtool.Helper, c *config) ([]*buildpb.Artifact, error) {
	var logs []*buildpb.Artifact
	cprojPath := filepath.Join("Engine", "Source", "Programs", "UnrealBuildTool", "UnrealBuildTool.csproj")
	ubtLog, err := buildRequiredTool(helper, c, "ubt", cprojPath)
	logs = append(logs, ubtLog)
	if err != nil {
		return logs, err
	}
	cprojPath = filepath.Join("Engine", "Source", "Programs", "AutomationTool", "AutomationTool.csproj")
	atLog, err := buildRequiredTool(helper, c, "automation_tool", cprojPath)
	logs = append(logs, atLog)
	if err != nil {
		return logs, err
	}
	cprojPath = filepath.Join("Engine", "Source", "Programs", "AutomationToolLauncher", "AutomationToolLauncher.csproj")
	atlLog, err := buildRequiredTool(helper, c, "automation_tool_launcher", cprojPath)
	logs = append(logs, atlLog)
	if err != nil {
		return logs, err
	}
	return logs, nil
}

func buildRequiredTool(helper buildtool.Helper, c *config, name, path string) (*buildpb.Artifact, error) {
	logFile, err := ioutil.TempFile(helper.Invocation().LogsDir, fmt.Sprintf("build_%s_*.log", name))
	if err != nil {
		return nil, fmt.Errorf("could not create %s log file: %w", name, err)
	}
	defer logFile.Close()
	logs := &buildpb.Artifact{
		Tag: fmt.Sprintf("%s_log", name),
		Uri: buildtool.LocalFileUri(logFile.Name()),
	}
	batchFilesDir := filepath.Join("Engine", "Build", "BatchFiles")
	// This script will add the platform specific bits to the make file.
	// This is only necessary for versions prior to 4.25.
	if c.editorVersion.LessThan(version.Must(version.NewVersion("4.25"))) {
		findDepsBat := filepath.Join(batchFilesDir, "FindPlatformExtensionSources.bat")
		cmd := exec.Command(findDepsBat)
		cmd.Dir = c.editor
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("could not run %q: %w", cmd.Args, err)
		}
	}
	msbuild := filepath.Join(batchFilesDir, "MSBuild.bat")
	cmd := exec.Command(msbuild,
		path,
		"/target:build",
		"/property:Configuration=Development",
		"/verbosity:minimal",
	)
	cmd.Dir = c.editor
	// Log to file and to logs.
	cmd.Stdout = io.MultiWriter(logFile, log.NewInfoLogger(log.Get()))
	cmd.Stderr = io.MultiWriter(logFile, log.NewInfoLogger(log.Get()))
	if err := cmd.Run(); err != nil {
		return logs, fmt.Errorf("could not build %s (args %s): %w", name, cmd.Args, err)
	}
	return logs, nil
}

func zipFolder(folderToZip string, zipFilename string) error {
	// filepath.Clean (among other things) removes trailing slashes, filepath.FromSlash makes slashes uniform
	folderToZip = filepath.FromSlash(filepath.Clean(folderToZip))
	if info, err := os.Stat(folderToZip); os.IsNotExist(err) || !info.IsDir() {
		return err
	}

	file, err := os.Create(zipFilename)
	if err != nil {
		return err
	}
	w := zip.NewWriter(file)

	walker := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// The relativePath for files walked should be relative to the folderToZip with no leading slash.
		// We can generate malformed zip files otherwise.
		relativePath := strings.TrimPrefix(path, folderToZip+string(os.PathSeparator))
		f, err := w.Create(relativePath)
		if err != nil {
			return err
		}

		_, err = io.Copy(f, file)
		if err != nil {
			return err
		}

		return nil
	}

	err = filepath.Walk(folderToZip, walker)
	zipCloseErr := w.Close()
	fileCloseErr := file.Close()
	if err != nil || zipCloseErr != nil || fileCloseErr != nil {
		os.Remove(zipFilename)
		if err != nil {
			return err
		} else if zipCloseErr != nil {
			return zipCloseErr
		} else if fileCloseErr != nil {
			return fileCloseErr
		}
	}
	return nil
}

func executePackage(helper buildtool.Helper, c *config) (*buildpb.ArtifactSet, []*buildpb.Artifact, error) {
	// Unreal Editor builds need to ensure UnrealBuildTool is built.
	var logs []*buildpb.Artifact
	toolingLogs, err := buildRequiredTooling(helper, c)
	if err != nil {
		logs = append(logs, toolingLogs...)
		return nil, logs, fmt.Errorf("could not build UnrealBuildTool: %w", err)
	}
	tempDir, err := ioutil.TempDir("", "uebuild*")
	if err != nil {
		return nil, nil, err
	}
	defer os.RemoveAll(tempDir)
	// Create the log file for stdout.
	stdoutLog, err := ioutil.TempFile(helper.Invocation().LogsDir, "build_editor_stdout_*.log")
	if err != nil {
		return nil, logs, fmt.Errorf("could not create stdout logs file: %w", err)
	}
	defer stdoutLog.Close()
	logs = append(logs, &buildpb.Artifact{
		Tag: "build_editor_stdout",
		Uri: buildtool.LocalFileUri(stdoutLog.Name()),
	})
	args := []string{
		"BuildCookRun",
		fmt.Sprintf("-project=%s", c.uproject),
		"-noP4",
		"-nocompileeditor",
		"-utf8output",
		"-build",
		"-cook",
		"-stage",
		"-compile",
		"-pak",
		"-package",
		"-compressed",
		fmt.Sprintf("-targetplatform=%s", c.platform),
		fmt.Sprintf("-clientConfig=%s", c.config),
		fmt.Sprintf("-serverConfig=%s", c.config),
		fmt.Sprintf("-stagingdirectory=%s", tempDir),
		fmt.Sprintf("-map=%s", strings.Join(c.maps, "+")),
	}
	tool := "Engine/Binaries/DotNET/AutomationToolLauncher.exe"
	cmd := exec.Command(tool, args...)
	cmd.Dir = c.editor
	cmd.Stdout = io.MultiWriter(stdoutLog, log.NewInfoLogger(log.Get()))
	cmd.Stderr = io.MultiWriter(stdoutLog, log.NewInfoLogger(log.Get()))
	log.Infof("Running: %s", cmd.Args)
	if err := cmd.Run(); err != nil {
		return nil, logs, fmt.Errorf("could not package Unreal: %w", err)
	}
	if c.platform == "Win64" {
		relZipPath := fmt.Sprintf("%s-%s-%s.zip", c.projectName, c.platform, c.config)
		zipPath, stablePath := helper.DeclareOutput(relZipPath)
		folderToZip := filepath.Join(tempDir, "WindowsNoEditor")
		if err = zipFolder(folderToZip, zipPath); err != nil {
			return nil, logs, fmt.Errorf("could not zip WindowsNoEditor folder: %w", err)
		}
		absPath, err := filepath.Abs(zipPath)
		if err != nil {
			return nil, logs, fmt.Errorf("could not resolve output zip: %v", err)
		}
		if !files.FileExists(absPath) {
			return nil, logs, fmt.Errorf("expected output file %s could not be found", absPath)
		}
		outputs := &buildpb.ArtifactSet{
			Artifacts: []*buildpb.Artifact{
				{
					StablePath: stablePath,
					Uri:        buildtool.LocalFileUri(absPath),
				},
			},
		}
		return outputs, logs, nil
	}
	return nil, logs, fmt.Errorf("failed to package unhandled platform %s", c.platform)
}

// Main --------------------------------------------------------------------------------------------

func internalMain() error {
	flags := ubFlags{}
	flag.StringVar(&flags.action, "action", "", "Action is what unreal_builder needs to do")
	flag.StringVar(&flags.uproject, "uproject", "", "Path to the .uproject file")
	flag.StringVar(&flags.editor, "editor", "", "Path to the base directory editor")
	flag.StringVar(&flags.platform, "platform", "", "Platform to build")
	flag.StringVar(&flags.config, "config", "", "Configuration to build")
	flag.Var(&flags.maps, "map", "Maps to include")
	flag.Parse()
	helper := buildtool.MustLoad()
	log.AddSink(log.NewGlog())
	var cl cloudlog.CloudLogger
	if envinstall.IsCloud() {
		var err error
		cl, err = cloudlog.New("unreal_builder", cloudlog.WithLabels(helper.LogLabels()))
		if err != nil {
			return fmt.Errorf("could not obtain a cloud logger: %v", err)
		}
		log.AddSink(cl)
	}
	defer log.Shutdown()

	c, err := newConfig(helper, flags)
	if err != nil {
		return err
	}
	if cl != nil {
		// Add the labels so that the cloud logging can be correctly searched.
		cl.AddLabels(map[string]string{
			// TODO: We should have a better proxy to determine the project name.
			"uproject": c.uproject,
			"editor":   c.editor,
			"action":   c.action,
		})
	}
	// Execute the action and collect the outputs.
	artifactSet, logs, err := executeAction(helper, c)
	helper.MustWriteBuildResult(&buildpb.BuildInvocationResult{
		Result: &buildpb.Result{
			Success: err == nil,
			Logs:    logs,
		},
		ArtifactSet: artifactSet,
	})
	if err != nil {
		return fmt.Errorf("could not execute action: %v", err)
	}
	return nil
}

func main() {
	if err := internalMain(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
