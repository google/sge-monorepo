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
	"fmt"
	"os"

	"google.golang.org/api/option"
	"sge-monorepo/libs/go/builddist"
)

func help() {
	const helpString = `
Usage:
    build-dist.exe [--credentials path] [--auth-default] <command> [arguments]

The commands are:

    delete-package [package-config.json] [version-id]
    install-package [package-config.json] [version-id]
    list-packages [package-config.json]
    make-package [package-config.json]
    register-product [package-config.json]

About authentication:
    For user accounts the application will launch a web browser window to authenticate.
    To use gcloud application-default authentication, specifiy --auth-default.
    For service accounts, use --credentials to specifiy the path to the json authentication token
`
	fmt.Println(helpString)
}

func consumeArgument(index *int, commandName string, argumentName string) string {
	//should we use the package flags instead?
	*index++
	if *index >= len(os.Args) {
		fmt.Printf("missing %v argument in %v command\n", argumentName, commandName)
		help()
		os.Exit(1)
	}
	return os.Args[*index]
}

func feedback(msg string) {
	fmt.Println(msg)
}

func main() {
	if len(os.Args) <= 1 {
		help()
		os.Exit(1)
	}

	ctx := context.Background()
	getAuthClientOption := builddist.MakeDefaultAuthClientOption

	for i := 1; i < len(os.Args); i++ {
		switch commandName := os.Args[i]; commandName {

		case "--auth-default":
			getAuthClientOption = func(context.Context, builddist.PackageConfig) option.ClientOption {
				return option.WithCredentials(nil)
			}

		case "--credentials":
			credentialsFilename := consumeArgument(&i, commandName, "product-config.json")
			getAuthClientOption = func(context.Context, builddist.PackageConfig) option.ClientOption {
				return option.WithCredentialsFile(credentialsFilename)
			}

		case "list-packages":
			configFile := consumeArgument(&i, commandName, "product-config.json")
			packageConfig, err := builddist.ReadPackageConfig(configFile)
			if err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
			lp, err := builddist.ListPackages(ctx, *packageConfig, getAuthClientOption(ctx, *packageConfig))
			if err != nil {
				fmt.Println(err)
				os.Exit(3)
			}
			for _, pkg := range lp {
				fmt.Println(pkg.Version, pkg.State)
			}

		case "delete-package":
			configFile := consumeArgument(&i, commandName, "product-config.json")
			versionId := consumeArgument(&i, commandName, "version-id")
			packageConfig, err := builddist.ReadPackageConfig(configFile)
			if err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
			builddist.DeletePackage(ctx, *packageConfig, versionId, getAuthClientOption(ctx, *packageConfig))

		case "install-package":
			configFile := consumeArgument(&i, commandName, "product-config.json")
			versionId := consumeArgument(&i, commandName, "version-id")
			packageConfig, err := builddist.ReadPackageConfig(configFile)
			if err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
			builddist.InstallPackage(ctx, *packageConfig, versionId, getAuthClientOption(ctx, *packageConfig), feedback)

		case "make-package":
			configFile := consumeArgument(&i, commandName, "product-config.json")
			packageConfig, err := builddist.ReadPackageConfig(configFile)
			if err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
			builddist.MakePackage(ctx, *packageConfig, getAuthClientOption(ctx, *packageConfig))

		case "register-product":
			configFile := consumeArgument(&i, commandName, "product-config.json")
			packageConfig, err := builddist.ReadPackageConfig(configFile)
			if err != nil {
				fmt.Println(err)
				os.Exit(2)
			}
			builddist.RegisterProduct(ctx, *packageConfig, getAuthClientOption(ctx, *packageConfig))

		default:
			fmt.Printf("unknown command %v\n", commandName)
			help()
			os.Exit(1)
		}
	}
}
