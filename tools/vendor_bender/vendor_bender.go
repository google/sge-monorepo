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

// Binary vendor-bender automates a lot of the work involved in vendoring external repos into our perforce server
// You give it the URL of a repo you want to vendor, it will download it and create a perforce changelist with all files you need to add

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/libs/go/files"
	"sge-monorepo/tools/vendor_bender/protos/manifestpb"
	"sge-monorepo/tools/vendor_bender/protos/metadatapb"
	"sge-monorepo/tools/vendor_bender/bazel"
	"sge-monorepo/tools/vendor_bender/golang"

	"github.com/golang/protobuf/proto"
)

type pkg struct {
	path     string
	mdPath   string
	metatada metadatapb.Metadata
}

type manifest struct {
	path     string
	wsPath   string
	manifest manifestpb.Manifest
	pkgs     []pkg
}

type pkgContext struct {
	pkg     *pkg
	pkgMf   *manifest
	entry   *manifestpb.ManifestEntry
	entryMf *manifest
}

type context struct {
	mrRoot     string // monorepo root
	pkgCtxs    []pkgContext
	pkgWsPaths map[string]string
	verbose    bool
}

type action interface {
	print(ctx *context)
	run(ctx *context) error
}

type plan struct {
	actions []action
}

// Given a list of file names, this function finds the first
// one that exists on disk in a given directory.
// Allows priority selections for BUILD, BUILD.bazel etc...
func getInputFile(dir string, names []string) *string {
	for _, name := range names {
		path := path.Join(dir, name)
		if stat, err := os.Stat(path); err == nil {
			if !stat.Mode().IsDir() {
				return &path
			}
		}
	}
	return nil
}

// We traverse the third party hierarchy folder in search of either metadata files or manifests
// All packages found that are a direct children of the folder a manifest resides in are associated it.
// Note that recursion stop on packages since you can't have manifests within them
func collectPhysicalData(dir string, wsPath string, manifests *[]manifest) error {
	if mdFile := getInputFile(dir, []string{"METADATA.textpb", "METADATA"}); mdFile != nil {
		if len(*manifests) == 0 {
			return fmt.Errorf("root MANIFEST file not found, found METADATA file %s instead", *mdFile)
		}
		if in, err := ioutil.ReadFile(*mdFile); err == nil {
			md := &metadatapb.Metadata{}
			if err = proto.UnmarshalText(string(in), md); err != nil {
				return fmt.Errorf("could not read METADATA file %s: %v", *mdFile, err)
			}
			// We add to the first element on the manifest list, since it's the element
			// we are iterating directly over in the children loop below
			mds := &((*manifests)[0].pkgs)
			*mds = append(*mds, pkg{dir, *mdFile, *md})
			return nil
		}
	}
	if mfFile := getInputFile(dir, []string{"MANIFEST.textpb", "MANIFEST"}); mfFile != nil {
		if in, err := ioutil.ReadFile(*mfFile); err == nil {
			mf := &manifestpb.Manifest{}
			if err = proto.UnmarshalText(string(in), mf); err != nil {
				return fmt.Errorf("could not read MANIFEST file %s: %v", *mfFile, err)
			}
			mfs := []manifest{manifest{dir, wsPath, *mf, []pkg{}}}
			children, err := ioutil.ReadDir(dir)
			if err != nil {
				return err
			}
			for _, child := range children {
				if child.IsDir() {
					if err = collectPhysicalData(path.Join(dir, child.Name()), wsPath+"/"+child.Name(), &mfs); err != nil {
						return err
					}
				}
			}
			*manifests = append(*manifests, mfs...)
		}
	}

	return nil
}

