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

package windows_utils

import (
	"fmt"
	"os"
)

func main() {
	executable, err := os.Executable()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Testing windows_utils\n")
	isAdmin := IsAdmin()
	fmt.Printf("IsAdmin: %v\n", isAdmin)

	if !isAdmin {
		fmt.Printf("Launching as admin: %v\n", RunElevatedShellCommand("cmd", fmt.Sprintf("/k title windows_utils_test & %v", executable), true))
	}
}
