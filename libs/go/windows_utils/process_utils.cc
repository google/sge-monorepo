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

#include "windows_utils.h"

// https://docs.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-checktokenmembership
BOOL IsUserAdmin(VOID)
/*++ 
Routine Description: This routine returns TRUE if the caller's
process is a member of the Administrators local group. Caller is NOT
expected to be impersonating anyone and is expected to be able to
open its own process and process token. 
Arguments: None. 
Return Value: 
   TRUE - Caller has Administrators local group. 
   FALSE - Caller does not have Administrators local group. --
*/ 
{
  BOOL b;
  SID_IDENTIFIER_AUTHORITY NtAuthority = SECURITY_NT_AUTHORITY;
  PSID AdministratorsGroup; 
  b = AllocateAndInitializeSid(
      &NtAuthority,
      2,
      SECURITY_BUILTIN_DOMAIN_RID,
      DOMAIN_ALIAS_RID_ADMINS,
      0, 0, 0, 0, 0, 0,
      &AdministratorsGroup); 
  if(b) 
  {
      if (!CheckTokenMembership( NULL, AdministratorsGroup, &b)) 
      {
          b = FALSE;
      } 
      FreeSid(AdministratorsGroup); 
  }

return(b);
}

BOOL IsAdmin() {
  return IsUserAdmin();
}

BOOL RunShellCommand(LPCWSTR file, LPCWSTR parameters, LPCWSTR directory, LPCWSTR verb, int show, BOOL waitForCompletion) {
  SHELLEXECUTEINFOW seiw;
  seiw.cbSize = sizeof(SHELLEXECUTEINFOW);
  if (waitForCompletion) {  
    seiw.fMask = SEE_MASK_NOCLOSEPROCESS;
  }
  seiw.lpVerb = verb;
  seiw.lpFile = file;
  seiw.lpParameters = parameters;
  seiw.lpDirectory = directory;
  seiw.nShow = show;

  BOOL result = ShellExecuteExW(&seiw);

  if (result && waitForCompletion) {
    WaitForSingleObject(seiw.hProcess, INFINITE);
    CloseHandle(seiw.hProcess);
  }

  return result;
}

