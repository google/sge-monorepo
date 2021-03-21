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

# Dockerfile image to build the a linux toolchain for windows.

FROM ubuntu:18.04

# Alternative images that can be used
#FROM launcher.gcr.io/google/debian9:latest
#FROM launcher.gcr.io/google/ubuntu18_04

# This fixes an issue with debconf when you try running this file
# ref https://github.com/phusion/baseimage-docker/issues/58#issuecomment-47995343
RUN echo 'debconf debconf/frontend select Noninteractive' | debconf-set-selections

# Installing all the new
RUN apt-get update && \
  apt-get dist-upgrade -y && \
  apt-get install -y build-essential wget curl autoconf bison flex texinfo help2man gawk libtool libtool-bin libncurses5-dev python3-dev python3-distutils git mingw-w64 unzip zip && \
  rm -rf /var/lib/apt/lists/*

ENV BUILD_PATH /xtools-build
ENV CROSSTOOL_NG_DIR crosstool-ng

WORKDIR $BUILD_PATH/$CROSSTOOL_NG_DIR

RUN git clone http://github.com/crosstool-ng/crosstool-ng . -b crosstool-ng-1.24.0 && \
  ./bootstrap && ./configure --enable-local && make -j$(nproc)

WORKDIR $BUILD_PATH

COPY build.sh .
COPY crosstool.config .

RUN groupadd --gid 1024 buildgroup && \
  useradd --shell /bin/bash --no-create-home --gid buildgroup --groups sudo --uid 1024 builduser && \
  chown -R builduser:buildgroup $BUILD_PATH && \
  chmod +x ./build.sh

USER builduser

CMD ["./build.sh"]
