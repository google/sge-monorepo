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

// Binary unity_builder implements the runner tool protocol in order to build Unity projects.

package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"sge-monorepo/build/cicd/sgeb/buildtool"
	"sge-monorepo/environment/envinstall"
	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/log/cloudlog"
	"sge-monorepo/libs/go/p4lib"

	"sge-monorepo/build/builders/unity_builder/protos/unity_builderpb"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
)

const (
	// The C# function to invoke within an Unity project in order to build.
	buildMethod   = "SGE.BuildUtils.BuildConfig.Build"
	targetWindows = "windows"
)

// Flags is the set of flags used by unity_builder.
type Flags struct {
	editor     string
	project    string
	outputName string
	profile    string
}

func initFlags() *Flags {
	f := &Flags{}
	flag.StringVar(&f.editor, "editor", "", "Path to the editor (before the Editor directory)")
	flag.StringVar(&f.project, "project", "", "Path to the project")
	flag.StringVar(&f.outputName, "output-name", "", "Name of the output executable. If none is set, filepath.Base(project) will be used")
	flag.StringVar(&f.profile, "profile", "", "Build profile to execute")
	return f
}

var flags = initFlags()

// Config holds all the necessary information to correctly build an Unity project.
type Config struct {
	editor     string
	project    string
	outputName string
	profile    string
	// credentials will be filled when running on a CI machine.
	credentials *unity_builderpb.Credentials
}

func NewConfig(helper buildtool.Helper, flags *Flags) (*Config, error) {
	// Find the editor path.
	if flags.editor == "" {
		flag.PrintDefaults()
		return nil, errors.New("empty editor flag")
	}
	editorPath, err := helper.ResolvePath(flags.editor)
	if err != nil {
		return nil, fmt.Errorf("could not resolve editor path %q: %v", flags.editor, err)
	}
	editor := filepath.Join(editorPath, "Editor", "Unity.exe")
	if !files.FileExists(editor) {
		return nil, fmt.Errorf("could not find Unity executable %q", editor)
	}
	// Project.
	// If the project is empty, we use the build unit as default.
	if flags.project == "" {
		flags.project = helper.Invocation().BuildUnitDir
	}
	project, err := helper.ResolvePath(flags.project)
	if err != nil {
		return nil, fmt.Errorf("could not resolve project path %q: %v", flags.project, err)
	}
	// Find the project manifest. All unity projects should have one.
	manifest := filepath.Join(project, "Packages", "manifest.json")
	if !files.FileExists(manifest) {
		return nil, fmt.Errorf("invalid project path %q: could not find %q", project, manifest)
	}
	// Find packages-lock.json.
	packageLock := filepath.Join(project, "Packages", "packages-lock.json")
	if !files.FileExists(packageLock) {
		return nil, fmt.Errorf("invalid project path %q: could not find %q", project, packageLock)
	}
	// OutputName.
	// If empty, we use the base dir of the project directory, as we assume that that is the
	// project name.
	if flags.outputName == "" {
		flags.outputName = filepath.Base(project)
	}
	outputName := flags.outputName
	// Profile.
	if flags.profile == "" {
		flag.PrintDefaults()
		return nil, errors.New("no profile flag provided")
	}
	profile := flags.profile
	// Credentials.
	credentials, err := ObtainCredentials()
	if err != nil {
		return nil, fmt.Errorf("could not obtain credentials: %v", err)
	}
	return &Config{
		editor:      editor,
		project:     project,
		outputName:  outputName,
		credentials: credentials,
		profile:     profile,
	}, nil
}

func labelsFromConfig(config *Config) map[string]string {
	labels := make(map[string]string)
	// Check if we're using a checked in editor, which are checked in with the following recipe:
	// //sge/engines/unity/editor/<VERSION>/Editor/Unity.exe
	if strings.HasPrefix(config.editor, filepath.Join("engines", "unity", "editor")) {
		labels["unity_editor"] = filepath.Base(filepath.Dir(filepath.Dir(config.editor)))
	} else {
		labels["unity_editor"] = config.editor
	}
	labels["project"] = config.project
	labels["profile"] = config.profile
	return labels
}

func run(helper buildtool.Helper, config *Config) (*buildpb.ArtifactSet, []*buildpb.Artifact, error) {
	var artifactSet *buildpb.ArtifactSet
	var logs []*buildpb.Artifact

	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		log.Infof("Building took: %s\n", elapsed)
	}()

	// All unity commands needs manifest.json to be writeable.
	p4 := p4lib.New()
	filesToOpen := []string{
		filepath.Join(config.project, "Packages", "manifest.json"),
		filepath.Join(config.project, "Packages", "packages-lock.json"),
	}
	if out, err := p4.Edit(filesToOpen, 0); err != nil {
		return artifactSet, logs, fmt.Errorf("could not p4 edit %s: %s", filesToOpen, out)
	}
	log.Infof("Opened %q for edit", filesToOpen)
	// We always revert the files if they haven't changed.
	defer func() {
		// -a means that only revert if unchanged.
		if out, err := p4.Revert(filesToOpen, "-a"); err != nil {
			log.Errorf("could not request %q: %s", filesToOpen, out)
		} else {
			log.Infof("Reverted %q (if unchanged).", filesToOpen)
		}
	}()
	// We prime the license (login to the license server) if applicable.
	// The caller function will always attempt to return the license.
	if out, err := PrimeLicense(config.editor, config.credentials); err != nil {
		fmt.Println(out)
		return artifactSet, logs, fmt.Errorf("could not prime unity license: %v", err)
	}
	// An automatic unity build requires 3 steps:
	// 1. Switch target.
	action := "switch-target"
	logs, err := config.RunUnityCommand(logs, helper, action)
	if err != nil {
		return artifactSet, logs, fmt.Errorf("could not switch target: %v", err)
	}
	// 2. Run the build.
	action = "build"
	logs, err = config.RunUnityCommand(logs, helper, action)
	if err != nil {
		return artifactSet, logs, fmt.Errorf("could not perform build: %v", err)
	}
	// Collect the artifacts.
	artifactSet, err = obtainArtifacts(helper)
	if err != nil {
		return artifactSet, logs, fmt.Errorf("could not obtain artifacts: %v", err)
	}
	// If we opened the file, we need to revert.
	return artifactSet, logs, nil
}

