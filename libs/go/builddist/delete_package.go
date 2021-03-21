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
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"cloud.google.com/go/datastore"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

func deleteFileFromBucket(ctx context.Context, bucket *storage.BucketHandle, root string, name string, onFileDeleted func()) {
	pathInBucket := strings.ReplaceAll(filepath.Join(root, name), "\\", "/")
	bucketObj := bucket.Object(pathInBucket)
	if err := bucketObj.Delete(ctx); err != nil {
		fmt.Println(pathInBucket, err)
	} else {
		fmt.Println("deleted ", pathInBucket)
	}
	//
	// What do we want to do with the metadata entry if we fail to delete the bucket object?
	// Right now the common case for this failure is  the delete failing because the object has never been uploaded.
	// But if we end up with a bunch of dangling bucket objects, we might want to do something smarter here.
	//
	onFileDeleted()
}

const datastoreBatchSize = 250

func deleteFileEntries(
	keysToDelete chan *datastore.Key,
	ctx context.Context,
	client *datastore.Client,
	done chan bool) error {

	batch := make([]*datastore.Key, 0, datastoreBatchSize)
	for key := range keysToDelete {
		batch = append(batch, key)
		if len(batch) == datastoreBatchSize {
			if err := client.DeleteMulti(ctx, batch); err != nil {
				return fmt.Errorf("failed to delete multi: %v", err)
			}
			batch = batch[:0]
		}
	}

	if err := client.DeleteMulti(ctx, batch); err != nil {
		return fmt.Errorf("failed to delete multi: %v", err)
	}
	done <- true
	return nil
}

//
//  DeletePackage : delete files in cloud storage and metadata in datastore associated with a specific package
//
func DeletePackage(ctx context.Context, config PackageConfig, versionId string, authOption option.ClientOption) error {

	client, err := datastore.NewClient(ctx, config.GcpProject, authOption)
	if err != nil {
		return fmt.Errorf("failed to create client: %v", err)
	}

	storageClient, err := storage.NewClient(ctx, authOption)
	if err != nil {
		return fmt.Errorf("couldn't create new client: %v", err)
	}
	bucket := storageClient.Bucket(config.BucketName)

	productKey := datastore.NameKey(ProductKind, config.ProductName, nil)
	packageKey := datastore.NameKey(PackageKind, versionId, productKey)

	packagePath := filepath.Join(config.ProductName, versionId)

	query := datastore.NewQuery(FileEntryKind).Ancestor(packageKey)

	keysToDelete := make(chan *datastore.Key, datastoreBatchSize)
	fileEntriesDeleted := make(chan bool)
	go deleteFileEntries(keysToDelete, ctx, client, fileEntriesDeleted)
	var wgFileDelete sync.WaitGroup

	it := client.Run(ctx, query)
	for {
		var item FileEntry
		if _, err := it.Next(&item); err == iterator.Done {
			break
		} else if err != nil {
			return fmt.Errorf("failed to get next file entry: %v", err)
		}

		onFileDeleted := func() {
			keysToDelete <- item.Key
			wgFileDelete.Done()
		}

		wgFileDelete.Add(1)
		go deleteFileFromBucket(ctx, bucket, packagePath, item.RelativePath, onFileDeleted)
	}
	wgFileDelete.Wait()
	close(keysToDelete)
	<-fileEntriesDeleted

	if err = client.Delete(ctx, packageKey); err != nil {
		return fmt.Errorf("failed to delete package: %v", err)
	}
	fmt.Println("package deleted")
	return nil

}
