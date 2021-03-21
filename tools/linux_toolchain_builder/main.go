// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

var flags = struct {
	help bool
	fromStep string
	toStep string

	host string
	user string

	identity string

	vmName string
	vmProject string
	vmZone string

	gcpCredentials string
	outputDir string

	clean bool
}{}

// Those are default as of June 2020
// they can be changed using -project and -zone
const (
	defaultProject = "INSERT_PROJECT"
	defaultZone    = "INSERT_ZONE"
)

// Hardcoded values that we don't can change and check in
const (
	// Toolchain generation takes 70mins on 4 cores n1 machines
	machineType  = "n1-standard-4"
	imageFamily  = "ubuntu-1804-lts"
	imageProject = "ubuntu-os-cloud"
	bootDiskSize = 50
	// Tool location relative to the exec root to load the necessary files
	toolLocation = "./tool/linux-toolchain-builder"
	// This path needs to be in sync with one used in remote-commands.sh
	remoteLocation = "~/xtools-build"
)

type step int

// If you add a step, you need to update commandline default values
// and the string list represtation below
const (
	copyFilesToRemote step = iota
	runRemoteCommands
	copyToolchainFromRemote
	copyToOutputDir
	invalid
)

// No way to compile time get constants string representation
var stepStrings = [...]string{
	"copyFilesToRemote",
	"runRemoteCommands",
	"copyToolchainFromRemote",
	"copyToOutputDir",
	"invalid",
}

func (s step) string() string {
	return stepStrings[s]
}

func (s step) canRun(from step, to step) bool {
	return s >= from && s <= to
}

func stepFromString(s string) step {
	for pos, stepString := range stepStrings {
		if strings.EqualFold(s, stepString) {
			return step(pos)
		}
	}
	return invalid
}

func allValidSteps() string {
	return strings.Join(stepStrings[0:len(stepStrings)-1], ",")
}

func printfAndExit(format string, a ...interface{}) {
	fmt.Printf(format, a...)
	os.Exit(1)
}

type sshContext struct {
	host     string
	user     string
	identity string
}

func (sshContext *sshContext) print() {
	fmt.Println("-- Using GCE context")
	fmt.Printf("---- Remote: %s\n", buildSSHRemote(sshContext))
	fmt.Printf("---- Identity: %s\n", sshContext.identity)
}

func buildSSHRemote(sshContext *sshContext) string {
	return fmt.Sprintf("%s@%s", sshContext.user, sshContext.host)
}

func runSCPCommand(sshContext *sshContext, args []string) error {
	contextArgs := []string{
		fmt.Sprintf("-i%s", sshContext.identity),
	}
	cmd := exec.Command("scp", append(contextArgs, args...)...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println(cmd.String())

	return cmd.Run()
}

func runSSHCommand(sshContext *sshContext, commands string) error {
	contextArgs := []string{
		"-t",
		buildSSHRemote(sshContext),
		fmt.Sprintf("-i%s", sshContext.identity),
	}
	cmd := exec.Command("ssh", contextArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		printfAndExit("Failed getting acquiring a Stdin Pipe: %s", err)
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, commands)
	}()

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println(cmd.String())

	return cmd.Run()
}

type gceContext struct {
	vmName    string
	vmProject string
	vmZone    string
	ctx       context.Context
	service   *compute.Service
}

func (gceContext *gceContext) print() {
	fmt.Println("-- Using GCE context")
	fmt.Printf("---- VM Name: %s\n", gceContext.vmName)
	fmt.Printf("---- VM Project: %s\n", gceContext.vmProject)
	fmt.Printf("---- VM Zone: %s\n", gceContext.vmName)
}

func newGceContext(name string, project string, zone string, credFile string) (*gceContext, error) {
	ctx := context.Background()

	credsJSON, err := ioutil.ReadFile(credFile)
	if err != nil {
		return nil, fmt.Errorf("failed reading the credential file: %s", err)
	}

	creds, err := google.CredentialsFromJSON(ctx, credsJSON, compute.ComputeScope)
	if err != nil {
		return nil, fmt.Errorf("failed parsing the credentials json : %s", err)
	}

	client := oauth2.NewClient(ctx, creds.TokenSource)
	computeService, err := compute.New(client)
	if err != nil {
		return nil, fmt.Errorf("failed creating a compute service instance : %s", err)
	}

	return &gceContext{name, project, zone, ctx, computeService}, nil
}

