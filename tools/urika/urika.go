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

// Binary urika is a application URI redirection system that redirects messages to SG&E apps
// this tool is installed as application URI to handle any external protocol with the SGE:// prefix
// appropriate app will be launched and arguments passed
// this enables deep link to SG&E applications and tools via URI

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/urika"
	"sge-monorepo/libs/go/windows_utils"

	"github.com/golang/glog"
)

const perculatorVersion = "0.0.2"

// dispatches a message to application containin URI path
func sendMessage(u *url.URL) error {
	glog.Infof("sending message: %v", u)
	appName := u.Host
	req, err := json.Marshal(u.String())
	if err != nil {
		return fmt.Errorf("failed to marshal json for post: %v", err)
	}
	resp, err := http.Post("http://localhost:8080/"+appName, "application/json", bytes.NewBuffer(req))
	if err != nil {
		return fmt.Errorf("failed to post: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("response err")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http bad status: %d", resp.StatusCode)
	}
	fmt.Println(string(body))
	return nil
}

// launch the intended receiver application
// application is assumed to be in same directory as urika (//shared/bin)
func launchApp(u *url.URL, parent string) error {
	dir := filepath.Dir(parent)

	extension := ""
	switch runtime.GOOS {
	case "darwin":
		extension = ".app"
	case "windows":
		extension = ".exe"
	}

	appPath := filepath.Join(dir, u.Hostname()+extension)
	if _, err := os.Stat(appPath); err != nil {
		return fmt.Errorf("app does not exist: %s :%v", appPath, err)
	}
	com := exec.Command(appPath, "-uri="+u.String())
	glog.Infof("launching app: %v", com)
	com.Dir = dir
	return com.Start()
}

func run() (int, error) {
	exe, err := os.Executable()
	if err != nil {
		return 1, fmt.Errorf("couldn't ascertain executable")
	}

	if err := urika.AppUriKeyInstall("sge", exe); err != nil {
		if windows_utils.IsAdmin() {
			glog.Errorf("failed to install to registry: %v", err)
		} else {
			glog.Warningf("failed to install to registry: %v - will re-try in administrator mode", err)
			windows_utils.RunElevatedShellCommand(exe, "", true)
		}
	}
	if len(os.Args) < 2 {
		return 2, fmt.Errorf("usage: urika <uri>")
	}
	u, err := url.Parse(os.Args[1])
	if err != nil {
		return 3, fmt.Errorf("couldn't parse URI %s: %v", os.Args[1], err)
	}
	if err := sendMessage(u); err != nil {
		fmt.Println(err)
		if err = launchApp(u, exe); err != nil {
			return 4, fmt.Errorf("failed to launch app %s: %v", u, err)
		}
	}
	return 0, nil
}

// application is invoked with uri as first argument
func main() {
	// glog to both stderr and to file
	// Note: for stderr logging to work, remove the -H=windowsgui linker flag from BUILD
	flag.Set("alsologtostderr", "true")
	flag.Set("stderrthreshold", "INFO")
	if ad, err := files.GetAppDir("sge", "urika"); err == nil {
		// set directory for glog to %APPDATA%/sge/urika
		flag.Set("log_dir", ad)
	}
	flag.Parse()
	glog.Info("application start")
	glog.Infof("%v", os.Args)
	exitCode, err := run()
	if err != nil {
		glog.Error(err)
	}
	glog.Info("application exit")
	glog.Flush()
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}
