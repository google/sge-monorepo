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
p4.user = "TenGB"
p4.password = "tengignic10"
p4.client = "TenGB_tests_template"

template_client = "TenGB_tests_template"
client_root = os.getcwd() + "\workspace"

os_big_file = client_root + "\\large_files_big_station.dat"
os_10k_files_dir = client_root + "\\large_files_10k\\*"
os_big_file_size_mb = 163840
os_10k_files_size_mb = 1280
p4_big_file = "//1p/10GB_tests/large_files_big_station.dat"
p4_10k_files_dir = "//1p/10GB_tests/large_files_10k/..."

##
def delete_large_file(): 
    desc = "deleting 20GB file, leave on workspace"
    cl = create_new_cl(desc)
    if cl == "0":
        quit()
        
    ret = p4.run_delete('-c',cl,"-k",p4_big_file)
    submit_cl(cl, desc)

##
def delete_10k_files(): 
    desc = "deleting 10000 * 16KB files, leave on workspace"
    cl = create_new_cl(desc)
    if cl == "0":
        quit()
        
    ret = p4.run_delete('-c',cl,"-k",p4_10k_files_dir)
    #print ret
    submit_cl(cl, desc)
    
##
def get_server_time():

    # Run "p4 info" (returns a dict)
    info = p4.run( "info" )        
    return info[0]["serverDate"]
    
##
def get_throughput(size, time):
    return size / time

##
def sync_latest(parallel=0):
    try:
        # Sync wrapped in try as being up to date is a "warning" <smh>
        if parallel > 0:            
            p4.run_sync("--parallel=threads={0}".format(parallel))
        else:
            p4.run_sync()
    except P4Exception:        
        print "Sync output:"
        for e in p4.errors:
            print "  " + e
        for e in p4.warnings:
            print "  " + e        

##
def create_new_cl(desc = "empty description"):
    changespec = p4.fetch_change()
    changespec["Files"] = ['']
    changespec[ "Description" ] = desc
    result = p4.save_change( changespec )
    results = result[0].split( )
    if results[0] == "Change" and results[2] == "created.":
        return results[1]
    else:
        print "Could not create cl"
        return "0"

##        
def submit_cl(cl, desc, parallel=0, batch=100, min=10):
    print "===================="
    print "Submitting CL " + cl + "..." + desc
    start = time.time()
    if parallel > 0:
        ret = p4.run_submit('-c',cl,"--parallel=threads={0},batch={1},min={2}".format(parallel,batch,min))
    else:
        ret = p4.run_submit('-c',cl)
    end = time.time()
    elapsed = end - start
    print "===================="
    print "Time elapsed (s): {0:0.2f}".format(elapsed)
    print "===================="
    return elapsed
    
##
def test_1a():

    print "===================="
    print "test_1a"    
    print "1 * 20GB file"
    print "commit, serial"
    print "===================="

    sync_latest()
    print "===================="
    
    desc = "test_1a - adding 20GB file, serial"
    cl = create_new_cl(desc)
    if cl == "0":
        quit()
            
    ret = p4.run_add('-c',cl,os_big_file)
    print ret
    elapsed = submit_cl(cl, desc)    
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_big_file_size_mb, elapsed))
    print "===================="

##      
def test_1b():

    print "===================="
    print "test_1b"    
    print "1 * 20GB file"
    print "sync, serial"
    print "===================="
    print "Syncing to " + p4_big_file
    start = time.time()
    p4.run_sync("-f",p4_big_file)
    end = time.time() 
    elapsed = end - start
    print "===================="
    print "Time elapsed (s): {0:0.2f}".format(elapsed)
    print "===================="    
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_big_file_size_mb, elapsed))
    print "===================="
    
    delete_large_file()

##
def test_1c():

    print "===================="
    print "test_1c"   
    print "1 * 20GB file"
    print "commit, --parallel=threads=100,batch=1,min=1"
    print "===================="

    sync_latest()
    print "===================="
    
    desc = "test_1c - adding 20GB file, --parallel=threads=100,batch=1,min=1"
    cl = create_new_cl(desc)
    if cl == "0":
        quit()
            
    ret = p4.run_add('-c',cl,os_big_file)
    print ret
    elapsed = submit_cl(cl, desc, 100, 1, 1)
    #elapsed = submit_cl(cl, desc, 0, 1, 1)
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_big_file_size_mb, elapsed))
    print "===================="
    
    
