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

// Package files handles common file operations.

package files

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
)

// FileExists returns true if |path| points to a regular file.
func FileExists(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.Mode().IsRegular()
}

// DirExists returns true if |path| points to a directory.
func DirExists(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.IsDir()
}

// Copy creates a copy of the file with broad permissions.
// For more control over permissions, use CopyEx
func Copy(src, dst string) error {
	return CopyEx(src, dst, 0664)
}

// CopyEx copies a file with control of the permissions.
func CopyEx(src, dst string, perm os.FileMode) error {
	dstDir := filepath.Dir(dst)
	if _, err := os.Stat(dstDir); os.IsNotExist(err) {
		if err := os.MkdirAll(dstDir, os.ModePerm); err != nil {
			return err
		}
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	dstFile, err := os.OpenFile(dst, os.O_RDWR|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	_, err = io.Copy(dstFile, srcFile)
	return err
}

func CopyDir(src, dst string) error {
	children, err := ioutil.ReadDir(src)
	if err != nil {
		return err
	}
	for _, child := range children {
		srcPath := path.Join(src, child.Name())
		dstPath := path.Join(dst, child.Name())
		if child.IsDir() {
			if err = CopyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err = Copy(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// GetAppDir returns the absolute directory for the application directory associated with app
// If this directory doesn't yet exist, it will be created.
// |appDomain| is the containing directly where the |appName| directory will be created.
func GetAppDir(appDomain, appName string) (string, error) {
	appDir := ""
	if runtime.GOOS == "windows" {
		appDir = filepath.Join(os.Getenv("APPDATA"), appDomain, appName)
		if _, err := os.Stat(appDir); os.IsNotExist(err) {
			if err := os.MkdirAll(appDir, os.ModeDir); err != nil {
				return "", fmt.Errorf("couldn't create app directory %s: %v", appDir, err)
			}
		}
	}
	return appDir, nil
}

// GetAppDirFileName returns the absolute path for the specified file in application directory
// If the application directory doesn't yet exist, it will be created.
func GetAppDirFileName(appDomain, appName, fileName string) (string, error) {
	base, err := GetAppDir(appDomain, appName)
	if err != nil {
		return "", err
	}
	return filepath.Join(base, fileName), nil
}

// JsonLoad loads data from fileName and deserializes to dest.
func JsonLoad(fileName string, dest interface{}) error {
	if b, e := ioutil.ReadFile(fileName); e == nil {
		if err := json.Unmarshal(b, dest); err != nil {
			return fmt.Errorf("couldn't deserialize json %s: %v", fileName, err)
		}
	}
	return nil
}

// JsonSave serializes data into json format and saves to fileName
func JsonSave(fileName string, data interface{}) error {
	b, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("couldn't marshal changes: %v\n", err)
	}
	err = ioutil.WriteFile(fileName, b, os.ModePerm)
	if err != nil {
		return fmt.Errorf("couldn't write changes to %s: %v\n", fileName, err)
	}
	return nil
}

// IsExecutable returns true if the file exists and is executable.
func IsExecutable(p string) (bool, error) {
	stat, err := os.Stat(p)
	if err != nil {
		return false, err
	}
	if runtime.GOOS == "windows" {
		ext := path.Ext(p)
		return ext == ".exe" || ext == ".bat", nil
	}
	return stat.Mode()&0111 != 0, nil
}
