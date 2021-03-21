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

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

//
// ListPackages returns the list of packages associated with a specified product.
//
func ListPackages(ctx context.Context, config PackageConfig, authOption option.ClientOption) ([]PackageEntry, error) {

	client, err := datastore.NewClient(ctx, config.GcpProject, authOption)
	if err != nil {
		return nil, fmt.Errorf("Failed to create client: %v", err)
	}

	productKey := datastore.NameKey(ProductKind, config.ProductName, nil)

	var product ProductEntry
	if err := client.Get(ctx, productKey, &product); err != nil {
		return nil, fmt.Errorf("error fetching product %s: %v ", config.ProductName, err)
	}

	packageQuery := datastore.NewQuery(PackageKind).Ancestor(productKey)

	var packages []PackageEntry

	it := client.Run(ctx, packageQuery)
	for {
		var pkg PackageEntry
		if _, err := it.Next(&pkg); err == iterator.Done {
			break
		} else if err != nil {
			return nil, fmt.Errorf("failed to iterate to next package entry: %v", err)
		}
		packages = append(packages, pkg)
	}
	return packages, nil
}

// ListCompletePackages returns the list of PackageEntry where State is CompleteState.
//   todo : this is very specific and it's in the commands package only because external code can't access
//          both commands and and common modules (build system not yet ready)
func ListCompletePackages(ctx context.Context, config PackageConfig, authOption option.ClientOption) ([]PackageEntry, error) {
	var packages []PackageEntry
	lp, err := ListPackages(ctx, config, authOption)
	if err != nil {
		return nil, err
	}
	for _, pkg := range lp {
		if pkg.State == CompleteState {
			packages = append(packages, pkg)
		}
	}
	return packages, nil
}