##
def test_1d():

    print "===================="
    print "test_1d"    
    print "1 * 20GB file"
    print "sync, --parallel=threads=100,batch=1,min=1"
    print "===================="
    print "Syncing to " + p4_big_file
    start = time.time()
    p4.run_sync("--parallel=threads=100,batch=1,min=1","-f",p4_big_file)
    end = time.time()    
    elapsed = end - start
    print "===================="
    print "Time elapsed (s): {0:0.2f}".format(elapsed)
    print "===================="    
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_big_file_size_mb, elapsed))
    print "===================="
        
    delete_large_file()
  
##  
def test_2a():

    print "===================="
    print "test_2a"    
    print "10000 * 16KB files"
    print "commit, serial"
    print "===================="

    sync_latest()
    print "===================="
    
    desc = "test_2a - adding 10000 * 16KB files, serial"
    cl = create_new_cl(desc)
    if cl == "0":
        quit()
    
    print "Adding " + p4_10k_files_dir
    ret = p4.run_add('-c',cl,p4_10k_files_dir)
    elapsed = submit_cl(cl, desc)
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_10k_files_size_mb, elapsed))
    print "===================="

##
def test_2b():

    print "===================="
    print "test_2b"    
    print "10000 * 16KB files"
    print "sync, serial"    
    print "===================="
    print "Syncing to " + p4_10k_files_dir
    start = time.time()
    p4.run_sync("-f",p4_10k_files_dir)
    end = time.time()    
    elapsed = end - start
    print "===================="
    print "Time elapsed (s): {0:0.2f}".format(elapsed)
    print "===================="
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_10k_files_size_mb, elapsed))
    print "====================" 
    
    delete_10k_files()

##
def test_2c():

    print "===================="
    print "test_2c"    
    print "10000 * 16KB files"
    print "commit, --parallel=threads=100,batch=100,min=10"
    print "===================="

    sync_latest()
    print "===================="
           
    desc = "test_2c - adding 10000 * 16KB files, --parallel=threads=100,batch=100,min=10"
    cl = create_new_cl(desc)
    if cl == "0":
        quit()
            
    print "Adding " + p4_10k_files_dir
    ret = p4.run_add('-c',cl,p4_10k_files_dir)
    elapsed = submit_cl(cl, desc, 100, 100, 10)
    #elapsed = submit_cl(cl, desc, 0, 100, 10)
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_10k_files_size_mb, elapsed))
    print "===================="
   
##   
def test_2d():

    print "===================="
    print "test_2d"    
    print "10000 * 16KB files"
    print "sync, --parallel=threads=100,batch=100,min=10"
    print "===================="
    print "Syncing to " + p4_10k_files_dir
    start = time.time()
    p4.run_sync("--parallel=threads=100,batch=100,min=10","-f",p4_10k_files_dir)
    end = time.time()    
    elapsed = end - start
    print "===================="
    print "Time elapsed (s): {0:0.2f}".format(elapsed)
    print "===================="    
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_10k_files_size_mb, elapsed))
    print "===================="
  
##  
def test_2e():

    print "===================="
    print "test_2e"    
    print "10000 * 16KB files"
    print "sync, --parallel=threads=50,batch=100,min=10"
    print "===================="
    print "Syncing to " + p4_10k_files_dir
    start = time.time()
    p4.run_sync("--parallel=threads=50,batch=100,min=10","-f",p4_10k_files_dir)
    end = time.time()    
    elapsed = end - start
    print "===================="
    print "Time elapsed (s): {0:0.2f}".format(elapsed)
    print "===================="    
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_10k_files_size_mb, elapsed))
    print "===================="
  
##  
def test_2f():

    print "===================="
    print "test_2f"    
    print "10000 * 16KB files"
    print "sync, --parallel=threads=25,batch=100,min=10"
    print "===================="
    print "Syncing to " + p4_10k_files_dir
    start = time.time()
    p4.run_sync("--parallel=threads=25,batch=100,min=10","-f",p4_10k_files_dir)
    end = time.time()    
    elapsed = end - start
    print "===================="
    print "Time elapsed (s): {0:0.2f}".format(elapsed)
    print "===================="    
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_10k_files_size_mb, elapsed))
    print "===================="

