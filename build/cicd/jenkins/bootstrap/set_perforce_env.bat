@rem Copyright 2021 Google LLC
@rem
@rem Licensed under the Apache License, Version 2.0 (the "License");
@rem you may not use this file except in compliance with the License.
@rem You may obtain a copy of the License at
@rem
@rem      http://www.apache.org/licenses/LICENSE-2.0
@rem
@rem Unless required by applicable law or agreed to in writing, software
@rem distributed under the License is distributed on an "AS IS" BASIS,
@rem WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
@rem See the License for the specific language governing permissions and
@rem limitations under the License.

:: This script sets the basic env in which a batch run can simply starts issuing p4 commands from
:: the root of the perforce repository. It is assumed that |bootstrap.bat| was already called.

@echo off
:: Add all the configuration artifacts to PATH.
set PATH=C:\artifacts;%PATH%

:: Add all the vendored binaries to PATH
set PATH=C:\p4\sge\bin\windows;%PATH%

:: Base ourself in the root of the client.
cd C:\p4\sge
@echo on
