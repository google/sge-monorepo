# Linux canadian cross toolchain generation procedure

## Introduction

If you are here you may be wondering how the linux toolchain is built and or maybe you want to
create a new one. This little helper tool creates a linux toolchain either from scratch or by
providing the sources of all the toolchain components in tar format (you can find them in a
previously built toolchain in /src/...). The expectation is that we're going to switch distributions
every two years or so.

## Prerequisites

The toolchain builder relies on a remote linux machine that can run docker containers, the builder
tool takes care of creating a machine as long as you have a valid GCP credential for the used
project. By default the credentials located in
"$env:AppData\Roaming\gcloud\legacy_credentials\$env:Username\adc.json" will be used, you
can provide another set of credentials using the -gcp-credentials argument, if you alread have a
working Google Cloud SDK installation, you probably already have the credentials.

If you need to get setup and never used GCP, you either create a new instance through the GCP
console by following [this guide](https://cloud.google.com/compute/docs/instances/create-start-instance),
or install the [Google Cloud SDK](go/softwarecenter) and follow
[this guide](https://cloud.google.com/sdk/docs/initializing) to initialize it.

## Using the builder helper

The builder is not distributed in binary format and is rather run through bazel, it relies on some
supporting files to create the toolchain (toolchain definition work file and such) those explicit
dependencies to the executable and are provided through runfiles.
If you have the necessary credentials you can run the following command to generate a toolchain

```powershell
bazel run --config=windows //tools/linux-toolchain-builder --enable_runfiles -- -vm-name=linux-toolchain-builder
```

Otherwise you can provide an already existing ssh host that has apt as a packet manager:

```powershell
bazel run --config=windows //tools/linux-toolchain-builder --enable_runfiles -- -ssh-host=x.x.x.x
```

It happens that the toolchain copy stays stuck, you can relaunch that specific step or a series of steps

```powershell
bazel run --config=windows //tools/linux-toolchain-builder --enable_runfiles -- -vm-name=linux-toolchain-builder -from-step=copytoolchainfromremote -to-step=copytoolchainfromremote
```

Note that when Unzipping the resulting toolchain, you'll have warnings about duplicate files, those
are not really duplicate files. Since Linux is a case sensitive OS, some header files only differ in
casing, windows treats them as the same. Choose Auto-rename to when unzipping so the files are still
there. We haven't encountered an issue with them yet, so if we do we will decide on a patching
strategy when building the toolchain.

When you are done with the toolchain and the machine used to create it, use the clean argument to
delete it

```powershell
bazel run --config=windows //tools/linux-toolchain-builder --enable_runfiles -- -vm-name=linux-toolchain-builder -clean
```

## Manual generation

This section helps you understand better the steps taken to generate a toolchain and are provided as
an extra helper if you need to tweak the toolchain and iterate faster while doing so.

#### In PowerShell:

NOTE: You can run them in Command Prompt, but will need to tweak some environment variables from \$var=x to %VAR%, etc.

### Creating a toolchain from scratch

- Create a VM either through the portal or by using the following command line:

```powershell
gcloud compute --project=$PROJECT instances create $env:USERNAME-linux-toolchain-creation-vm --zone=us-east4-c --machine-type=n1-standard-4 --subnet=default --network-tier=PREMIUM --metadata-from-file ssh-keys="$env:USERPROFILE/.ssh/google_compute_engine.pub" --maintenance-policy=MIGRATE --image-family=ubuntu-1804-lts --image-project=ubuntu-os-cloud --boot-disk-size=50GB --boot-disk-type=pd-standard --boot-disk-device-name=linux-toolchain-creation-vm --reservation-affinity=any
```

- Register the machine ip address in the following variable:

```powershell
$remoteIp=$(gcloud compute instances describe $env:USERNAME-linux-toolchain-creation-vm --format='get(networkInterfaces[0].accessConfigs.natIP)')
```

- Copy the content of this folder to the machine.

NOTE: (Internal Google only): Depending on how you created your ssh key, you may need to append `_google_com` to `$($env:USERNAME)` in all scp and ssh commands.

NOTE: (Internal Google only): You might need to set ssh configurations to make this work internally.
      See internal documentation.

```powershell
scp -i"$env:USERPROFILE/.ssh/google_compute_engine" -r .\docker "$($env:USERNAME)@$($remoteIp):~/xtools-build/"
```

- SSH to the machine

```powershell
ssh "$($env:USERNAME)@$remoteIp" -i"$env:USERPROFILE/.ssh/google_compute_engine"
```

- Add the following linux user that allows the Docker container instance to write a readable shared file

#### In ssh/bash:

```sh
sudo groupadd --gid 1024 buildgroup
sudo useradd --shell /bin/bash --no-create-home --gid buildgroup --uid 1024 builduser
```

- Create an output folder and set it's rights access right to the created user

```sh
cd ~/xtools-build/
mkdir output
sudo chown -R builduser:buildgroup output
```

- Install docker Pre-reqs:

```sh
set -x
set -eu
sudo apt-get update
sudo apt-get -y upgrade
sudo apt-get install apt-transport-https ca-certificates curl gnupg-agent software-properties-common
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
# Verify that you have the key with fingerprint 9DC8 5822 9FC7 DD38 854A  E2D8 8D81 803C 0EBF CD88
sudo apt-key fingerprint 0EBFCD88
# Should produce:
# pub   rsa4096 2017-02-22 [SCEA]
#      9DC8 5822 9FC7 DD38 854A  E2D8 8D81 803C 0EBF CD88
# uid           [ unknown] Docker Release (CE deb) <docker@docker.com>
# sub   rsa4096 2017-02-22 [S]
sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable"
```

- Install Docker

```sh
sudo apt-get update
sudo apt-get install docker-ce docker-ce-cli containerd.io
# Optional - test
sudo docker run hello-world
# Downloads new image and prints a "Hello from Docker!" message.
```

- Create the container

```sh
sudo docker build -t xtools .
```

- Run the container

```sh
sudo docker run -v $PWD/output:/xtools-build/output xtools
```

- If the output is valid, push the resulting image our registry. (These instructions are modelled off
[the container registry authentication docs](https://cloud.google.com/container-registry/docs/advanced-authentication)).
First, some setup:

```sh
# add yourself to the docker security group
sudo usermod -a -G ${USER}
# apply changes
sudo reboot
```

- This will obviously end your ssh session while the vm reboots. Give it a few minutes, then reconnect.

#### In PowerShell:

```powershell
ssh "$($env:USERNAME)@$remoteIp" -i"$env:USERPROFILE/.ssh/google_compute_engine"
```

#### In ssh/bash:

```sh
# Set "project" to your GCP project's id.
project=<your-gcp-projectid-here>
sudo docker tag xtools gcr.io/$project/xtools:tag
sudo docker push gcr.io/$project/xtools:tag
exit
```

#### In PowerShell:

- If you'd like to move this to a new machine later, copy back the resulting zip

```powershell
scp -i"$env:USERPROFILE/.ssh/google_compute_engine" "$($env:USERNAME)@$($remoteIp):~/xtools-build/output/toolchain.zip" toolchain.zip
```

### Rebuilding a toolchain from existing packages

To rebuild from an existing state, you'll need the original docker image tag used to generate the
toolchain, and the src packaged used during the generation:

```powershell
scp -i"$env:USERPROFILE/.ssh/google_compute_engine" -r path_to_src "$($env:USERNAME)@$remoteIp:~/xtools-build/src"
```

- SSH to the machine

```powershell
ssh "$($env:USERNAME)@$remoteIp" -i"$env:USERPROFILE/.ssh/google_compute_engine"
```

- Run the the script build script again

```sh
sudo chown -R builduser:buildgroup ~/xtools-build/src
sudo docker run -v $PWD/src:/xtools-build/src -v $PWD/output:/xtools-build/output xtools:tag
```

- Copy the resulting package back

```powershell
scp -i "$env:USERPROFILE/.ssh/google_compute_engine" "$($env:USERNAME)@$remoteIp:~/xtools-build/output/toolchain.zip" toolchain.zip
```
