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

package rust

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/tools/vendor_bender/protos/metadatapb"

	"github.com/BurntSushi/toml"
)

var cargoToml = `[package]
name = "fake_lib"
version = "0.1.0"
edition = "2018"

[lib]
path = "fake_lib.rs"

[dependencies]
`

var crateExcludes = map[string]bool{
	"winapi_i686_pc_windows_gnu":   true,
	"winapi_x86_64_pc_windows_gnu": true,
}

type cargoInfo struct {
	Package packageInfo
}

type packageInfo struct {
	Name       string
	Version    string
	Repository string
}

// compares two version number l and r, in format "x.y.z"
// if l > r -> return 1
// if l < r -> return -1
// if l == r -> return 0
func compareVersions(l string, r string) int {
	lArr := strings.Split(l, ".")
	rArr := strings.Split(r, ".")

	i := 0
	for i < len(lArr) && i < len(rArr) {
		if lArr[i] > rArr[i] {
			return 1
		} else if lArr[i] < rArr[i] {
			return -1
		}
		i++
	}

	if len(lArr) == len(rArr) {
		return 0
	} else if len(lArr) > len(rArr) {
		return 1
	} else {
		return -1
	}
}

func PrintRustPkgUsage() {
	fmt.Println("  rust-pkg crate-name@version")
	fmt.Println("    vendor the given rust package and its dependencies given its name and version")
	fmt.Println("    -upgrade: Upgrade the package and its dependencies")
}

var rustDir = ""

func execCargo(wd string, verbose bool, args ...string) error {
	if rustDir == "" {
		var err error
		if rustDir, err = findRustDir(); err != nil {
			fmt.Errorf("failed to find local rust toolchain: %v", err)
		}
	}
	cmd := exec.Command(fmt.Sprintf("%s\\cargo.exe", rustDir), args...)
	cmd.Env = []string{fmt.Sprintf("Path=%s", rustDir)} // cargo uses PATH to find the cargo subcommands, eg. "cargo raze"
	cmd.Dir = wd
	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	}
	return cmd.Run()
}

func findRustDir() (string, error) {
	p4 := p4lib.New()
	rustDir, err := p4.Where("//sge/third_party/toolchains/rust")
	if err != nil {
		return "", err
	}
	rustVersionedDirs, err := filepath.Glob(rustDir + "\\*")
	if err != nil {
		return "", err
	}
	latestVersion := ""
	for _, rustVersionedDir := range rustVersionedDirs {
		if _, err := os.Stat(rustVersionedDir + "\\bin\\cargo.exe"); err == nil {
			version := filepath.Base(rustVersionedDir)
			if latestVersion == "" || compareVersions(version, latestVersion) == 1 {
				latestVersion = version
			}
		}
	}
	if latestVersion == "" {
		return "", fmt.Errorf("no valid rust toolchain")
	}
	return rustDir + "\\" + latestVersion + "\\bin", nil
}

func crateProcess(pkgPath, ver, dst string) (*metadatapb.Metadata, error) {
	packageSrcDir := pkgPath + "-" + ver
	pb := filepath.Dir(pkgPath)
	pf := filepath.Base(pkgPath)
	// convert kebab-case to snake_case
	pf = strings.ReplaceAll(pf, "-", "_")
	packageDir := filepath.Join(pb, pf)

	packageName := filepath.Base(packageDir)
	if _, ok := crateExcludes[packageName]; ok {
		fmt.Println("excluded package", packageName)
		return &metadatapb.Metadata{}, nil
	}

	if packageDir != packageSrcDir {
		err := os.Rename(packageSrcDir, packageDir)
		if err != nil {
			return &metadatapb.Metadata{}, fmt.Errorf("couldn't rename %s to %s %v", packageSrcDir, packageDir, err)
		}
	}

	buildFile := filepath.Join(packageDir, "BUILD")
	content, err := ioutil.ReadFile(buildFile)
	if err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("couldn't open build file %s %v", buildFile, err)
	}
	lines := strings.Split(string(content), "\n")
	var outLines []string
	for _, line := range lines {
		// raze output malformed paths that break bazel due to incorrect seperators, fix this
		newLine := strings.Replace(line, "src\\lib.rs", "src/lib.rs", 1)
		if matches := depRx.FindStringSubmatch(newLine); matches != nil {
			newLine = matches[1] + "@" + strings.ReplaceAll(matches[2], "-", "_") + "//" + matches[3]
		}
		outLines = append(outLines, newLine)
	}

	file, err := os.Create(buildFile)
	if err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("couldn't create build file %s %v", buildFile, err)
	}
	defer file.Close()
	for _, line := range outLines {
		file.WriteString(line + "\n")
	}

	if err = files.CopyDir(packageDir, dst); err != nil {
		fmt.Printf("couldn't copy directory %s -> %s %v\n", packageDir, dst, err)
		return &metadatapb.Metadata{}, nil
	}

	cargoFile := filepath.Join(dst, "Cargo.toml")
	cargoData, err := ioutil.ReadFile(cargoFile)
	if err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("failed to read Cargo.toml: %v", err)
	}
	var cargo cargoInfo
	if _, err := toml.Decode(string(cargoData), &cargo); err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("failed to parse Cargo.toml data: %v", err)
	}

	metadata := &metadatapb.Metadata{
		Name: cargo.Package.Name,
		ThirdParty: &metadatapb.ThirdParty{
			Source: &metadatapb.Source{
				RustPkg: &metadatapb.RustPkg{
					Url:     cargo.Package.Repository,
					Version: cargo.Package.Version,
				},
			},
		},
	}

	return metadata, nil
}

