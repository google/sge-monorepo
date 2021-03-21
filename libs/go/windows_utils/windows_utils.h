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

#ifndef WINDOWS_UTILS_H
#define WINDOWS_UTILS_H

#include <windows.h>

#ifdef __cplusplus
extern "C" {
#endif

  // Returns whether or not the current process is running with administrative privileges.
  BOOL IsAdmin();

  // Wrapper around native ShellExevuteEx optionally allowing to wait for the process termination.
  // See https://docs.microsoft.com/en-us/windows/win32/api/shellapi/nf-shellapi-shellexecuteexw for details.
  BOOL RunShellCommand(LPCWSTR file, LPCWSTR parameters, LPCWSTR directory, LPCWSTR verb, int show, BOOL waitForCompletion);

#ifdef __cplusplus
}
#endif

#endif
