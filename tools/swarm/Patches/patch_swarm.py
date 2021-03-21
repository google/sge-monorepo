#!/usr/bin/env python3
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


import hashlib
import json
import os
import sys

def get_md5sum(path):
  md5_processor = hashlib.md5()
  with open(path, 'rb') as f:
    for file_chunk in iter(lambda: f.read(4096), b''):
      md5_processor.update(file_chunk)
  return md5_processor.hexdigest()

def load_patch_definitions(script_dir):
  patch_definitions = {}
  for entry in os.listdir(script_dir):
    json_path = os.path.join(script_dir, entry, 'patches.json')
    if os.path.isfile(json_path):
      with open(json_path) as f:
        patch_definition = json.load(f)
        patch_definition['base_dir'] = os.path.join(script_dir, entry)
        patch_definitions[patch_definition['swarm_release'] + patch_definition['swarm_patch']] = patch_definition
        
  return patch_definitions

def load_swarm_version(swarm_dir):
  version_path = os.path.join(swarm_dir, 'Version')
  if not os.path.isfile(version_path):
    return None, None
  
  swarm_release = None
  swarm_patch = None

  with open(version_path) as f:
    for line in f:
      parts = line.strip().split('=')
      if len(parts) == 2:
        value = parts[1][:-1].strip()
        if parts[0].strip() == 'RELEASE':
          swarm_release = value
        elif parts[0].strip() == 'PATCHLEVEL':
          swarm_patch = value
  return swarm_release, swarm_patch

def apply_patch_definition(swarm_dir, script_dir, patch_definition):
  for file in patch_definition['files']:
    target_path = os.path.join(swarm_dir, file['path'])
    patch_path = os.path.join(patch_definition['base_dir'], file['patch'])
    original_md5 = file['original_md5']
    new_md5 = file['new_md5']
    if not os.path.isfile(target_path):
      print('File %s does not exist' % target_path)
      continue
    if not os.path.isfile(patch_path):
      print('File %s does not exist' % patch_path)
      continue

    md5sum = get_md5sum(target_path)
    if md5sum == original_md5:
      print('%s matches original hash, will be patching' % target_path)
    elif md5sum == new_md5:
      print('%s matches new hash, skipping' % target_path)
      # Note: if we didn't skip, we the patch would reverse which could be useful for undoing the changes.
      continue
    else:
      print('WARNING: Unexpected hash %s for %s, skipping' % (md5sum, target_path))
    os.system('patch "%s" "%s"' % (target_path, patch_path))
    md5sum = get_md5sum(target_path)
    if md5sum != new_md5:
      print('ERROR: Expected hash %s after patching but got %s instead for %s' % (new_md5, md5sum, target_path))

def main(args):

  if len(args) == 1:
    print('Usage: patch_swarm.py [PATH TO SWARM ROOT]')
    print('       Example: patch_swarm.py /opt/perforce/swarm')
    sys.exit(1)

  swarm_dir = args[1]

  if not os.path.isdir(swarm_dir):
    print('Error: %s does not exist' % swarm_dir)
    sys.exit(1)

  script_dir = os.path.dirname(args[0])
  if not script_dir:
    script_dir = '.'
  patch_definitions = load_patch_definitions(script_dir)

  swarm_release, swarm_patch = load_swarm_version(swarm_dir)
  if not swarm_release or not swarm_patch:
    print('Unable to detect Swarm version from %s' % swarm_dir)
    sys.exit(1)

  print('Detected Swarm release %s patch %s' % (swarm_release, swarm_patch))
  patch_key = swarm_release + swarm_patch
  
  if patch_key not in patch_definitions:
    print('Cannot find a matching patch definition. Available keys: %s' % patch_definitions.keys())
    sys.exit(1)

  patch_definition = patch_definitions[patch_key]
  apply_patch_definition(swarm_dir, script_dir, patch_definition)

main(sys.argv)