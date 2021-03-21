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

package envinstall

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func (m manager) SyncAndInstallDependencies() error {
	// Sync the dependencies.
	output, err := m.p4.Sync([]string{"//sge/environment/data/..."}, "-f")
	if err != nil {
		return err
	}
	fmt.Println(output)

	if err := m.installDependencies(); err != nil {
		return err
	}
	return m.writeCurrentVersion()
}

// Returns the command for further processing in case of errors.
func (m manager) installDependencies() error {
	if err := m.installVSDependencies(); err != nil {
		return fmt.Errorf("could not install Visual Studio dependencies: %v", err)
	}
	if err := m.installUnrealPrereqs(); err != nil {
		return fmt.Errorf("could not install unreal dependencies: %v", err)
	}
	if err := m.installVCRedist(); err != nil {
		return fmt.Errorf("could not install VC Redistributable: %v", err)
	}
	if runningInCI() {
		// TODO(b/164400783): Stackdriver could be messing up with CI machines, evaluate over some
		//                    period of time if this is correct.
		// if err := m.installStackDriverMonitoring(); err != nil {
		// 	return fmt.Errorf("could not install Stack Driver monitoring: %v", err)
		// }
	}
	return nil
}

func (m manager) installVSDependencies() error {
	fmt.Println("Installing Visual Studio dependencies...")
	args := []string{
		m.asDataPath("vs_buildtools.exe"),
		"--quiet", "--wait", "--norestart", "--nocache",
		"--installPath", `C:\Toolchain\BuildTools`,
		"--channelUri", m.asDataPath("VisualStudio.chman"),
		"--installChannelUri", m.asDataPath("VisualStudio.chman"),
		"--add", "Microsoft.Net.Component.4.6.2.SDK",
		"--add", "Microsoft.Net.Component.4.6.2.TargetingPack",
		"--add", "Microsoft.VisualStudio.Component.NuGet",
	}

	result := m.runAndWait(args...)

	// Sadly VS dependencies are noisy, we have to allowlist which errors are actually errors.
	switch result.ExitCode {
	case 0:
		return nil
	case 1602: // Cancelled.
		fmt.Print(result.Out)
		return result.Err
	default:
		fmt.Println("Got exit code:", result.ExitCode)
		return nil
	}
}

func (m manager) installUnrealPrereqs() error {
	fmt.Println("Installing Unreal dependencies...")
	args := []string{
		m.asDataPath("UE4PrereqSetup_x64.exe"),
		"/install", "/quiet", "/norestart", `/log=C:\Toolchain\install-ue-prereq.log`}
	result := m.runAndWait(args...)
	fmt.Print(result.Out)
	return result.Err
}

func (m manager) installVCRedist() error {
	fmt.Println("Install VC Redistributables...")
	args := []string{
		m.asDataPath("vc_redist.x64.exe"),
		"/install", "/quiet", "/norestart", "/log", `C:\Toolchain\vc_install.log`}
	result := m.runAndWait(args...)

	switch result.ExitCode {
	case 1638: // could not install because newer installation exists.
		result.Err = nil
	}
	fmt.Print(result.Out)
	return result.Err
}

func (m manager) installStackDriverMonitoring() error {
	fmt.Println("Installing Stack Driver monitoring...")
	if m.isStackDriverInstalled() {
		return nil
	}
	args := []string{m.asDataPath("StackdriverMonitoring-GCM-46.exe"), "/S"}
	result := m.runAndWait(args...)
	if result.Err != nil {
		fmt.Println(result.Out)
		return result.Err
	}
	// We wait until the process appears as installed.
	timeout := make(chan bool, 1)
	go func() {
		time.Sleep(30 * time.Second)
		timeout <- true
	}()

	for {
		fmt.Println("Checking for running Stack Driver daemon...")
		if m.isStackDriverInstalled() {
			fmt.Println("Daemon found!")
			break
		}
		// Check if the timeout has happened.
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for service")
		default:
		}
		time.Sleep(5 * time.Second)
	}
	return nil
}

func (m manager) isStackDriverInstalled() bool {
	r := m.runAndWait("powershell", "Get-Service", "-Name", "StackdriverMonitoring")
	if strings.Contains(r.Out, "Running") {
		return true
	}
	return false
}

func (m manager) asDataPath(file string) string {
	return filepath.Join(m.data, file)
}

// cmdResult represents the execution result of a subprocess.
type cmdResult struct {
	ExitCode int
	Err      error
	Out      string
}

func (m manager) runAndWait(args ...string) cmdResult {
	cmdArgs := []string{"start", "/wait", "/C"}
	cmdArgs = append(cmdArgs, args...)

	fmt.Println(cmdArgs)
	cmdExe := filepath.Join(m.sysroot, "System32", "cmd.exe")
	cmd := exec.Command(cmdExe, cmdArgs...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return cmdResult{
		ExitCode: cmd.ProcessState.ExitCode(),
		Err:      err,
		Out:      buf.String(),
	}
}

// runningInCI verifies that we can get a secret from the environment in which CI runners run.
func runningInCI() bool {
	env, err := Environment()
	if err != nil {
		return false
	}
	// TODO: We're gating this to the dev environment first.
	//       Note that this will require another bump of the current version value,
	//       as the prod machines will (rightfully) think they're up to date.
	return env == CiDev
}
