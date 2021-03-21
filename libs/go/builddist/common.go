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
	"cloud.google.com/go/datastore"
)

// ProductEntry is only an entity used to group packages.
type ProductEntry struct {
}

// PackageEntry is a set of files specific to a product.
type PackageEntry struct {
	State   string
	Version string
}

// FileEntry represents one file in one package entry.
type FileEntry struct {
	RelativePath string         `datastore:",noindex"`
	Hash         string         `datastore:",noindex"`
	Key          *datastore.Key `datastore:"__key__"`
}

//types of metadata entities in datastore
const FileEntryKind = "file_entry"
const PackageKind = "package"
const ProductKind = "product"

//package states
const CompleteState = "complete"
const WritingeState = "writing"