// Collect physical data and computes a compact structure to be used by the rest of program
func buildContext(root string, verbose bool) (context, error) {
	var manifests []manifest
	if err := collectPhysicalData(path.Join(root, "third_party"), "//third_party", &manifests); err != nil {
		return context{}, err
	}
	pkgNameToCtx := make(map[string]pkgContext)
	for i := range manifests {
		for j := range manifests[i].pkgs {
			name := manifests[i].pkgs[j].metatada.Name
			if val, ok := pkgNameToCtx[name]; ok {
				return context{}, fmt.Errorf("package %s at %s conflicst with the one at %s)",
					name, manifests[i].pkgs[j].path, val.pkg.path)
			}
			pkgNameToCtx[name] = pkgContext{&manifests[i].pkgs[j], &manifests[i], nil, nil}
		}
		for _, entry := range manifests[i].manifest.ManifestEntry {
			if err := validateManifestEntry(root, entry, verbose); err != nil {
				return context{}, err
			}
			if val, ok := pkgNameToCtx[entry.Name]; ok {
				if val.entry != nil {
					return context{}, fmt.Errorf("manifest entry %s in %s conflicst with the one in %s",
						entry.Name, manifests[i].path, val.entryMf.path)
				}
				pkgNameToCtx[entry.Name] = pkgContext{val.pkg, val.pkgMf, entry, &manifests[i]}
			} else {
				pkgNameToCtx[entry.Name] = pkgContext{nil, nil, entry, &manifests[i]}
			}
		}
	}
	pkgCtxs := make([]pkgContext, len(pkgNameToCtx))
	pkgWsPaths := make(map[string]string)
	idx := 0
	for name, pkgCtx := range pkgNameToCtx {
		pkgCtxs[idx] = pkgCtx
		if pkgCtx.entry != nil && !pkgCtx.entry.SetAsLocalRepository {
			pkgWsPaths[name] = pkgCtx.entryMf.wsPath + "/" + name
		}
		idx += 1
	}
	return context{root, pkgCtxs, pkgWsPaths, verbose}, nil
}

// Handles addition/delete/update logic
func buildVendoringPlan(ctx *context, updateLabels bool) (plan, error) {
	var actions []action
	for _, pkgCtx := range ctx.pkgCtxs {
		if pkgCtx.pkg == nil && pkgCtx.entry == nil {
			return plan{}, fmt.Errorf(
				"internal error, either the package or its entry must exist")
		}
		updateLabel := false
		if pkgCtx.pkg == nil {
			// handle addition, we have a manifest entry with no package
			pkgPath := path.Join(pkgCtx.entryMf.path, pkgCtx.entry.Name)
			if files.DirExists(pkgPath) {
				return plan{}, fmt.Errorf("package directory already exists")
			}
			repoDeclPath := ""
			if pkgCtx.entry.SetAsLocalRepository {
				repoFile, err := bazel.RepoDeclFile(ctx.mrRoot, pkgPath, pkgCtx.entry.Name)
				if err != nil {
					return plan{}, err
				}
				repoDeclPath = repoFile
			}
			actions = append(actions, &pkgAdd{pkgCtx.entry, pkgPath, repoDeclPath})
		} else if pkgCtx.entry == nil {
			// // handle deletion, we have a package with no manifest entry
			actions = append(actions, &pkgDelete{pkgCtx.pkg.path})
		} else {
			// handle updates/moves
			if pkgCtx.pkgMf != pkgCtx.entryMf || needsUpdate(pkgCtx.pkg, pkgCtx.entry) {
				pkgPath := path.Join(pkgCtx.entryMf.path, pkgCtx.entry.Name)
				detectDivergences(pkgPath)
				actions = append(actions, &pkgUpdate{pkgCtx.entry, pkgPath, pkgCtx.pkg.path})
			} else {
				updateLabel = !pkgCtx.entry.SetAsLocalRepository && (updateLabels || pkgCtx.pkg.metatada.ThirdParty.IsLocalRepository)
			}
		}

		if updateLabel {
			replacementMap, err := bazel.CollectLabelReplacements(ctx.pkgWsPaths, pkgCtx.pkg.path, ctx.pkgWsPaths[pkgCtx.entry.Name])
			if err != nil {
				return plan{}, err
			}
			if len(replacementMap) != 0 {
				// todo: this logic should move to the bazel lib
				wsFile := path.Join(pkgCtx.pkg.path, "WORKSPACE")
				if !files.FileExists(wsFile) {
					wsFile = ""
				}
				actions = append(actions, &labelUpdate{replacementMap, wsFile})
			}
		}
	}
	return plan{actions}, nil
}

func buildRegenPlan(ctx *context, packageName string, clean bool) (plan, error) {
	var actions []action
	for _, pkgCtx := range ctx.pkgCtxs {
		if pkgCtx.pkg == nil || pkgCtx.entry == nil {
			continue
		}
		if packageName != "" && packageName != pkgCtx.entry.Name {
			continue
		}
		actions = append(actions, &bazelRegen{pkgCtx.entry, pkgCtx.pkg.path, clean})
	}
	return plan{actions}, nil
}

