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

package builddist

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AuthConfig is used for the default user authentification
type AuthConfig struct {
	ClientID           string
	ClientSecret       string
	Scopes             []string
	TokenCacheFilename string
}

// PackageConfig is used to read the json config file.
type PackageConfig struct {
	ProductName  string
	Placeholders map[string]string
	Folders      []string
	GcpProject   string
	BucketName   string
	Auth         AuthConfig
}

//
// The package config is mostly the folders that will be scanned to find the binaries
// that will be distributed
//
func ReadPackageConfig(filename string) (*PackageConfig, error) {
	textReader, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("Error reading package config: %v", err)
	}
	defer textReader.Close()

	jsonDecoder := json.NewDecoder(textReader)
	packageConfig := PackageConfig{}
	if err = jsonDecoder.Decode(&packageConfig); err != nil {
		return nil, fmt.Errorf("Couldn't decode json: %v", err)
	}

	if strings.Contains(packageConfig.Auth.TokenCacheFilename, "%APPDATA%") {
		appdataFolder := os.Getenv("APPDATA")
		if appdataFolder == "" {
			return nil, fmt.Errorf("can't read APPDATA environment variable")
		}
		packageConfig.Auth.TokenCacheFilename = strings.ReplaceAll(packageConfig.Auth.TokenCacheFilename, "%APPDATA%", appdataFolder)
	}

	config_path_abs, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("Couldn't creat absolute path: %v", err)
	}
	config_dir, _ := filepath.Split(config_path_abs)

	//make placeholder paths absolute
	for key, value := range packageConfig.Placeholders {
		absPlaceholder, err := filepath.Abs(filepath.Join(config_dir, value))
		if err != nil {
			return nil, fmt.Errorf("Couldn't creat absolute path: %v", err)
		}
		packageConfig.Placeholders[key] = absPlaceholder
	}

	return &packageConfig, nil
}
