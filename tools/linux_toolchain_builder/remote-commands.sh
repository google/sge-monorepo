#!/bin/bash
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


set -x
set -eu

# install docker and the google cloud sdk
sudo apt-get update
sudo apt-get upgrade -y
sudo apt-get install -y docker.io google-cloud-sdk

# These group Ids needs to be in sync with the Dockerfile
sudo groupadd --gid 1024 buildgroup
sudo useradd --shell /bin/bash --no-create-home --gid buildgroup --uid 1024 builduser

#keep in sync with the main.go remoteLocation constant
cd ~/xtools-build/

mkdir output
sudo chown -R builduser:buildgroup output
sudo docker build -t xtools .
sudo docker run -v $PWD/output:/xtools-build/output xtools
