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
p4.port = "ssl:localhost:1666"
p4.user = "super"
p4.password = ""
#p4.client = "TenGB_tests_template"

BaseUserName = "pongperfwide"
NumberOfUsers = 180

##
def create_users(basename, numberUsers): 
    
    # add new user to p4
    for i in range(numberUsers):
        newuser = {
            'User': '{0}-{1}'.format(basename, i),
            'Email': '{0}-{1}@domain.com'.format(basename, i),
            'FullName': 'PongPerfWide User {0}'.format(i),
            # 'Password': '{0}-{1}'.format(basename, i) # passwords have to be set by user or by super manually!
        }
        ret = p4.save_user(newuser,"-f")
        print ret        
            
    # print to use in adding to groups
    for i in range(numberUsers):
        print '{0}-{1}'.format(basename, i)
    
#------------------------------
    
# Main
try:
    # Connect + login
    ret = p4.connect()
    print ret
    ret = p4.run_login()
    #print ret

    create_users(BaseUserName, NumberOfUsers)
            
    # Disconnect from the server
    p4.disconnect()    
    print "************************************************************"
except P4Exception as ex:
    print "%s" % P4Exception
    for e in p4.errors:
        print e
    for e in p4.warnings:
        print e