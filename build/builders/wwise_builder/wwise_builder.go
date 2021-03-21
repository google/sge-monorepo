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

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"sge-monorepo/build/builders/wwise_builder/protos/soundbankpb"
	"sge-monorepo/build/cicd/sgeb/buildtool"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/log"

	"github.com/golang/protobuf/proto"
)

type wbFlags struct {
	wproj       string
	wwiseCliExe string
}

// resolvePath returns a valid monorepo-relative path
func resolvePath(helper buildtool.Helper, path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("must not be empty")
	}
	monorepoPath, err := helper.ResolvePath(path)
	if err != nil {
		return "", fmt.Errorf("could not resolve path %s: %v", path, err)
	}
	if !files.FileExists(monorepoPath) {
		return "", fmt.Errorf("%s doesn't exist", monorepoPath)
	}
	return monorepoPath, nil
}

// findFileByTag returns a valid path to the file with a certain input artifact tag
func findFileByTag(tag string, fileMap map[string]string) (string, error) {
	file, ok := fileMap[tag]
	if !ok {
		return "", fmt.Errorf("tag %s is not found", tag)
	}
	if !files.FileExists(file) {
		return "", fmt.Errorf("%s doesn't exist", file)
	}
	return file, nil
}

// appendOutputFileToArtifacts appends all files in path to outputs
func appendOutputFileToArtifacts(outputs []*buildpb.Artifact, outputBase, path string) ([]*buildpb.Artifact, error) {
	stat, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get expected output %s: %v", path, err)
	}
	if stat.IsDir() {
		subpaths, err := ioutil.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read dir %s：%v", path, err)
		}
		for _, subpath := range subpaths {
			if outputs, err = appendOutputFileToArtifacts(outputs, outputBase, filepath.Join(path, subpath.Name())); err != nil {
				return nil, err
			}
		}
		return outputs, nil
	}
	// if path is a file, add it to artifact outputs
	stablePath, err := filepath.Rel(outputBase, path)
	if err != nil {
		return nil, err
	}
	outputs = append(outputs, &buildpb.Artifact{
		StablePath: stablePath,
		Uri:        buildtool.LocalFileUri(path),
	})
	return outputs, nil
}

func buildSoundbanks(flags wbFlags) error {
	helper := buildtool.MustLoad()
	wproj, err := resolvePath(helper, flags.wproj)
	if err != nil {
		return fmt.Errorf("-wproj input is not valid: %v", err)
	}
	wwiseCliExe, err := resolvePath(helper, flags.wwiseCliExe)
	if err != nil {
		return fmt.Errorf("-wwise_exe input is not valid: %v", err)
	}
	// Get the input artifact paths
	inputArtifactSets := helper.Invocation().GetInputs()
	inputFileMap := map[string]string{}
	for _, inputArtifactSet := range inputArtifactSets {
		if inputArtifactSet.GetTag() != "SoundbanksList" {
			continue
		}
		inputArtifacts := inputArtifactSet.GetArtifacts()
		for _, inputArtifact := range inputArtifacts {
			inputFilePath, isLocalPath := buildtool.ResolveArtifact(inputArtifact)
			if !isLocalPath {
				return fmt.Errorf("failed to resolve input artifact path: input artifact doesn't have a valid local path")
			}
			inputFileMap[inputArtifact.GetTag()] = inputFilePath
		}
	}
	// Get Wwise command args
	args := []string{
		wproj,
		"-GenerateSoundBanks",
	}
	defFile, err := findFileByTag("temp_definition_file", inputFileMap)
	if err != nil {
		return fmt.Errorf("failed to get definition file: %v", err)
	}
	args = append(args, "-ImportDefinitionFile", defFile)
	// Add bank name args
	sbManifestFile, err := findFileByTag("soundbank_manifest", inputFileMap)
	if err != nil {
		return fmt.Errorf("failed to get soundbank manifest: %v", err)
	}
	sbManifestContent, err := ioutil.ReadFile(sbManifestFile)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %v", sbManifestFile, err)
	}
	sbManifest := &soundbankpb.SoundbankManifest{}
	if err := proto.UnmarshalText(string(sbManifestContent), sbManifest); err != nil {
		return fmt.Errorf("failed to unmarshal SoundbankManifest from %s: %v", sbManifestFile, err)
	}
	banks := sbManifest.GetBanks()
	for _, bank := range banks {
		args = append(args, "-Bank", bank)
	}
	// Add output path args
	outputDir := helper.Invocation().GetBuildInvocation().GetOutputDir()
	outputBase := helper.Invocation().GetBuildInvocation().GetOutputBase()
	if outputDir == "" || outputBase == "" {
		return fmt.Errorf("failed to get output dir or output base")
	}
	args = append(args, "-Platform", "Windows", "-SoundBankPath", "Windows", filepath.Join(outputDir, "Windows"))
	// Run wwise command
	log.Info("Run WwiseCLI's GenerateSoundBanks")
	com := exec.Command(wwiseCliExe, args...)
	log.Info(com)
	var stdoutLogsByte, stderrLogsByte bytes.Buffer
	com.Stdout = io.MultiWriter(log.NewInfoLogger(log.Get()), &stdoutLogsByte)
	com.Stderr = io.MultiWriter(log.NewInfoLogger(log.Get()), &stderrLogsByte)
	if err := com.Run(); err != nil {
		// WwiseCLI returns 2 when it has warnings.
		// https://www.audiokinetic.com/fr/library/2017.2.10_6745/?source=SDK&id=bankscommandline.html
		if com.ProcessState.ExitCode() == 2 {
			log.Warning("WwiseCLI executed with warnings")
		} else {
			return fmt.Errorf("failed to Run WwiseCLI's GenerateSoundBanks: %v", err)
		}
	}
	logs := []*buildpb.Artifact{
		&buildpb.Artifact{
			Tag:      "stdout",
			Contents: stdoutLogsByte.Bytes(),
		},
		&buildpb.Artifact{
			Tag:      "stderr",
			Contents: stderrLogsByte.Bytes(),
		},
	}
	// Write the wwise output files to artifacts
	outputs := []*buildpb.Artifact{}
	if outputs, err = appendOutputFileToArtifacts(outputs, outputBase, outputDir); err != nil {
		return err
	}
	artifactSet := &buildpb.ArtifactSet{
		Tag:       "soundbanks",
		Artifacts: outputs,
	}
	helper.MustWriteBuildResult(&buildpb.BuildInvocationResult{
		Result: &buildpb.Result{
			Success: err == nil,
			Logs:    logs,
		},
		ArtifactSet: artifactSet,
	})
	return nil
}

func main() {
	flags := wbFlags{}
	flag.StringVar(&flags.wproj, "wproj", "", "Path to the Wwise .wproj file")
	flag.StringVar(&flags.wwiseCliExe, "wwise_exe", "", "Path to WwiseCLI exe")
	flag.Parse()
	log.AddSink(log.NewGlog())
	defer log.Shutdown()
	if err := buildSoundbanks(flags); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}
