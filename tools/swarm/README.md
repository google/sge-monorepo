# Swarm Tools

## Patches

Allows patching the Swarm source code in a safe way.

### Generating the patch

1. Ensure a subdirectory exists for every version of Swarm you would like to patch.
   The directory would need a patches.json file that defines the target version
   and the list of files to patch.

2. For each file that needs patching:
   - Run md5sum to record the checksum of the original file
   - Copy the original file somewhere
   - Modify the file as needed
   - Run md5sum to record the checksum of the modified file
   - Run "diff -u" between the original file and the modified file and save the diff to a file
   - Optional: clean up the paths in the diff
   - Create an entry in patches.json for the file

See existing patches.json files for examples.

### Patching Swarm

1. Copy the contents of the Patches folder to the Swarm VM
2. Run patch_swarm.py on the Swarm server, passing in the Swarm root path as an argument
   For example:
  ```
  ./patch_swarm.py /opt/perforce/swarm
  ```
  
  Tip: Make sure to run this as admin or with sudo!