func renderPlan(ctx *context, plan *plan, dryRun bool) error {
	for _, action := range plan.actions {
		action.print(ctx)
		if dryRun {
			continue
		}
		if err := action.run(ctx); err != nil {
			return err
		}
	}
	return nil
}

func vendor(ctx *context, args []string) error {
	flags := flag.NewFlagSet("vendor", flag.ExitOnError)
	updateLabels := flags.Bool("update-labels", false, "Update labels for bazel")
	dryRun := flags.Bool("dry-run", false, "Prints the plan without performing any IO")
	diffDiv := flags.Bool("diff-divergences", false, "Opens the diff tool to diff the divergences")
	flags.Parse(args)
	plan, err := buildVendoringPlan(ctx, *updateLabels)
	if err != nil {
		return fmt.Errorf("failed to create a vendoring plan: %v", err)
	}
	if err := renderPlan(ctx, &plan, *dryRun); err != nil {
		return err
	}
	if *diffDiv && !*dryRun {
		return diffDivergences()
	}
	return nil
}

func regen(ctx *context, args []string) error {
	flags := flag.NewFlagSet("regen", flag.ExitOnError)
	clean := flags.Bool("clean", false, "delete BUILD files")
	dryRun := flags.Bool("dry-run", false, "Prints the plan without performing any IO")
	flags.Parse(args)
	packageName := ""
	if flags.NArg() == 1 {
		packageName = flags.Arg(0)
	}
	plan, err := buildRegenPlan(ctx, packageName, *clean)
	if err != nil {
		return fmt.Errorf("failed to create a regen plan: %v", err)
	}
	return renderPlan(ctx, &plan, *dryRun)
}

func analyse(ctx *context, args []string) error {
	flags := flag.NewFlagSet("regen", flag.ExitOnError)
	omitNonGo := flags.Bool("omit-non-go", true, "omit non-Go repos from output")
	return golang.GazelleAnalyze(ctx.mrRoot, ctx.verbose, *omitNonGo)
}

func printUsage() {
	fmt.Println("usage: vendor_bender [-verbose] cmd <args> <options>")
	fmt.Println("commands:")
	fmt.Println("  - vendor [-update-labels] [-diff-divergences] [-dry-run]")
	fmt.Println("    Reads MANIFEST files in the monorepo and resolves the state of the packages with manifest content")
	fmt.Println("    args:")
	fmt.Println("      -update-labels: Check and update all labels for consistency")
	fmt.Println("      -diff-divergences: Opens the diff tool to diff the divergences")
	fmt.Println("      -dry-run: Prints the plan without performing any IO")
	fmt.Println("  - regen [package] [-clean] [-dry-run]")
	fmt.Println("    Generate BUILD files using the package type generator")
	fmt.Println("    args:")
	fmt.Println("      package: Package to regen BUILD files for")
	fmt.Println("      -clean: Delete old build files before proceeding")
	fmt.Println("      -dry-run: Prints all performed actions without performing any IO")
	fmt.Println("  - analyse [-omit-non-go]")
	fmt.Println("    Analyses the vendored packages for common issues and prints messages")
	fmt.Println("    args:")
	fmt.Println("      -omit-non-go: Omit non-Go repos from output")
}

func vendorBender() error {
	verbose := flag.Bool("verbose", false, "display more information")
	flag.Parse()
	if len(flag.Args()) < 1 {
		printUsage()
		return nil
	}
	monorepo, _, err := monorepo.NewFromPwd()
	if err != nil {
		return fmt.Errorf("failed to find a monorepo root: %v", err)
	}

	ctx, err := buildContext(monorepo.Root, *verbose)
	if err != nil {
		return fmt.Errorf("failed to create vendor bender context: %v", err)
	}

	cmd := flag.Args()[0]
	switch cmd {
	case "vendor":
		return vendor(&ctx, flag.Args()[1:])
	case "regen":
		return regen(&ctx, flag.Args()[1:])
	case "analyse":
		return analyse(&ctx, flag.Args()[1:])
	default:
		printUsage()
	}
	return nil
}

func main() {
	if err := vendorBender(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