// use raze to obtain crate and all transitive dependencies
func razeCrate(tempDir string, pkg string, version string, verbose bool) error {
	// build Cargo.toml file that references required crate
	toml := cargoToml
	toml += fmt.Sprintf("%s = \"%s\"\n\n", pkg, version)
	toml += "[raze]\n"
	toml += "workspace_path = \"//third_party/rust\"\n"
	toml += "target = \"x86_64-pc-windows-msvc\"\n"
	toml += "default_gen_buildrs = true\n"

	fpath := filepath.Join(tempDir, "Cargo.toml")
	if err := ioutil.WriteFile(fpath, []byte(toml), os.ModePerm); err != nil {
		return fmt.Errorf("couldn't write cargo.toml.file %v", err)
	}

	dir := tempDir

	// generate lock file Cargo.lock for the package
	if err := execCargo(dir, verbose, "generate-lockfile"); err != nil {
		return err
	}

	// Retrieve all the dependencies with up-to-date lock file
	if err := execCargo(dir, verbose, "vendor", "--versioned-dirs", "--locked"); err != nil {
		return err
	}

	// generate Bazel BUILD for rust crates
	if err := execCargo(dir, verbose, "raze"); err != nil {
		return err
	}

	return nil
}

// regex to extract package name from semantic version
// (packagename)-major.minor.patch
// example:
// memchr-1.3.0
// (memchr)
var packageRx = regexp.MustCompile(`^([^\.]+)-(\d+.\d+.\d+)`)

// regex to extract dependency path that raze creates
// (prelude)vendor(packagename)semantic.version(suffix)
// example:
//         "//third_party/rust/vendor/winapi-x86_64-pc-windows-msvc-0.4.0:winapi_x86_64_pc_windows_msvc",
// (         "//third_party/rust/)(winapi-x86_64-pc-windows-msvc)(:winapi_x86_64_pc_windows_msvc",)
var depRx = regexp.MustCompile(`(^\s+").*\/vendor\/([^\.]+)-\d+.\d+.\d+(:.*)`)

func RustPkg(name, versionSpec, dst string, verbose bool) (*metadatapb.Metadata, error) {
	// argument expected is crate@version
	// create a temp directory for vendoring
	tempDir, err := ioutil.TempDir("", "rust_vendor_*")
	if err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("couldn't create temp dir %v", err)
	}
	defer os.RemoveAll(tempDir)

	if err := razeCrate(tempDir, name, versionSpec, verbose); err != nil {
		return &metadatapb.Metadata{}, fmt.Errorf("fail to obtain crate: %v", err)
	}

	dirs, err := filepath.Glob(filepath.Join(tempDir, "vendor", "*"))

	for _, d := range dirs {
		var packagePath string
		var curVersion string
		if matches := packageRx.FindStringSubmatch(d); matches != nil {
			packagePath = matches[1]
			curVersion = matches[2]
			if path.Base(packagePath) == name {
				return crateProcess(packagePath, curVersion, dst)
			}
		}
	}

	return &metadatapb.Metadata{}, fmt.Errorf("internal error: package not found")
}
