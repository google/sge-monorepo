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
	"google.golang.org/api/option"
)

//
// RegisterProduct : add a product entity in the metedata store
//
func RegisterProduct(ctx context.Context, config PackageConfig, authOption option.ClientOption) error {

	client, err := datastore.NewClient(ctx, config.GcpProject, authOption)
	if err != nil {
		return fmt.Errorf("Failed to create client: %v", err)
	}

	productKey := datastore.NameKey("product", config.ProductName, nil)

	product := ProductEntry{}
	if _, err := client.Put(ctx, productKey, &product); err != nil {
		return fmt.Errorf("Failed to save product: %v", err)
	}

	return nil
}
