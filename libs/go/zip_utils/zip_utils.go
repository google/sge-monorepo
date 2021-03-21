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

package zip_utils

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func extractSingleFile(file *zip.File, dest string) error {
	zippedFile, err := file.Open()
	if err != nil {
		return err
	}
	defer zippedFile.Close()

	extractedFilePath := filepath.Join(dest, file.Name)

	if file.FileInfo().IsDir() {
		if err = os.MkdirAll(extractedFilePath, file.Mode()); err != nil {
			return fmt.Errorf("failed to create directory %s : %v", extractedFilePath, err)
		}
	} else {
		if err = os.MkdirAll(filepath.Dir(extractedFilePath), file.Mode()); err != nil {
			return fmt.Errorf("failed to create directory %s : %v", filepath.Dir(extractedFilePath), err)
		}

		outputFile, err := os.OpenFile(extractedFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
		if err != nil {
			return fmt.Errorf(" failed to create file %s :%v", extractedFilePath, err)
		}
		defer outputFile.Close()

		if _, err = io.Copy(outputFile, zippedFile); err != nil {
			return fmt.Errorf("failed to copy %v to %s : %v", outputFile, zippedFile, err)
		}
	}
	return nil
}

// Unzips a file under the specified destination directory, optionally filtering out by subdirectory prefix.
func unzipFile(filename string, subdirectory string, dest string) error {
	zipReader, err := zip.OpenReader(filename)
	if err != nil {
		return err
	}
	defer zipReader.Close()

	for _, file := range zipReader.Reader.File {
		if len(subdirectory) > 0 && !strings.HasPrefix(file.Name, subdirectory) {
			continue
		}
		err := extractSingleFile(file, dest)
		if err != nil {
			return err
		}
	}
	return nil
}

// Unzips a file under the specified destination directory.
func UnzipFile(filename string, dest string) error {
	return unzipFile(filename, "", dest)
}

// Unzips a file under the specified destination directory, filtering out by subdirectory prefix.
func UnzipSubdirectoriesFromFile(filename string, subdirectory string, dest string) error {
	return unzipFile(filename, subdirectory, dest)
}