func instanceIP(gceContext *gceContext) (string, error) {
	instance, err := gceContext.service.Instances.Get(gceContext.vmProject,
		gceContext.vmZone, gceContext.vmName).Context(gceContext.ctx).Do()
	if err != nil {
		return "", fmt.Errorf("failed getting instance %s: %s", gceContext.vmName, err)
	}

	// We create an external interface that nats to the local Ip
	// so we expect one
	if len(instance.NetworkInterfaces) == 0 ||
		len(instance.NetworkInterfaces[0].AccessConfigs) == 0 {
		return "", fmt.Errorf("instance %s has no public interface", gceContext.vmName)
	}

	return instance.NetworkInterfaces[0].AccessConfigs[0].NatIP, nil
}

func createInstance(gceContext *gceContext, identity string) error {
	pubKey, err := ioutil.ReadFile(identity + ".pub")
	if err != nil {
		return fmt.Errorf("failed reading your identity public key: %s", err)
	}
	pubKeyStr := string(pubKey)

	// first get the image url
	image, err := gceContext.service.Images.GetFromFamily(imageProject, imageFamily).Context(gceContext.ctx).Do()
	if err != nil {
		return fmt.Errorf("failed getting an image %s/%s: %s", imageProject, imageFamily, err)
	}
	insetOp, err := gceContext.service.Instances.Insert(gceContext.vmProject, gceContext.vmZone, &compute.Instance{
		Name:        gceContext.vmName,
		MachineType: fmt.Sprintf("zones/%s/machineTypes/%s", gceContext.vmZone, machineType),
		Disks: []*compute.AttachedDisk{
			{
				Boot:       true,
				AutoDelete: true,
				InitializeParams: &compute.AttachedDiskInitializeParams{
					DiskSizeGb:  bootDiskSize,
					SourceImage: image.SelfLink,
				},
			},
		},
		Metadata: &compute.Metadata{
			Items: []*compute.MetadataItems{
				{
					Key:   "ssh-keys",
					Value: &pubKeyStr,
				},
			},
		},
		NetworkInterfaces: []*compute.NetworkInterface{
			{
				Network: "global/networks/default",
				AccessConfigs: []*compute.AccessConfig{
					{
						Type: "ONE_TO_ONE_NAT",
						Name: "external-nat",
					},
				},
			},
		},
	}).Context(gceContext.ctx).Do()
	if err != nil {
		return fmt.Errorf("failed inserting the new instance: %s", err)
	}

	_, err = gceContext.service.ZoneOperations.Wait(gceContext.vmProject,
		gceContext.vmZone, insetOp.Name).Context(gceContext.ctx).Do()
	if err != nil {
		return fmt.Errorf("failed waiting on instance creation: %s", err)
	}
	return nil
}

func deleteInstance(gceContext *gceContext) error {
	op, err := gceContext.service.Instances.Delete(gceContext.vmProject,
		gceContext.vmZone, gceContext.vmName).Context(gceContext.ctx).Do()
	if err != nil {
		return fmt.Errorf("failed deleting the instance: %s", err)
	}
	_, err = gceContext.service.ZoneOperations.Wait(gceContext.vmProject,
		gceContext.vmZone, op.Name).Context(gceContext.ctx).Do()
	if err != nil {
		return fmt.Errorf("failed waiting on instance deletion: %s", err)
	}
	return nil
}

