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

package zip

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"sge-monorepo/libs/go/zip_utils"
	"sge-monorepo/tools/vendor_bender/protos/metadatapb"
)

func downloadFile(url string, file *os.File) (string, error) {
	//Get the response bytes from the url
	response, err := http.Get(url)
	if err != nil {
	}
	defer response.Body.Close()

	//Write the bytes to the file
	if _, err = io.Copy(file, response.Body); err != nil {
		return "", err
	}
	if _, err = file.Seek(0, 0); err != nil {
		return "", err
	}
	hasher := sha1.New()
	if _, err = io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func unzipFile(filename string, dest string) error {
	return zip_utils.UnzipFile(filename, dest)
}

func copyDir(srcPath, dstPath string) error {
	srcStat, err := os.Stat(srcPath)
	if err != nil {
		return err
	}

	if err = os.MkdirAll(dstPath, srcStat.Mode()); err != nil {
		return err
	}

	children, err := ioutil.ReadDir(srcPath)
	if err != nil {
		return err
	}
	for _, child := range children {
		chilSrcPath := filepath.Join(srcPath, child.Name())
		childDstPath := filepath.Join(dstPath, child.Name())

		if child.IsDir() {
			if err = copyDir(chilSrcPath, childDstPath); err != nil {
				return err
			}
		} else {
			srcFile, err := os.Open(chilSrcPath)
			if err != nil {
				return err
			}
			defer srcFile.Close()

			dstFile, err := os.Create(childDstPath)
			if err != nil {
				return err
			}
			defer dstFile.Close()

			if _, err = io.Copy(dstFile, srcFile); err != nil {
				return err
			}

			srcStat, err := os.Stat(chilSrcPath)
			if err != nil {
				return err
			}
			if err = os.Chmod(childDstPath, srcStat.Mode()); err != nil {
				return err
			}
		}
	}
	return nil
}

func ZipPkg(name, url, version, dst string) (*metadatapb.Metadata, error) {
	zipFile, err := ioutil.TempFile("", name+".*.zip")
	if err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("temp file creation failed: %v", err)
	}
	defer os.Remove(zipFile.Name()) // clean up

	sha1, err := downloadFile(url, zipFile)
	if err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("zip download failed: %v", err)
	}

	if err = unzipFile(zipFile.Name(), dst); err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("failed to unzip the file %s: %v", zipFile.Name(), err)
	}

	children, err := ioutil.ReadDir(dst)
	if err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("failed to read unzipped directory %s: %v", dst, err)
	}
	// A lot of the zip files are made of one subdirectory we move it to the repo path instead
	if len(children) == 1 {
		subDir := filepath.Join(dst, children[0].Name())
		if err = copyDir(subDir, dst); err != nil {
			return &metadatapb.Metadata{}, fmt.Errorf("failed to move single sub-directory %s: %v", subDir, err)
		}
		if err = os.RemoveAll(subDir); err != nil {
			return &metadatapb.Metadata{}, fmt.Errorf("failed to move single sub-directory %s: %v", subDir, err)
		}
	}
	metadata := &metadatapb.Metadata{
		Name: name,
		ThirdParty: &metadatapb.ThirdParty{
			Source: &metadatapb.Source{
				ZipPkg: &metadatapb.ZipPkg{
					Url:     url,
					Sha:     sha1,
					Version: version,
				},
			},
		},
	}
	return metadata, nil
}
