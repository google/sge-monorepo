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
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/hashicorp/go-version"

	"sge-monorepo/build/cicd/sgeb/buildtool"
	"sge-monorepo/libs/go/files"
)

// config is the set of flags that drives unreal_builder.
// NOTE: You must call |Validate| on config before using it.
type config struct {
	action        string
	uproject      string
	projectName   string
	editor        string
	config        string
	platform      string
	editorVersion *version.Version
	maps          []string
}

func newConfig(helper buildtool.Helper, flags ubFlags) (*config, error) {
	c := &config{}

	if flags.action == "" {
		return nil, errors.New("-action must not be empty")
	}
	c.action = flags.action

	if flags.uproject == "" {
		return nil, errors.New("-uproject must not be empty")
	}
	uproject, err := helper.ResolvePath(flags.uproject)
	if err != nil {
		return nil, fmt.Errorf("could not resolve uproject path %q: %w", flags.uproject, err)
	}
	if !files.FileExists(uproject) {
		return nil, fmt.Errorf("could not find uproject file %q", uproject)
	}
	absUProject, err := filepath.Abs(uproject)
	if err != nil {
		return nil, fmt.Errorf("could not get absolute path for %q: %w", uproject, err)
	}
	c.uproject = absUProject
	c.projectName = baseWithoutExtension(c.uproject)

	if flags.editor == "" {
		return nil, errors.New("-editor must not be empty")
	}
	editor, err := helper.ResolvePath(flags.editor)
	if err != nil {
		return nil, fmt.Errorf("could not resolve editor path %q: %w", editor, err)
	}
	if !files.DirExists(editor) {
		return nil, fmt.Errorf("editor path %q is not a directory", editor)
	}
	engineDir := filepath.Join(editor, "Engine")
	if !files.DirExists(engineDir) {
		return nil, fmt.Errorf("could not find %q directory", engineDir)
	}
	c.editor = editor
	if c.editorVersion, err = readEngineVersion(editor); err != nil {
		return nil, fmt.Errorf("could not read engine version: %v", err)
	}

	if flags.action != "build_editor" && flags.platform == "" {
		return nil, errors.New("-platform must not be empty")
	}
	c.platform = flags.platform

	if flags.action != "build_editor" && flags.config == "" {
		return nil, errors.New("-config must not be empty")
	}
	c.config = flags.config
	c.maps = flags.maps

	return c, nil
}

func readEngineVersion(editor string) (*version.Version, error) {
	versionBytes, err := ioutil.ReadFile(filepath.Join(editor, "Engine/Build/Build.version"))
	if err != nil {
		return nil, err
	}
	versionMap := map[string]interface{}{}
	if err := json.Unmarshal(versionBytes, &versionMap); err != nil {
		return nil, err
	}
	major := versionMap["MajorVersion"].(float64)
	minor := versionMap["MinorVersion"].(float64)
	patch := versionMap["PatchVersion"].(float64)
	return version.NewVersion(fmt.Sprintf("%d.%d.%d", int32(major), int32(minor), int32(patch)))
}

// baseWithoutExtension gives the path filename without extension:
// eg. tests/unreal/test-game-sge/sgegame.uproject -> sgegame
func baseWithoutExtension(path string) string {
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	return base[0 : len(base)-len(ext)]
}
