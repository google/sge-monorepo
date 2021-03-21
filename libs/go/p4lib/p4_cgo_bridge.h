/*
 * Copyright 2021 Google LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Define a C interface for the p4 API that is callable from Go.
#include <stddef.h>
#include <stdbool.h>
#ifdef __cplusplus
extern "C" {
#endif

  // Simple string view that can be used to pass data between C and Go
  // without excess copying or allocation.
  typedef struct {
	const char* p;
	int len;
  } strview;

  // Runs a p4 command, sending output to the specified callback.
  int p4runcb(strview cmd, strview user, strview passwd, strview input,
			  strview joined, int argc, void* argv, int cb, bool tag);

#ifdef __cplusplus
}
#endif

