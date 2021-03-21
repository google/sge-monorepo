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

from __future__ import print_function

import subprocess
import sys

def read_config(filename):
  config = {}
  with open(filename) as f:
    for line in f.readlines():
      parts = line.strip().split('=')
      if len(parts) == 2:
        config[parts[0].strip()] = parts[1].strip()
  return config

def main(argv):
  if len(argv) != 4:
    raise SystemExit('Expecting 3 arguments')

  config=read_config(argv[1])
  server_id = argv[2].split('.')[0]
  cl_number = argv[3]

  if server_id not in config:
    raise SystemExit('Could not workspace for server %s' % server_id)
  
  workspace = config[server_id]
  print('Setting CL %s to restricted via %s' % (cl_number, workspace))

  cmd = ['p4', '-u', config['user'], '-c', workspace, 'change', '-t', 'restricted', '-f', cl_number]
  subprocess.Popen(cmd).wait()

main(sys.argv)