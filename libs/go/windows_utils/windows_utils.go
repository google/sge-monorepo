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

// #include "windows_utils.h"
import "C"

import (
	"syscall"
	"unsafe"
)

// Converts a Go string to LPCWSTR.
// The cgo caller should never store a reference to that pointer.
func makeStringParam(param string) C.LPCWSTR {
	ptr, _ := syscall.UTF16PtrFromString(param)
	return C.LPCWSTR(unsafe.Pointer(ptr))
}

// Convers a Golang bool to C int
func boolToInt(param bool) C.int {
	if param {
		return 1
	} else {
		return 0
	}
}

func IsAdmin() bool {
	return C.IsAdmin() != 0
}

// Runs a shell command in regular mode
func RunShellCommand(file string, parameters string, waitForCompletion bool) bool {
	return C.RunShellCommand(
		makeStringParam(file),
		makeStringParam(parameters),
		makeStringParam(""),
		makeStringParam(""),
		1, /* SW_SHOWNORMAL */
		boolToInt(waitForCompletion)) != 0
}

// Runs a shell command in elevated (admin) mode
func RunElevatedShellCommand(file string, parameters string, waitForCompletion bool) bool {
	return C.RunShellCommand(
		makeStringParam(file),
		makeStringParam(parameters),
		makeStringParam(""),
		makeStringParam("runas"),
		1, /* SW_SHOWNORMAL */
		boolToInt(waitForCompletion)) != 0
}
