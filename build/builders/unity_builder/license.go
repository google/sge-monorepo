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
	"fmt"
	"os/exec"
	"strings"

	"github.com/golang/protobuf/proto"

	"sge-monorepo/build/builders/unity_builder/protos/unity_builderpb"
	"sge-monorepo/environment/envinstall"
	"sge-monorepo/libs/go/cloud/secretmanager"
	"sge-monorepo/libs/go/log"
)

// ObtainCredentials gets the credentials stored in the GCP secrets project. If the builder is
// running locally, it will return nil credentials, as licensing is managed by Unity Hub locally.
func ObtainCredentials() (*unity_builderpb.Credentials, error) {
	env, err := envinstall.Environment()
	if err != nil {
		return nil, fmt.Errorf("could not obtain running environment: %v", err)
	}
	// Local environment does not need credentials.
	if env == envinstall.Local {
		return nil, nil
	}
	// Unity credentials are obtained in cloud secrets.
	secrets, err := secretmanager.NewFromDefaultProject()
	if err != nil {
		return nil, fmt.Errorf("could not obtain secret manager: %v", err)
	}
	secretName := "unity_credentials"
	secret, ok, err := secrets.AccessLatest(secretName)
	if err != nil || !ok {
		return nil, fmt.Errorf("could not obtain secret %q (nil == not found): %v", secretName, err)
	}
	credentials := &unity_builderpb.Credentials{}
	if err := proto.UnmarshalText(secret, credentials); err != nil {
		return nil, fmt.Errorf("could not unmarshal proto %s: %v", secret, err)
	}
	return credentials, nil
}

// CredentialArgs returns the unity arguments that need to be appended to the Unity invocation.
// If |credentials| is nil this returns an empty slice.
func CredentialArgs(credentials *unity_builderpb.Credentials) []string {
	if credentials == nil {
		return nil
	}
	args := []string{}
	if credentials.Username != "" {
		args = append(args, "-username", credentials.Username)
	}
	if credentials.Password != "" {
		args = append(args, "-password", credentials.Password)
	}
	if credentials.Serial != "" {
		args = append(args, "-serial", credentials.Serial)
	}
	return args
}

// CleanSecrets hides any secrets that are within the arguments.
// If |credentials| is nil, this is a no-op.
func CleanSecrets(args []string, credentials *unity_builderpb.Credentials) []string {
	if credentials == nil {
		return args
	}
	var replacements []string
	if credentials.Username != "" {
		replacements = append(replacements, credentials.Username, "<USERNAME>")
	}
	if credentials.Password != "" {
		replacements = append(replacements, credentials.Password, "<PASSWORD>")
	}
	if credentials.Serial != "" {
		replacements = append(replacements, credentials.Serial, "<SERIAL>")
	}
	replacer := strings.NewReplacer(replacements...)
	var cleanArgs []string
	for _, arg := range args {
		cleanArgs = append(cleanArgs, replacer.Replace(arg))
	}
	return cleanArgs
}

// PrimeLicense runs a dummy command that makes Unity authenticate to the license server and then
// return. If |credentials| is nil this is a no-op. Returns the error stdout on error.
func PrimeLicense(editor string, credentials *unity_builderpb.Credentials) (string, error) {
	if credentials == nil {
		return "", nil
	}
	args := []string{
		editor, "-quit", "-batchmode", "-nographics",
		"-logFile", "-", // Logs to stdout.
	}
	args = append(args, CredentialArgs(credentials)...)
	log.Infof("Running: %s", CleanSecrets(args, credentials))
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Returns the license back to the license server. Pair function of |PrimeLicense| and should be
// always called, as otherwise we would be locking up a license slot. If |credentials| is nil, this
// function is a no-op. Returns the error stdout on error.
func ReturnLicense(editor string, credentials *unity_builderpb.Credentials) (string, error) {
	if credentials == nil {
		return "", nil
	}
	args := []string{
		editor, "-quit", "-batchmode", "-nographics", "-returnlicense",
	}
	args = append(args, CredentialArgs(credentials)...)
	log.Infof("Running: %s", CleanSecrets(args, credentials))
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
