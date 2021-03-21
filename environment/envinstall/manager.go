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

package envinstall

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/protobuf/proto"

	"sge-monorepo/build/cicd/cirunner/protos/cirunnerpb"
	"sge-monorepo/libs/go/cloud/secretmanager"
	"sge-monorepo/libs/go/p4lib"
)

const (
	gInstallFilename = "env-install-version"
	currentVersion   = "2"
)

// EnvironmentType describes in which environment code is running.
type EnvironmentType string

const (
	// None represents a no-op environment. Used when returning an error.
	None EnvironmentType = "<none>"
	// Local means a local workspace environment which no special credential access.
	Local = "local"
	// CiDev means the code is configured to obtain CI dev credentials.
	CiDev = "ci_dev"
	// CiProd means the code is configured to obtain CI prod credentials.
	CiProd = "ci_prod"
)

// Manager handles the appdata file that determines if the file has been installed or not.
type Manager interface {
	// UpToDate returns whether the current installed environment is up to date or not.
	// Some users might prefer not to reinstall if this is true. Reinstalling dependencies will
	// maintain the system in a valid state, so this check is for optimization purposes.
	UpToDate() (bool, error)

	// SyncAndInstallDependencies performs a full install of all the known dependencies.
	// Will also update the current installed version marker.
	SyncAndInstallDependencies() error
}

// IsCloud returns whether we are running on a jenkins machine.
// TODO: Fix to be more generic.
func IsCloud() bool {
	return os.Getenv("NODE_NAME") != ""
}

// Environment returns in which environment this code is running. It does this by querying for
// environment variables that would be set on CI. If found, it queries the secret service of the
// cloud environment to find out in which environment it is running.
func Environment() (EnvironmentType, error) {
	// Jenkins sets up some environment variables on each runner.
	if os.Getenv("NODE_NAME") == "" {
		return Local, nil
	}
	// Since we have a Jenkins environment variable, we can now query for cloud secrets, as the
	// setup _should_ be running.
	secrets, err := secretmanager.NewFromDefaultProject()
	if err != nil {
		return None, fmt.Errorf("could not create secretmanager: %v", err)
	}
	// CI projects have a special credentials that indicate what project they're on.
	secretName := "cirunner_environment"
	secret, ok, err := secrets.AccessLatest(secretName)
	if err != nil || !ok {
		return None, fmt.Errorf("could not obtain secret %q (nil == not found): %v", secretName, err)
	}
	// Attemp to parse the environment.
	env := &cirunnerpb.Environment{}
	if err := proto.UnmarshalText(secret, env); err != nil {
		return None, fmt.Errorf("could not unmarshal environment proto %s: %v", secret, err)
	}
	if env.Env == cirunnerpb.Environment_DEV {
		return CiDev, nil
	} else if env.Env == cirunnerpb.Environment_PROD {
		return CiProd, nil
	}
	return None, fmt.Errorf("unexpected env proto value: %v", env.Env)
}

// Impl --------------------------------------------------------------------------------------------

type manager struct {
	p4      p4lib.P4
	appdata string
	data    string
	sysroot string
}

func NewManager(p4 p4lib.P4) (Manager, error) {
	appdata := os.Getenv("APPDATA")
	if appdata == "" {
		return nil, fmt.Errorf("could not find APPDATA folder")
	}
	// We obtain the path to the sge environment data.
	dataPath, err := p4.Where("//sge/environment/data")
	if err != nil {
		return nil, fmt.Errorf("cannot find //sge/environment: %v. Please verify syncing", err)
	}
	// Obtain where the Windows installation is.
	sysroot := os.Getenv("SYSTEMROOT")
	if sysroot == "" {
		return nil, fmt.Errorf("could not find SYSTEMROOT")
	}
	return manager{
		p4:      p4,
		appdata: appdata,
		data:    dataPath,
		sysroot: sysroot,
	}, nil
}

func (m manager) UpToDate() (bool, error) {
	installed, err := m.readCurrentVersion()
	if err != nil {
		return false, err
	}
	return installed == currentVersion, nil
}

func (m manager) readCurrentVersion() (string, error) {
	// If file doesn't exist, we return empty.
	path := filepath.Join(m.appdata, "sge", gInstallFilename)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", nil
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	return strings.Trim(string(data), "\r\n"), nil
}

func (m manager) writeCurrentVersion() error {
	// Create the directory just in case.
	if err := os.MkdirAll(filepath.Join(m.appdata, "sge"), os.ModeDir); err != nil {
		return err
	}

	path := filepath.Join(m.appdata, "sge", gInstallFilename)
	return ioutil.WriteFile(path, []byte(currentVersion), 0755)
}
