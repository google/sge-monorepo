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

package p4lib

// #include "p4_cgo_bridge.h"
import "C"

// The p4str function is used to pass 'views' of Go strings to C/C++ without
// incurring any allocation.  These views *must not* be held on the C/C++ side
// past the return of the function that takes them as arguments.

// strview p4str(_GoString_ s) {
//   strview str;
//   str.p = _GoStringPtr(s);
//   str.len = _GoStringLen(s);
//   return str;
// }
import "C"
