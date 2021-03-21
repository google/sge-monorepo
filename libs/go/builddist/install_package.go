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

//temp auth : execute "gcloud auth application-default login" before running the script

import (
	"compress/gzip"
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type feedback func(string)

// isFileUpToDate : compares hash of local file to metadata entry
func isFileUpToDate(path string, hash string) (bool, error) {
	file, err := os.Open(path)
	if err != nil && os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	defer file.Close()

	h := sha1.New()
	if _, err := io.Copy(h, file); err != nil {
		return false, err
	}

	fileHash := fmt.Sprintf("%x", h.Sum(nil))
	return hash == fileHash, nil
}

func downloadFile(
	fileEntry FileEntry, //metadata entry of the file
	packageConfig *PackageConfig, //json config with folder mapping
	ctx context.Context, //to access bucket
	bucket *storage.BucketHandle, //bucket
	packagePath string, //root of the package in the bucker
	wgFileDownload *sync.WaitGroup, //semaphore
	fb feedback) error {

	defer wgFileDownload.Done()

	// about paths...
	// json config refers to paths with placeholders like this : %ue%/Engine/Binaries
	// these will end up in a bucket : product/version_id/ue/engine/binaries/some.exe
	// with product/version_id/ being the root of the package in the bucket
	// the json config will map top-level placeholders to relative local paths (i.e. "%ue%" : "../../../ue4/Release-4.24-sge" )
	relativePath := strings.ReplaceAll(fileEntry.RelativePath, "\\", "/")
	relativePathSplit := strings.Split(relativePath, "/")
	topLevelPlaceholder := "%" + relativePathSplit[0] + "%"
	local_path := filepath.Join(packageConfig.Placeholders[topLevelPlaceholder], strings.Join(relativePathSplit[1:], "/"))
	if up, err := isFileUpToDate(local_path, fileEntry.Hash); up || err != nil {
		if err == nil && up {
			fb(fmt.Sprintf("%q up to date", local_path))
		}
		return err
	}
	bucket_path := strings.ReplaceAll(filepath.Join(packagePath, fileEntry.RelativePath), "\\", "/")
	remote_reader, err := bucket.Object(bucket_path).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("failed to create remote reader: %v", err)
	}

	fb(fmt.Sprintf("downloading %q", local_path))

	zipReader, err := gzip.NewReader(remote_reader)
	if err != nil {
		return fmt.Errorf("failed to create zip reader: %v", err)
	}

	local_directory, _ := filepath.Split(local_path)
	os.MkdirAll(local_directory, os.ModePerm)
	file, err := os.Create(local_path)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	if _, err := io.Copy(file, zipReader); err != nil {
		return fmt.Errorf("copy failed: %v", err)
	}
	defer file.Close()

	if err := zipReader.Close(); err != nil {
		return fmt.Errorf("problem closing zip reader: %v", err)
	}

	if err := remote_reader.Close(); err != nil {
		return fmt.Errorf("problem closing remote reader: %v", err)
	}
	return nil
}

//
// InstallPackage : download all the files from a package to the local disk. Uses sha1
//    to avoid downloading files that are already up to date
//
func InstallPackage(ctx context.Context, config PackageConfig, versionId string, authOption option.ClientOption, fb feedback) error {
	fb("starting install")

	client, err := datastore.NewClient(ctx, config.GcpProject, authOption)
	if err != nil {
		return fmt.Errorf("Failed to create datastore client: %v", err)
	}

	storageClient, err := storage.NewClient(ctx, authOption)
	if err != nil {
		return fmt.Errorf("storage.NewClient: %v", err)
	}
	bucket := storageClient.Bucket(config.BucketName)

	productKey := datastore.NameKey(ProductKind, config.ProductName, nil)
	packageKey := datastore.NameKey(PackageKind, versionId, productKey)

	var pkg PackageEntry
	if err := client.Get(ctx, packageKey, &pkg); err != nil {
		return fmt.Errorf("error fetching package: %v", err)
	}

	if pkg.State != CompleteState {
		return fmt.Errorf("Package incomplete: %v", pkg)
	}

	packagePath := filepath.Join(config.ProductName, versionId)
	var wgFileDownload sync.WaitGroup

	query := datastore.NewQuery(FileEntryKind).Ancestor(packageKey)
	it := client.Run(ctx, query)
	for {
		var item FileEntry
		if _, err := it.Next(&item); err == iterator.Done {
			break
		} else if err != nil {
			return err
		}
		wgFileDownload.Add(1)
		go downloadFile(item, &config, ctx, bucket, packagePath, &wgFileDownload, fb)
	}
	wgFileDownload.Wait()
	fb("install complete")
	return nil
}

// GetPackageContents returns list of all file entries in package
func GetPackageFileEntries(ctx context.Context, config PackageConfig, versionId string, authOption option.ClientOption) ([]FileEntry, error) {
	client, err := datastore.NewClient(ctx, config.GcpProject, authOption)
	if err != nil {
		return nil, fmt.Errorf("Failed to create datastore client: %v", err)
	}

	productKey := datastore.NameKey(ProductKind, config.ProductName, nil)
	packageKey := datastore.NameKey(PackageKind, versionId, productKey)

	var pkg PackageEntry
	if err := client.Get(ctx, packageKey, &pkg); err != nil {
		return nil, fmt.Errorf("error fetching package: %v", err)
	}

	if pkg.State != CompleteState {
		return nil, fmt.Errorf("Package incomplete: %v", pkg)
	}

	query := datastore.NewQuery(FileEntryKind).Ancestor(packageKey)
	it := client.Run(ctx, query)
	var entries []FileEntry
	for {
		var item FileEntry
		if _, err := it.Next(&item); err == iterator.Done {
			break
		} else if err != nil {
			return nil, err
		}
		entries = append(entries, item)
	}
	return entries, nil
}
