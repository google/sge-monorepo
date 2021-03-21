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

// Binary inplace_p4_publisher will take a package and submit its contents to perforce
package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"strconv"
	"strings"

	"sge-monorepo/build/cicd/sgeb/buildtool"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
	"sge-monorepo/build/packagemanifest"
	"sge-monorepo/libs/go/p4lib"
	"sge-monorepo/libs/go/sgeflag"

	"github.com/golang/glog"
)

// publisherFlags contains the flags given to the publisher by sgeb
type publisherFlags struct {
	change      string
	submitCl    bool
	description sgeflag.StringList
	name        string
	packageBase string
}

func main() {
	var flags = publisherFlags{}
	defer glog.Flush()
	flag.StringVar(&flags.change, "c", "", "change to add files to. If omitted a new CL is created.")
	flag.BoolVar(&flags.submitCl, "submit_cl", false, "submits the CL after it is created")
	flag.Var(&flags.description, "desc", "additional lines of description to add to the CL")
	flag.StringVar(&flags.name, "name", "", "name of the package (added to CL)")
	flag.StringVar(&flags.packageBase, "package_base", "", "base of the package")
	flag.Parse()
	if err := publish(flags); err != nil {
		glog.Errorf("%v", err)
		os.Exit(1)
	}
}

func partition(domain []string, maxPartLen int, callback func(slicePart []string) error) error {
	nbPartitions := len(domain) / maxPartLen
	for i := 0; i < nbPartitions; i++ {
		begin := i * maxPartLen
		end := begin + maxPartLen
		if end > len(domain) {
			end = len(domain)
		}
		if err := callback(domain[begin:end]); err != nil {
			return err
		}
	}

	return nil
}

// publish implements the publisher logic
func publish(flags publisherFlags) error {
	helper := buildtool.MustLoad()
	packageBase, err := helper.ResolveBuildPath(flags.packageBase)
	if err != nil {
		return fmt.Errorf("cannot resolve out_dir %q: %v", flags.packageBase, err)
	}
	p4 := p4lib.New()
	changelist, err := makeChangelist(p4, helper, flags.name, flags.change, flags.description)
	if err != nil {
		return fmt.Errorf("error getting/creating changelist: %v", err)
	}
	glog.Info("changelist: ", changelist)

	// Search input artifacts for package manifests
	pkgBuilder := packagemanifest.NewPackageBuilder()
	for _, inputs := range helper.Invocation().Inputs {
		for _, input := range inputs.Artifacts {
			p, ok := buildtool.ResolveArtifact(input)
			if !ok {
				continue
			}
			if !packagemanifest.IsPkgArtifact(input) {
				return fmt.Errorf("non-package artifact found in inputs: %s", p)
			}
			if err := pkgBuilder.AddPkgManifest(p); err != nil {
				return err
			}
		}
	}
	var paths []string
	for _, e := range pkgBuilder.Entries() {
		paths = append(paths, path.Join(packageBase, e.PkgPath))
	}

	// p4.Reconcile could generate a command that is too long to execute if we were to give it all the paths in one call
	if err := partition(paths, 100, func(pathsSubset []string) error {
		for _, path := range pathsSubset {
			glog.Info(path)
		}
		reconcileResult, err := p4.Reconcile(pathsSubset, changelist)
		if err != nil {
			return fmt.Errorf("error reconciling %q: %v", pathsSubset, err)
		}
		glog.Info(reconcileResult)
		return nil
	}); err != nil {
		return err
	}
	descriptions, err := p4.Describe([]int{changelist})
	if err != nil {
		return fmt.Errorf("error describing changelist: %v", err)
	}
	glog.Info("flags.submitCl: ", flags.submitCl)
	if areAllChangelistDescriptionsEmpty(descriptions) {
		glog.Info("changelist empty, cleaning up")
		cleanupChange(p4, changelist)
	} else if flags.submitCl {
		glog.Info("submitting cl: ", changelist)
		if _, err := p4.Submit(changelist); err != nil {
			glog.Errorf("error in submit: %v", err)
			if err := cleanupChange(p4, changelist); err != nil {
				glog.Error(err)
			}
			return err
		}
	}
	if err := writePublishResult(paths, flags.name, helper); err != nil {
		return fmt.Errorf("error writing publish results: %v", err)
	}
	return nil
}

// areAllChangelistDescriptionsEmpty returns false if any of the description constains an open file, false otherwise
func areAllChangelistDescriptionsEmpty(descriptions []p4lib.Description) bool {
	for _, desc := range descriptions {
		if len(desc.Files) != 0 {
			return false
		}
	}
	return true
}

// writePublishResult writes the publish result file
func writePublishResult(paths []string, packageName string, helper buildtool.Helper) error {
	var publishedFiles []*buildpb.PublishedFile
	for _, f := range paths {
		fileInfo, err := os.Stat(f)
		if err != nil {
			return fmt.Errorf("could not get size of file %q: %v", f, err)
		}
		publishedFiles = append(publishedFiles, &buildpb.PublishedFile{
			Size: fileInfo.Size(),
		})
	}
	var version string
	baseCl := helper.Invocation().PublishInvocation.BaseCl
	if baseCl != 0 {
		version = fmt.Sprintf("%d", baseCl)
	}
	publishResult := &buildpb.PublishResult{
		Name:    packageName,
		Version: version,
		Files:   publishedFiles,
	}
	helper.MustWritePublishResult(&buildpb.PublishInvocationResult{
		PublishResults: []*buildpb.PublishResult{publishResult},
	})
	return nil
}

// makeChangelist returns the number of an existing or new changelist, depending on specifiedChange
func makeChangelist(p4 p4lib.P4, helper buildtool.Helper, name string, specifiedChange string, additionalDesc sgeflag.StringList) (int, error) {
	var change int
	switch specifiedChange {
	case "":
		var desc []string
		desc = append(desc, fmt.Sprintf("Publish %s.", name))
		if len(additionalDesc) > 0 {
			desc = append(desc, "")
			for _, d := range additionalDesc {
				desc = append(desc, d)
			}
		}
		pubInv := helper.Invocation().PublishInvocation
		if pubInv.BaseCl != 0 || pubInv.CiResultUrl != "" {
			desc = append(desc, "")
		}
		if pubInv.BaseCl != 0 {
			desc = append(desc, fmt.Sprintf("Built from CL/%d", pubInv.BaseCl))
		}
		if pubInv.CiResultUrl != "" {
			desc = append(desc, fmt.Sprintf("CI results: %s", pubInv.CiResultUrl))
		}
		var err error
		if change, err = p4.Change(strings.Join(desc, "\n")); err != nil {
			return -1, fmt.Errorf("could not create changelist. %v", err)
		}
	case "default":
		change = 0
	default:
		var err error
		if change, err = strconv.Atoi(specifiedChange); err != nil {
			return -1, fmt.Errorf("invalid change %q: %v", specifiedChange, err)
		}
	}
	return change, nil
}

// cleanupChange reverts the contents of the specified changelist and then deletes it
func cleanupChange(p4 p4lib.P4, change int) error {
	if _, err := p4.ExecCmd("revert", "-c", strconv.Itoa(change), "//..."); err != nil {
		return fmt.Errorf("could not revert change %d: %v", change, err)
	}
	if _, err := p4.ExecCmd("change", "-d", strconv.Itoa(change)); err != nil {
		return fmt.Errorf("could not delete change %d: %v", change, err)
	}
	return nil
}