func main() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		printfAndExit("Error while getting the user home dir: %s", err)
	}

	flag.BoolVar(&flags.help, "help", false, "Display help")

	flag.StringVar(&flags.fromStep, "from-step", copyFilesToRemote.string(), "Run from step: "+allValidSteps())
	flag.StringVar(&flags.toStep, "to-step", copyToOutputDir.string(), "Run to step: "+allValidSteps())

	flag.StringVar(&flags.host, "ssh-host", "", "The SSH host if empty, we create a VM using new-vm-name")
	flag.StringVar(&flags.user, "ssh-user", os.Getenv("USERNAME"), "Remote user")

	flag.StringVar(&flags.identity, "ssh-identity", homeDir+"/.ssh/google_compute_engine", "Identity cert base file name")

	flag.StringVar(&flags.vmName, "vm-name", "", "New VM Name to be used")
	flag.StringVar(&flags.vmProject, "vm-project", defaultProject, "VM project")
	flag.StringVar(&flags.vmZone, "vm-zone", defaultZone, "VM zone")

	flag.StringVar(&flags.gcpCredentials, "gcp-credentials", filepath.Join(os.Getenv("APPDATA"),
		"gcloud", "legacy_credentials", os.Getenv("USERNAME"), "adc.json"), "GCP credentials json")

	flag.StringVar(&flags.outputDir, "output-dir", homeDir+"/Desktop", "Output directory")

	flag.BoolVar(&flags.clean, "clean", false, "Clean the VM")

	flag.Parse()

	if flags.help {
		flag.Usage()
		os.Exit(0)
	}

	fromStep := stepFromString(flags.fromStep)
	if fromStep == invalid {
		printfAndExit("Invalid 'to' step %s, valid steps are: %s", flags.fromStep, allValidSteps())
	}

	toStep := stepFromString(flags.toStep)
	if toStep == invalid {
		printfAndExit("Invalid 'from' step %s, valid steps are: %s\n", flags.toStep, allValidSteps())
	}

	if fromStep > toStep {
		printfAndExit("'from' step higher than 'to' step")
	}

	sshContext := sshContext{flags.host, flags.user, flags.identity}

	if sshContext.host != "" && flags.vmName != "" {
		fmt.Println("Bot an SSH host and and a new VM name was presented, discarding vm name")
	} else if sshContext.host == "" && flags.vmName == "" {
		printfAndExit("You need to either present an ssh host or a new vm name")
	}

	// If no host is provided, we get the host form the GCE
	if sshContext.host == "" {
		gceContext, err := newGceContext(flags.vmName, flags.vmProject, flags.vmZone, flags.gcpCredentials)
		if err != nil {
			printfAndExit("Failed creating the GCE context : %s", err)
		}

		gceContext.print()

		if flags.clean {
			fmt.Println("Deleting the vm instance")
			if err = deleteInstance(gceContext); err != nil {
				printfAndExit("Failed deleting %s: %s", gceContext.vmName, err)
			}
			fmt.Println("VM instance successfully deleted")
			os.Exit(0)
		}

		sshContext.host, err = instanceIP(gceContext)

		// Error fetching the instance info, instance probably not existant
		// Trying to create it
		if err != nil {
			fmt.Println("Creating a vm instance to build the toolchain")

			if err = createInstance(gceContext, sshContext.identity); err != nil {
				printfAndExit("Failed creating the vm instance : %s", err)
			}

			fmt.Println("VM instance successfully created")

			sshContext.host, err = instanceIP(gceContext)
			if err != nil {
				printfAndExit("Failed to get the host of the newly created instance : %s", err)
			}

			// Safety wait an additional 30s to have the machine reachable
			const nbrSeconds = 30
			for i := 0; i < nbrSeconds; i++ {
				fmt.Printf("Waiting %2d seconds\r", nbrSeconds-i)
				time.Sleep(time.Second)
			}
		}
	}

	sshContext.print()

	if copyFilesToRemote.canRun(fromStep, toStep) {
		fmt.Println("Copying files to the remote instance")
		err = runSCPCommand(&sshContext, []string{
			"-r",
			toolLocation + "/docker",
			fmt.Sprintf("%s:%s", buildSSHRemote(&sshContext), remoteLocation+"/"),
		})
		if err != nil {
			printfAndExit("Failed to copy docker files: %s\n", err)
		}
	}

	if runRemoteCommands.canRun(fromStep, toStep) {
		fmt.Println("Running remote-commands.sh")
		remoteCommands, err := ioutil.ReadFile(toolLocation + "/remote-commands.sh")
		if err != nil {
			printfAndExit("Failed to read remote commands: %s\n", err)
		}
		err = runSSHCommand(&sshContext, string(remoteCommands))
		if err != nil {
			printfAndExit("Failed to run remote commands: %s\n", err)
		}
	}

	toolchainZip := toolLocation + "/toolchain.zip"
	if copyToolchainFromRemote.canRun(fromStep, toStep) {
		fmt.Println("Copying toolchain from remote")
		err = runSCPCommand(&sshContext, []string{
			fmt.Sprintf("%s:%s", buildSSHRemote(&sshContext), remoteLocation+"/output/toolchain.zip"),
			toolchainZip,
		})
		if err != nil {
			printfAndExit("Failed to copy the resulting toolchain: %s\n", err)
		}
	}

	if copyToOutputDir.canRun(fromStep, toStep) {
		fmt.Println("Copying toolchain to its final location")
		err = os.Rename(toolchainZip, flags.outputDir + "\\toolchain.zip")
		if err != nil {
			printfAndExit("Failed to copy the resulting toolchain: %s\n", err)
		}
	}
}
