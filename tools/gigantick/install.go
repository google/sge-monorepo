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
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"
)

type toolDefinition struct {
	XMLName   xml.Name `xml:"Definition"`
	Name      string   `xml:"Name"`
	Command   string   `xml:"Command"`
	Arguments string   `xml:"Arguments"`
	Shortcut  string   `xml:"Shortcut"`
}

type prompt struct {
	PromptText string `xml:"PromptText,omitempty"`
	ShowBrowse bool   `xml:"ShowBrowse,omitempty"`
}

type console struct {
	CloseOnExit bool `xml:"CloseOnExit,omitempty"`
}

type customToolDef struct {
	XMLName        xml.Name       `xml:"CustomToolDef"`
	Definition     toolDefinition `xml:"Definition"`
	Prompt         *prompt        `xml:"Prompt,omitempty"`
	Console        *console       `xml:"Console,omitempty"`
	AddToContext   bool           `xml:"AddToContext,omitempty"`
	Refresh        bool           `xml:"Refresh,omitempty"`
	IgnoreP4Config bool           `xml:"IgnoreP4Config,omitempty"`
}

type customToolDefList struct {
	XMLName       xml.Name        `xml:"CustomToolDefList"`
	VarName       string          `xml:"varName,attr"`
	CustomToolDef []customToolDef `xml:"CustomToolDef"`
}

func install() error {
	userDir := os.ExpandEnv("${USERPROFILE}")
	p4Dir := filepath.Join(userDir, ".p4qt")
	if _, err := os.Stat(p4Dir); os.IsNotExist(err) {
		return fmt.Errorf("could not find p4 installation: %v", err)
	}
	toolsFile := filepath.Join(p4Dir, "customtools.xml")
	glog.Infof("installing custom tools : %s", toolsFile)
	ctd := customToolDefList{
		VarName: "customtooldeflist",
	}
	if b, err := ioutil.ReadFile(toolsFile); err == nil {
		err := xml.Unmarshal(b, &ctd)
		if err != nil {
			glog.Warningf("unable to unmarshal %s: %v", toolsFile, err)
		}
	}
	gctd := customToolDef{
		Definition: toolDefinition{
			Name:      "Gigantick",
			Command:   os.Args[0],
			Arguments: "-port=$p -user=$u -workspace=$c -change%p",
		},
		AddToContext: true,
	}
	found := false
	for i := range ctd.CustomToolDef {
		def := ctd.CustomToolDef[i].Definition
		if def.Name == gctd.Definition.Name {
			if def.Command == gctd.Definition.Command && def.Arguments == gctd.Definition.Arguments && ctd.CustomToolDef[i].AddToContext == gctd.AddToContext {
				glog.Infof("gigantick custom tool already installed")
				return nil
			}
			ctd.CustomToolDef[i] = gctd
			found = true
			break
		}
	}
	if !found {
		ctd.CustomToolDef = append(ctd.CustomToolDef, gctd)
	}
	b, err := xml.MarshalIndent(&ctd, "", " ")
	if err != nil {
		return fmt.Errorf("couldn't generate customtools.xml: %v", err)
	}
	b = []byte(xml.Header + string(b))

	if err = ioutil.WriteFile(toolsFile, b, os.ModePerm); err != nil {
		return fmt.Errorf("couldn't save %s: %v", toolsFile, err)
	}
	glog.Infof("gigantick custom p4v tooling installed successfully")
	return nil
}