// RunUnityCommand runs a Unity command and appends the associated logs.
func (config *Config) RunUnityCommand(logs []*buildpb.Artifact, helper buildtool.Helper, action string) ([]*buildpb.Artifact, error) {
	// Create the summary logs.
	summary, err := ioutil.TempFile(helper.Invocation().LogsDir, fmt.Sprintf("%s-summary_*.log", action))
	if err != nil {
		return logs, fmt.Errorf("could not create summary logs file: %v", err)
	}
	summary.Close()

	// Create the full logs.
	done := make(chan struct{})
	full, err := ioutil.TempFile(helper.Invocation().LogsDir, fmt.Sprintf("%s-full_*.log", action))
	if err != nil {
		return logs, fmt.Errorf("could not create full logs file: %v", err)
	}
	full.Close()
	// We read from the full logs until we're done. This permits users running with logs displayed
	// to see realtime output.
	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func(done <-chan struct{}) {
		TailLogFile(full.Name(), done)
		wg.Done()
	}(done)

	// Execute the editor call.
	output := filepath.Join(helper.Invocation().BuildInvocation.OutputDir, config.outputName)
	args := []string{
		config.editor,
		"-quit",
		"-batchmode",
	}
	args = append(args, CredentialArgs(config.credentials)...)
	args = append(args, []string{
		"-logFile", full.Name(),
		"-projectPath", config.project,
		"-executeMethod", buildMethod,
		"--action", action,
		"--output", output,
		"--summary", summary.Name(),
		"--profile", config.profile,
		//
		"-EnableCacheServer",
		"-cacheServerEndpoint", "10.200.3.205:10080",
		"-cacheServerNamespacePrefix", filepath.Base(config.project),
		"-cacheServerEnableDownload", "true",
		"-cacheServerEnableUpload", "true",
	}...)
	cmd := exec.Command(args[0], args[1:]...)
	// Before we print, we replace secrets
	log.Infof("Running: %s", CleanSecrets(args, config.credentials))
	_, cmdErr := cmd.CombinedOutput()
	// Signal the log listener we're done and wait for it to exit.
	done <- struct{}{}
	wg.Wait()
	// Process the logs and try to attach them.
	// If we could not process the logs, we return the full logs yolo-style.
	if cmdErr != nil {
		if log, err := ProcessUnityLogs(helper.Invocation().LogsDir, summary, full); err != nil {
			logs = append(logs, &buildpb.Artifact{
				Tag: "logs",
				Uri: buildtool.LocalFileUri(full.Name()),
			})
		} else {
			logs = append(logs, log)
		}
	}
	return logs, cmdErr
}

func obtainArtifacts(helper buildtool.Helper) (*buildpb.ArtifactSet, error) {
	artifactSet := &buildpb.ArtifactSet{}
	outputDir := helper.Invocation().BuildInvocation.OutputDir
	outputBase := helper.Invocation().BuildInvocation.OutputBase
	err := filepath.Walk(outputDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(outputBase, path)
		if err != nil {
			return fmt.Errorf("could not obtain relative path: %v", err)
		}
		stablePath := strings.ReplaceAll(rel, `\`, `/`)
		artifactSet.Artifacts = append(artifactSet.Artifacts, &buildpb.Artifact{
			StablePath: stablePath,
			Uri:        buildtool.LocalFileUri(path),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not walk directory %s: %v", outputDir, err)
	}
	return artifactSet, nil
}

func internalMain() int {
	flag.Parse()
	helper := buildtool.MustLoad()

	config, configErr := NewConfig(helper, flags)
	// We get the potential logs with which we want cloud logger to be initialized with, which
	// depends on the config value.
	labels := func() map[string]string {
		if configErr != nil {
			return nil
		}
		return labelsFromConfig(config)
	}()
	log.AddSink(log.NewGlog())
	if envinstall.IsCloud() {
		cl, err := cloudlog.New("unity_builder", cloudlog.WithLabels(labels), cloudlog.WithLabels(helper.LogLabels()))
		if err != nil {
			fmt.Printf("Could not initialize cloud logger: %v", err)
			return 1
		}
		log.AddSink(cl)
	}
	defer log.Shutdown()
	// If config failed, we log it and exit.
	if configErr != nil {
		log.Errorf("could not load config: %v", configErr)
		return 1
	}
	// We always return the license after running.
	defer func() {
		if out, err := ReturnLicense(config.editor, config.credentials); err != nil {
			log.Error(out)
			log.Errorf("could not return license: %v", err)
		}
	}()
	artifactSet, logs, runErr := run(helper, config)
	helper.MustWriteBuildResult(&buildpb.BuildInvocationResult{
		Result: &buildpb.Result{
			Success: runErr == nil,
			Logs:    logs,
		},
		ArtifactSet: artifactSet,
	})
	if runErr != nil {
		log.Error(runErr)
		return 1
	}
	return 0
}

func main() {
	os.Exit(internalMain())
}
