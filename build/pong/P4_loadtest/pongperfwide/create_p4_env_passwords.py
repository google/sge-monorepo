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

import os

BaseUserName = "pongperfwide"
NumberOfUsers = 180

##
def create_user_pwds(basename, numberUsers): 
    
    # add new user to p4
    for i in range(numberUsers):        
        p = os.popen("p4 passwd {0}-{1}".format(basename, i), "w")
        p.write("{0}-{1}\n".format(basename, i))     
        p.write("{0}-{1}\n".format(basename, i))
    
#------------------------------
    
create_user_pwds(BaseUserName, NumberOfUsers)