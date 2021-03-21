The script covers the tests 1.A-D and 2.A-H from the test matrix.

The sub-tests are designed to be run one after another (as they depend on state of checked-in data) i.e. Test 1 must be run A to D.
Test 2 can be run independantly from Test 1 though.

Pre-requisite is to unpack and place data needed for the tests into a "workspace" folder.
The pathing should look like :

<git repo root>\SGE\pongpoc\P4_loadtest\10GB\workspace\large_files_10k\10k - location of unpacked file(s) from '10k.zip'
<git repo root>\SGE\pongpoc\P4_loadtest\10GB\workspace - location of 'large_files_big_station.dat'

Usage is to run for eg. 'python test_run.py test_1a'
