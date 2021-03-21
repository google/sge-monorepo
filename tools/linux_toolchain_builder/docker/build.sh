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

# We're expecting to output files relative to the script location
SCRIPT_PATH="$(cd "$(dirname "${BASH_SOURCE[0]}")" >/dev/null 2>&1 && pwd)"
cd $SCRIPT_PATH

# Generating a complete .config file if one is not provided
if test -f "./src/.config"; then
    cp ./src/config .config
else
    DEFCONFIG=crosstool.config ./crosstool-ng/ct-ng defconfig
fi

OUTPUT="./output"
TOOLCHAIN="$OUTPUT/toolchain"
TOOLCHAIN_SRC="$TOOLCHAIN/src"

# Copy the config file so we can repro this build
mkdir -p $TOOLCHAIN_SRC
cp .config $TOOLCHAIN_SRC/.config

# Building the cross compiling tooling
./crosstool-ng/ct-ng build.$(nproc)

chmod -R u+w,o+r build

TOOLCHAIN_RAW="$OUTPUT/toolchain_raw"
TARGET_TOOLCHAIN="x86_64-unknown-linux-gnu"
mkdir -p $TOOLCHAIN_RAW

# Copying the build folder and removing symlinks
cp -r -L build/* $TOOLCHAIN_RAW

# Flatten the toolchain so we can point to it easily with --sysroot=path
cp -r $TOOLCHAIN_RAW/bin $TOOLCHAIN
cp -r $TOOLCHAIN_RAW/include $TOOLCHAIN
cp -r $TOOLCHAIN_RAW/lib $TOOLCHAIN
cp -r $TOOLCHAIN_RAW/libexec $TOOLCHAIN
cp -r $TOOLCHAIN_RAW/share $TOOLCHAIN
cp -r $TOOLCHAIN_RAW/$TARGET_TOOLCHAIN/bin $TOOLCHAIN
cp -r $TOOLCHAIN_RAW/$TARGET_TOOLCHAIN/include $TOOLCHAIN
cp -r $TOOLCHAIN_RAW/$TARGET_TOOLCHAIN/lib $TOOLCHAIN
cp -r $TOOLCHAIN_RAW/$TARGET_TOOLCHAIN/lib64 $TOOLCHAIN
cp -r $TOOLCHAIN_RAW/$TARGET_TOOLCHAIN/sysroot/* $TOOLCHAIN

# Add the source tarballs and the config file so we can repro this build
cp .build/tarballs/* $TOOLCHAIN_SRC/

# Zip all artifacts
pushd $OUTPUT
zip -r toolchain_raw.zip ./toolchain_raw/
zip -r toolchain.zip ./toolchain/
popd