##    
def test_2g():

    print "===================="
    print "test_2g"    
    print "10000 * 16KB files"
    print "sync, --parallel=threads=10,batch=100,min=10"
    print "===================="
    print "Syncing to " + p4_10k_files_dir
    start = time.time()
    p4.run_sync("--parallel=threads=10,batch=100,min=10","-f",p4_10k_files_dir)
    end = time.time()    
    elapsed = end - start
    print "===================="
    print "Time elapsed (s): {0:0.2f}".format(elapsed)
    print "===================="    
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_10k_files_size_mb, elapsed))
    print "===================="
 
## 
def test_2h():

    print "===================="
    print "test_2h"    
    print "10000 * 16KB files"
    print "sync, --parallel=threads=5,batch=100,min=10"
    print "===================="
    print "Syncing to " + p4_10k_files_dir
    start = time.time()
    p4.run_sync("--parallel=threads=5,batch=100,min=10","-f",p4_10k_files_dir)
    end = time.time()    
    elapsed = end - start
    print "===================="
    print "Time elapsed (s): {0:0.2f}".format(elapsed)
    print "===================="    
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_10k_files_size_mb, elapsed))
    print "===================="
    
## 
def test_2i():

    print "===================="
    print "test_2i"    
    print "10000 * 16KB files"
    print "sync, --parallel=threads=50,batch=200,min=10"
    print "===================="
    print "Syncing to " + p4_10k_files_dir
    start = time.time()
    p4.run_sync("--parallel=threads=50,batch=200,min=10","-f",p4_10k_files_dir)
    end = time.time()    
    elapsed = end - start
    print "===================="
    print "Time elapsed (s): {0:0.2f}".format(elapsed)
    print "===================="    
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_10k_files_size_mb, elapsed))
    print "===================="

## 
def test_2j():

    print "===================="
    print "test_2j"    
    print "10000 * 16KB files"
    print "sync, --parallel=threads=20,batch=500,min=10"
    print "===================="
    print "Syncing to " + p4_10k_files_dir
    start = time.time()
    p4.run_sync("--parallel=threads=20,batch=500,min=10","-f",p4_10k_files_dir)
    end = time.time()    
    elapsed = end - start
    print "===================="
    print "Time elapsed (s): {0:0.2f}".format(elapsed)
    print "===================="    
    print "Throughput (mb/s) : {0:0.2f}".format(get_throughput(os_10k_files_size_mb, elapsed))
    print "===================="
            
    delete_10k_files()

#------------------------------

# Find test to run
try:
    testfunc = globals()[sys.argv[1]]
except:
    print "Test has to be one of:"
    print "test_1a"
    print "test_1b"
    print "test_1c"
    print "test_1d"
    print "test_2a"
    print "test_2b"
    print "test_2c"
    print "test_2d"
    print "test_2e"
    print "test_2f"
    print "test_2g"
    print "test_2h"
    print "test_2i"
    print "test_2j"
    print "delete_large_file"
    print "delete_10k_files"
    quit()
    
# Main
try:
    # Connect + login
    ret = p4.connect()
    print ret
    ret = p4.run_login()
    #print ret

    # Switch client root relative to cwd
    client = p4.fetch_client( "-t", template_client )
    client._root = client_root    
    client._host = socket.gethostname()
    client._client = client._client + "_" + client._host
    #print client
    p4.save_client( client )
    p4.client = client._client           

    # Run "p4 info" (returns a dict)
    #info = p4.run( "info" )        
    #for key in info[0]:            # and display all key-value pairs
    #    print key, "=", info[0][key]
    
    print "************************************************************"
    print "===================="
    print "server time = " + get_server_time()    
    
    testfunc()
        
    # Disconnect from the server
    p4.disconnect()    
    print "************************************************************"
except P4Exception as ex:
    print ('Error: %s' % ex)
    for e in p4.errors:
        print e
    for e in p4.warnings:
        print e