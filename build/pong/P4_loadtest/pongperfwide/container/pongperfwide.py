# Copyright 2021 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import os,sys,socket,time
from P4 import P4,P4Exception    # Import the module
# Create the P4 instance
p4 = P4()

# Set some environment variables
p4.port = "ssl:edge-1-us-east4:1666"
p4.user = "TenGB"
p4.password = "tengignic10"
p4.client = "TenGB_tests_template"

template_client = "TenGB_tests_template"
client_root = os.getcwd() + "\workspace"

# Connect + login
ret = p4.connect()
print ret
os.environ["P4TRUST"]="/root/.p4trust"
p4.run_trust("-i", 'C6:02:DE:53:C9:91:49:62:2E:67:49:BA:48:A5:6A:CC:CA:06:05:2B')
ret = p4.run_login()
print ret
p4.disconnect()
    
    