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

package p4lib

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
)

// Print invokes "p4 print" and retrieves specified version(s) of files(s) from the server
func (p4 *impl) Print(args ...string) (string, error) {
	cmd := []string{"print"}
	cmd = append(cmd, args...)
	return p4.ExecCmd(cmd...)
}

// Callback handlers for PrintEx and Files
type printcb []FileDetails

// The p4 API will call outputStat for each file.  It is expected that
// this function is invoked before outputBinary or outputText.
func (cb *printcb) outputStat(stats map[string]string) error {
	idx := len(*cb)
	*cb = append(*cb, FileDetails{})
	details := &(*cb)[idx]
	for key, value := range stats {
		if err := setTaggedField(details, key, value, false); err != nil {
			glog.Warningf("Couldn't set field %v: %v", key, err)
		}
	}
	return nil
}

// The p4 API will call outputBinary for each file with type 'binary'.
// It is expected that this function is invoked after outputStat.
func (cb *printcb) outputBinary(data []byte) error {
	idx := len(*cb) - 1
	if idx < 0 {
		return fmt.Errorf("expected stats before payload")
	}
	(*cb)[idx].Content = append((*cb)[idx].Content, data...)
	return nil
}

// The p4 API will call outputText for each file with type 'text'.
// It is expected that this function is invoked after outputStat.
func (cb *printcb) outputText(data string) error {
	return cb.outputBinary([]byte(data))
}

// The p4 API will call onRetry before retrying the command.  Abandon any
// partial results.
func (cb *printcb) onRetry(context, err string) {
	// Reset data.
	*cb = []FileDetails{}
}

// Implementing tagProtocol indicates to the p4 API that we want to use the
// 'tag' protocol instead of the default protocol.  The tag protocol will
// precede each file contents with a call to outputStat with details about
// each file.
func (p *printcb) tagProtocol() {}

// PrintEx uses the API to retrieve specified version(s) of file(s) from the server.
func (p4 *impl) PrintEx(files ...string) ([]FileDetails, error) {
	cb := printcb{}
	err := p4.runCmdCb(&cb, "print", files...)
	if err != nil && strings.Contains(err.Error(), "no such file(s).") {
		return cb, ErrFileNotFound
	}
	return cb, err
}

func (p4 *impl) Files(files ...string) ([]FileDetails, error) {
	cb := printcb{}
	err := p4.runCmdCb(&cb, "files", files...)
	if err != nil && strings.Contains(err.Error(), "no such file(s).") {
		return cb, ErrFileNotFound
	}
	return cb, err
}
