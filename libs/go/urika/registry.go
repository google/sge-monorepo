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

package urika

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

type keyHolder struct {
	path  string
	name  string
	value string
}

// CheckAnySubKeyWithValue determines if any of the subkeys in path have the specified value
// chrome maintains a list of allowlists URI handlers, we want to know if we are in this this
func CheckAnySubKeyWithValue(root registry.Key, path string, value string) error {
	k, err := registry.OpenKey(root, path, registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("couldn't open key %s: %v", path, err)
	}
	defer k.Close()
	// a value of -1 means that all value names are read
	subkeys, err := k.ReadValueNames(-1)
	if err != nil {
		return err
	}
	mask := fmt.Sprintf("%s://*", value)
	for _, sk := range subkeys {
		v, _, err := k.GetStringValue(sk)
		if err != nil {
			return fmt.Errorf("couldn't get subkey value %s: %v", sk, err)
		}
		if v == mask {
			return nil
		}
	}
	return fmt.Errorf("value not found in any subkey")
}

// We want to allowlist our URI handler so that Chrome doesn't fire popups every time we click a link
func allowlistInstall(name string) error {
	allowlist := `SOFTWARE\Policies\Google\Chrome\URLallowlist`

	// first, ascertain if allowlist already contains our URI
	if err := CheckAnySubKeyWithValue(registry.LOCAL_MACHINE, allowlist, name); err == nil {
		return err
	}

	// create base allowlist key
	k, _, err := registry.CreateKey(registry.LOCAL_MACHINE, allowlist, registry.WRITE|registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("couldn't create root key %s: %v", allowlist, err)
	}
	defer k.Close()

	mask := fmt.Sprintf("%s://*", name)
	// iterate through all subkeys to find valid empty subkey for us to use
	sk := 1
	subkeys, err := k.ReadValueNames(-1)
	if err == nil {
		skmap := make(map[string]bool)
		for _, sk := range subkeys {
			skmap[sk] = true
		}
		for {
			// if key for this value doesn't exist, exit loop
			if _, ok := skmap[fmt.Sprintf("%d", sk)]; !ok {
				break
			}
			sk++
		}
	}
	return k.SetStringValue(fmt.Sprintf("%d", sk), mask)
}

// determine if all specified keys and values already exist in registry
func checkHolders(baseKey registry.Key, basePath string, holders []keyHolder) error {
	k, err := registry.OpenKey(baseKey, basePath, registry.QUERY_VALUE)
	if err != nil {
		return fmt.Errorf("couldn't open key %s: %v", basePath, err)
	}
	defer k.Close()

	for _, h := range holders {
		sk := k
		if len(h.path) > 0 {
			sk, err = registry.OpenKey(k, h.path, registry.QUERY_VALUE)
			if err != nil {
				return fmt.Errorf("couldn't open sub key %s: %v", h.path, err)
			}
		}
		v, _, err := sk.GetStringValue(h.name)
		if err != nil {
			return fmt.Errorf("couldn't get key:%s value name %s: %v", h.path, h.name, err)
		}

		if v != h.value {
			return fmt.Errorf("value mismatch. Wanted %s got %s", h.value, v)
		}
	}
	return nil
}

// set all specified keys and values into registry
func installHolders(baseKey registry.Key, basePath string, holders []keyHolder) error {
	k, _, err := registry.CreateKey(baseKey, basePath, registry.WRITE)
	if err != nil {
		return fmt.Errorf("couldn't create root key %s: %v", basePath, err)
	}
	defer k.Close()

	for _, h := range holders {
		sk := k
		if len(h.path) > 0 {
			sk, _, err = registry.CreateKey(k, h.path, registry.WRITE)
			if err != nil {
				return fmt.Errorf("couldn't create sub key %s: %v", h.path, err)
			}
		}
		if err = sk.SetStringValue(h.name, h.value); err != nil {
			return fmt.Errorf("couldn't create name:%s value:%s on:%s : %v", h.name, h.value, h.path, err)
		}
	}

	return nil
}

// AppUriKeyInstall installs the necessary registry keys to facilitate app URI launching
func AppUriKeyInstall(name string, appPath string) error {
	// try to install chrome allowlist; not essential, but minimises user friction
	if err := allowlistInstall(name); err != nil {
		fmt.Printf("error installing chrome allowlist: %v\n", err)
	}

	// set of keys needed for application uri launching
	var holders []keyHolder
	holders = append(holders, keyHolder{value: fmt.Sprintf("URL:%s protocol", name)})
	holders = append(holders, keyHolder{name: "URL protocol"})
	holders = append(holders, keyHolder{path: `shell\open\command`, value: appPath + " " + `"%1"`})

	// determine if registry keys are already installed, if so don't attempt reinstallation (which requires admin rights)
	if err := checkHolders(registry.CLASSES_ROOT, name, holders); err == nil {
		return nil
	}

	return installHolders(registry.CLASSES_ROOT, name, holders)
}
