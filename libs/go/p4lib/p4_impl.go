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
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/glog"
)

var (
	ErrKeyNotSet = errors.New("p4 key not set")
)

// Add executes a p4 add, marking everything in paths for add using the options received as params
func (p4 *impl) Add(paths []string, options ...string) (string, error) {
	args := []string{"add"}

	args = append(args, options...)

	args = append(args, paths...)
	return p4.ExecCmd(args...)
}

// AddDir executes a p4 add for everything in directory dir and adds it using the options received as params
func (p4 *impl) AddDir(dir string, options ...string) (string, error) {
	p := dir + "/*"
	entries, err := filepath.Glob(p)
	if err != nil {
		return "", err
	}

	var files []string
	for _, entry := range entries {
		if fi, err := os.Stat(entry); err == nil {
			if fi.Mode().IsDir() {
				if out, err := p4.AddDir(entry, options...); err != nil {
					return out, err
				}

			} else {
				files = append(files, entry)
			}
		}
	}

	var subFiles []string
	total := 0
	const cmdLineMax = 4000
	for _, f := range files {
		if total+len(f) > cmdLineMax {
			if _, err := p4.Add(subFiles, options...); err != nil {
				return "", err
			}
			total = 0
			subFiles = nil
		}

		// We call p4 ignores on the file before adding it to our list of file to add.
		// If ignores returns a string it means the file is ignored so we can skip it.
		// Not skipping it might make the p4 add fail and our function would stop processing
		// other files.
		var ignoresOut string
		if ignoresOut, err := p4.Ignores([]string{f}); err != nil {
			return ignoresOut, err
		}
		if len(ignoresOut) == 0 {
			subFiles = append(subFiles, f)
			total += len(f)
		}
	}

	if len(subFiles) == 0 {
		return "", nil
	}
	return p4.Add(subFiles, options...)
}

// Change executes a p4 change command and creates a new changelist with specified description
func (p4 *impl) Change(desc string) (int, error) {
	info, err := p4.Info()
	if err != nil {
		return 0, err
	}

	spec := "Change:\tnew\n\n"
	spec += fmt.Sprintf("Client:\t%s\n\n", info.Client)
	spec += fmt.Sprintf("User:\t%s\n\n", info.User)
	spec += fmt.Sprintf("Status:\tnew\n\n")
	spec += "Description:\n"
	for _, line := range strings.Split(desc, "\n") {
		spec += fmt.Sprintf("\t%s\n", line)
	}

	var b bytes.Buffer
	b.Write([]byte(spec))
	stdOutErr, err := p4.execCmdWithStdin(&b, []string{"change", "-i"})
	if err != nil {
		return 0, err
	}

	lines := strings.Split(string(stdOutErr), "\n")
	if len(lines) >= 1 {
		words := strings.Split(lines[0], " ")
		if len(words) >= 3 {
			if words[0] == "Change" {
				return strconv.Atoi(words[1])
			}
		}
	}

	return 0, fmt.Errorf("couldn't find CL number")
}

// ChangeUpdate executes a p4 change command and creates a new changelist with specified description
func (p4 *impl) ChangeUpdate(desc string, cl int) error {
	stdOutErr, err := p4.ExecCmd("change", "-o", fmt.Sprintf("%d", cl))
	if err != nil {
		return err
	}

	newDesc := ""
	lines := strings.Split(stdOutErr, "\n")
	copy := true
	for _, line := range lines {
		if strings.HasPrefix(line, "Description:") {
			newDesc += line + "\n"
			newLines := strings.Split(desc, "\n")
			for _, nl := range newLines {
				newDesc += fmt.Sprintf("\t%s\n", nl)
			}
			copy = false
			continue
		}
		copy = copy || !strings.HasPrefix(line, "\t")
		if copy {
			newDesc += line + "\n"
		}
	}

	var b bytes.Buffer
	b.Write([]byte(newDesc))
	_, err = p4.execCmdWithStdin(&b, []string{"change", "-i"})
	return err
}

// ObtainClient tries to query for a given client. If |name| is empty, it is assumed that the
// default client will be queried (the one set in P4CLIENT).
func (p4 *impl) Client(clientName string) (*Client, error) {
	args := []string{"client", "-o"}
	if clientName != "" {
		args = append(args, clientName)
	}

	data, err := p4.ExecCmd(args...)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, data)
	}

	return parseClient(data)
}

func (p4 *impl) ClientSet(client *Client) (string, error) {
	var b bytes.Buffer
	b.Write([]byte(client.String()))
	return p4.execCmdWithStdin(&b, []string{"client", "-i"})
}

var clientsRegex = regexp.MustCompile(`Client\s(.+)\s(\d+)/(\d+)/(\d+)\sroot\s(.+)\s'(.*)'`)

func parseClients(input string) ([]string, error) {
	// Line format is:
	// Client <CLIENT_NAME> <CREATION_DATE> root <ROOT> '<DESCRIPTION>'
	var clients []string
	for i, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		matches := clientsRegex.FindStringSubmatch(line)
		if len(matches) == 0 {
			return nil, fmt.Errorf("could not parse client line %d: %s.", i, line)
		}
		// We really just care about the client names.
		clients = append(clients, matches[1])
	}
	sort.Slice(clients, func(i, j int) bool {
		return clients[i] < clients[j]
	})
	return clients, nil
}

func (p4 *impl) Clients() ([]string, error) {
	out, err := p4.ExecCmd("clients")
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, out)
	}
	return parseClients(out)
}

func parseClient(data string) (*Client, error) {
	client := &Client{}

	// Go over each line. Some markers are multi-line and might advance |i|.
	lines := strings.Split(data, "\n")
	for i := 0; i < len(lines); {
		line := strings.Trim(lines[i], " \n\r")
		i += 1

		// Ignore empty and comment lines.
		if line == "" || line[0] == '#' {
			continue
		}

		tokens := tokenize(line)

		// We first check for multiline entries. These can advance the line we're iterating on
		// with inner loops.

		isSingleLineField := false // Whether this was hadled in the multi-line switch.
		switch tokens[0] {
		case "ChangeView:":
			for i < len(lines) {
				newLine := strings.Trim(lines[i], " \r\n")
				newTokens := tokenize(newLine)

				// Ignore empty and comment lines.
				if len(newTokens) == 0 || strings.HasPrefix(newTokens[0], "#") {
					i += 1
					continue
				}

				// If we find a new field token, we know we're at a new field.
				if isFieldToken(newTokens[0]) {
					break
				}

				client.ChangeView = append(client.ChangeView, newLine)
				i += 1
			}
		case "Description:":
			for i < len(lines) {
				newLine := strings.Trim(lines[i], " \r\n")

				// If we find a new field token, we know we're at a new field.
				newTokens := tokenize(newLine)
				if len(newTokens) > 0 && isFieldToken(newTokens[0]) {
					break
				}

				client.Description += newLine + "\n"
				i += 1
			}
		case "View:":
			for i < len(lines) {
				newLine := strings.Trim(lines[i], " \r\n")
				newTokens := tokenize(newLine)

				// Ignore empty and comment lines.
				if len(newTokens) == 0 || strings.HasPrefix(newTokens[0], "#") {
					i += 1
					continue
				}

				// If we find a new field token, we know we're at a new field.
				if isFieldToken(newTokens[0]) {
					break
				}

				// Line should be two entries
				if len(newTokens) != 2 {
					return nil, fmt.Errorf("wrong view entry: %s", newLine)
				}

				client.View = append(client.View, ViewEntry{
					Source:      newTokens[0],
					Destination: newTokens[1],
				})
				i += 1
			}
		default:
			isSingleLineField = true
		}

		if !isSingleLineField {
			continue
		}

		// All other fields are single line.
		if len(tokens) < 2 {
			return nil, fmt.Errorf("wrong line: %s", line)
		}

		// Verify line tokens are precisely two for certain fields.
		switch tokens[0] {
		case "Client:", "Owner:", "Root:", "LineEnd:", "Host:", "Stream:", "StreamAtChange:", "ServerID:":
			if len(tokens) != 2 {
				return nil, fmt.Errorf("wrong line: %s", line)
			}
		}

		// Apply the fields.
		switch tokens[0] {
		case "Client:":
			client.Client = tokens[1]
		case "Owner:":
			client.Owner = tokens[1]
		case "Root:":
			client.Root = tokens[1]
		case "Options:":
			if err := client.addOptions(tokens[1:]); err != nil {
				return nil, err
			}
		case "SubmitOptions:":
			client.SubmitOptions = tokens[1:]
		case "LineEnd:":
			client.LineEnd = tokens[1]
		case "Host:":
			client.Host = tokens[1]
		case "AltRoots:":
			client.AltRoots = tokens[1:]
		case "Stream:":
			client.Stream = tokens[1]
		case "StreamAtChange:":
			client.StreamAtChange = tokens[1]
		case "ServerID:":
			client.ServerId = tokens[1]
		}
	}

	// Verify that we got all the mandatory fields.
	if err := verifyClient(client); err != nil {
		return nil, err
	}

	return client, nil
}

var clientOptions = []ClientOption{
	AllWrite,
	Clobber,
	Compress,
	Locked,
	Modtime,
	Rmdir,
	NoAllWrite,
	NoClobber,
	NoCompress,
	Unlocked,
	NoModtime,
	NoRmdir,
}

func (client *Client) addOptions(opts []string) error {
	options := client.Options
	for _, o := range opts {
		foundOption := ""
		for _, co := range clientOptions {
			if ClientOption(o) == co {
				foundOption = o
				break
			}
		}
		if foundOption == "" {
			return fmt.Errorf("client option %q doesn't exist", o)
		}
		var err error
		options, err = AppendClientOption(options, ClientOption(foundOption))
		if err != nil {
			return err
		}
	}
	client.Options = options
	return nil
}

func verifyClient(client *Client) error {
	// Verify that the mandatory fields are there.
	if client.Client == "" {
		return fmt.Errorf("missing Client")
	}
	if client.Owner == "" {
		return fmt.Errorf("missing Owner")
	}
	if client.Root == "" {
		return fmt.Errorf("missing Root")
	}
	if len(client.Options) == 0 {
		return fmt.Errorf("missing Options")
	}
	if len(client.SubmitOptions) == 0 {
		return fmt.Errorf("missing SubmitOptions")
	}
	if client.LineEnd == "" {
		return fmt.Errorf("missing LineEnd")
	}
	if len(client.View) == 0 {
		return fmt.Errorf("missing View")
	}

	return nil
}

// tokenize will separete the line into separate works, trimming unnecessary separation chars.
func tokenize(line string) []string {
	trimmed := strings.Trim(line, " \t\n\r")
	split := strings.Fields(trimmed)

	// Only add non-empty elements.
	var result []string
	for _, elem := range split {
		if elem == "" {
			continue
		}

		result = append(result, elem)
	}

	return result
}

// Whether this token marks a field (eg. "Client:" or "Description:").
func isFieldToken(token string) bool {
	return token != "" && token[len(token)-1] == ':'
}

func (p4 *impl) Delete(paths []string, cl int) (string, error) {
	args := []string{"delete"}
	if cl != 0 {
		args = append(args, "-c", strconv.Itoa(cl))
	}
	args = append(args, paths...)
	return p4.ExecCmd(args...)
}

// Ensure the regex is only compiled once at startup
// By default, perforce uses the basic unix diff format as described here https://en.wikipedia.org/wiki/Diff
// This gives a block range for left file, followed by op type (add/change/delete) then range for right file
// The closing point of range is optional [if omitted, a single line is assumed]. Examples:
//  2a3 [after line 2 in left file, add line 3 from right file]
//  4d3 [delete line 4 from left file, will then sync to line 3 on right file]
//  12,20c12,20 [changes lines 12-20 in left file to match lines 12-20 in right file]
//  202a223,262 [after line 202 in left file, add lines 223-262 from right file]
// This regex has 7 capture groups, with optional groups for the closing ranges.
var diffCmd = regexp.MustCompile(`^(\d+)(,(\d+))?([^,\d])(\d+)(,(\d+))?`)

func diffsBuild(p4 *impl, cmd string, file0 string, file1 string) ([]Diff, error) {
	out, err := p4.ExecCmd(cmd, file0, file1)
	if err != nil {
		return nil, err
	}

	var diffs []Diff
	lines := strings.Split(out, "\n")
	// skip the first line as it contains header infomation
	for _, line := range lines[1:] {
		line := strings.TrimSpace(line)
		if matches := diffCmd.FindStringSubmatch(line); matches != nil {
			// if regex is succesful, left range will be captured in groups 1-3, right in 5-7
			// group 4 will contain the op type (a,d,c)
			// groups 2 and 6 will either have commas (if there is a closing range) or will be
			left0, _ := strconv.Atoi(matches[1])
			left1, _ := strconv.Atoi(matches[3])
			right0, _ := strconv.Atoi(matches[5])
			right1, _ := strconv.Atoi(matches[7])
			if left1 < left0 {
				left1 = left0
			}
			if right1 < right0 {
				right1 = right0
			}
			var dt DiffType
			switch matches[4] {
			case "a":
				dt = DiffAdd
			case "c":
				dt = DiffChange
			case "d":
				dt = DiffDelete
			default:
				return nil, fmt.Errorf("unhandled diff type %s", matches[4])
			}
			diffs = append(diffs, Diff{
				LeftStartLine:  left0,
				LeftEndLine:    left1,
				RightStartLine: right0,
				RightEndLine:   right1,
				DiffType:       dt,
			})
		}
	}
	return diffs, nil
}

// DiffFile opens the P4Merge to diff between a local file and its revisions on the perforce server.
func (p4 *impl) DiffFile(file string) error {
	err := p4.Set("P4DIFF", "P4Merge")
	if err != nil {
		return err
	}
	_, err = p4.ExecCmd("diff", file)
	return err
}

// Diff executes a "p4 diff" command that performs a diff between a local file and file that exists on the perforce server
func (p4 *impl) Diff(file0 string, file1 string) ([]Diff, error) {
	return diffsBuild(p4, "diff", file0, file1)
}

// Diff2 executes a "p4 diff2" command that performs a diff between two files that exist on the perforce server
func (p4 *impl) Diff2(file0 string, file1 string) ([]Diff, error) {
	return diffsBuild(p4, "diff2", file0, file1)
}

// Dirs invokes "p4 dirs" and returns a list of subdirectories in specific root folder
type dirscb []string

func (cb *dirscb) outputInfo(level int, info string) error {
	*cb = append(*cb, info)
	return nil
}
func (cb *dirscb) retry(context, err string) {
	*cb = []string{}
}
func (p4 *impl) Dirs(root string) ([]string, error) {
	if strings.HasSuffix(root, "...") {
		root = root[:len(root)-3] + "*"
	}
	dirs := &dirscb{}
	err := p4.runCmdCb(dirs, "dirs", root)
	if err != nil && strings.Contains(err.Error(), "- no such file(s).") {
		return *dirs, nil
	}
	return *dirs, err
}

// Edit executes a p4 edit of every file in the paths slice and adds them to changelist cl
func (p4 *impl) Edit(paths []string, cl int) (string, error) {
	args := []string{"edit"}
	if cl != 0 {
		args = append(args, "-c", strconv.Itoa(cl))
	}
	args = append(args, paths...)
	return p4.ExecCmd(args...)
}

// ExecCmd executes a perforce command with specified arguments
func (p4 *impl) ExecCmd(args ...string) (string, error) {
	return p4.execCmdWithStdin(nil, args)
}

func (p4 *impl) ExecCmdWithOptions(args []string, opts ...Option) (string, error) {
	return p4.execCmdWithStdin(nil, args, opts...)
}

// outputMultiplexer implements the io.Writer interface so that it can both store the the data
// written internally and output it to an optional external io.Writer as well. This is used to
// implement the OutputOption.
type outputMultiplexer struct {
	internal bytes.Buffer
	external io.Writer
}

func newOutputMultiplexer(external io.Writer) outputMultiplexer {
	return outputMultiplexer{
		external: external,
	}
}

func (om *outputMultiplexer) Write(p []byte) (int, error) {
	// Write into external if available.
	if om.external != nil {
		n, err := om.external.Write(p)
		if err != nil {
			return n, err
		}
	}

	// Write into the internal buffer.
	return om.internal.Write(p)
}

// Define a buffer object that implements generic p4 callbacks that produce
// the same results as invoking 'p4 <args...>' when using the API.
type buffer struct {
	output bytes.Buffer
	input  io.Reader
}

func (b *buffer) outputBinary(data []byte) error {
	b.output.Write(data)
	return nil
}
func (b *buffer) outputText(data string) error {
	b.output.WriteString(data)
	return nil
}
func (b *buffer) outputInfo(level int, info string) error {
	const prefix = "... ... "
	if level > 0 && level <= 2 {
		b.output.WriteString(prefix[0 : 4*level])
	}
	b.output.WriteString(info)
	b.output.WriteRune('\n')
	return nil
}
func (b *buffer) outputStat(stats map[string]string) error {
	for k, v := range stats {
		level := 1
		if strings.HasPrefix(k, "other") {
			level = 2
		}
		b.outputInfo(level, fmt.Sprintf("%v %v", k, v))
	}
	b.outputInfo(0, "")
	return nil
}
func (b *buffer) Read(p []byte) (int, error) {
	if b.input != nil {
		return b.input.Read(p)
	}
	return 0, io.EOF
}
func (b *buffer) retry(context, err string) {
	b.output.Reset()
}

// Allowlist of commands that are known to work with the API.
// Mostly commands that read data from perforce and don't manipulate the
// workspace. Expect this list to grow over time as more commands are tested
// and confirmed to work.
var useApi = map[string]struct{}{
	"fstat": {},
	"key":   {},
	"keys":  {},
	"login": {},
	"print": {},
}

func (p4 *impl) execCmdWithStdin(stdin io.Reader, args []string, opts ...Option) (string, error) {
	appliedOpts := options{}
	for _, opt := range opts {
		opt.apply(&appliedOpts)
	}

	if _, ok := useApi[args[0]]; ok {
		b := buffer{input: stdin}
		err := p4.runCmdCb(&b, args[0], args[1:]...)
		if appliedOpts.output != nil {
			appliedOpts.output.Write(b.output.Bytes())
		}
		return b.output.String(), err
	}

	if p4.tracer != nil {
		endtrace := p4.tracer(args[0])
		defer endtrace()
	}
	start := time.Now()
	defer func() {
		stop := time.Now()
		updateStats(args[0], stop.Sub(start).Microseconds(), 0)
	}()

	var p4Args []string
	p4Args = append(p4Args, "-C", "utf8")
	if p4.user != "" {
		p4Args = append(p4Args, "-u", p4.user)
	}
	if p4.passwd != "" {
		p4Args = append(p4Args, "-P", p4.passwd)
	}
	p4Args = append(p4Args, args...)
	com := exec.Command(p4.exePath, p4Args...)

	// ensure dos window is hidden when process is started
	hideWindow(com)
	com.Stdin = stdin

	// If the user defined another output buffer, we make sure it getrs redirected there as well.
	om := newOutputMultiplexer(appliedOpts.output)
	com.Stdout = &om
	com.Stderr = &om
	err := com.Run()
	output := om.internal.String()

	if err != nil {
		log.Println(com)
		log.Println(output)
		log.Println(err)
		return output, err
	}
	return output, nil
}

// Grep executes a p4grep and returns details of files and lines matching input pattern
// This is designed for small greps and has a limit of 10K files participating in each action
func (p4 *impl) Grep(pattern string, caseSensitive bool, depotPaths ...string) ([]Grep, error) {
	args := []string{"grep"}
	if !caseSensitive {
		args = append(args, "-i")
	}
	args = append(args, "-n", "-s", "-e", pattern)
	args = append(args, depotPaths...)
	out, err := p4.ExecCmd(args...)
	if err != nil {
		return nil, err
	}

	var greps []Grep
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		semi0 := strings.Index(line, ":")
		if semi0 < 0 {
			continue
		}
		semi1 := strings.Index(line[semi0+1:], ":")
		if semi1 < 0 {
			continue
		}
		semi1 += semi0
		hash := strings.LastIndex(line[:semi0], "#")
		revision := 0
		if hash >= 0 {
			revision, _ = strconv.Atoi(line[hash+1 : semi0])
		} else {
			hash = 0
		}
		lineNum, _ := strconv.Atoi(line[semi0+1 : semi1+1])
		greps = append(greps, Grep{
			Contents:   line[semi1+2:],
			DepotPath:  line[:hash],
			LineNumber: lineNum,
			Revision:   revision,
		})

	}
	return greps, nil
}

type grepContext struct {
	wg     sync.WaitGroup
	greps  []Grep
	m      sync.Mutex
	status *GrepStatus
}

// perforce imposes a hard limit in the number of files that can participate in a grep
// for our server, that is set at 10,000 files
// we circumvent this by chunking up the grep call into a series of greps operating on a subset of depot
// we scale this horizontally with each chunk running in its own goroutine
// status data is atomically updated and can be rendered by callee to display progress of this long running operation
func grepChunker(p4 *impl, ctx *grepContext, pattern string, depotPath string, caseSensitive bool) error {
	sizes, err := p4.Sizes(depotPath)
	if err != nil {
		return err
	}
	var groups [][]Size
	var dirs []Size
	const limit = 10000

	if sizes.TotalFileSize < limit {
		dirs = append(dirs, sizes.Sizes[0])
	} else {
		d, err := p4.Dirs(depotPath)
		if err != nil {
			return err
		}
		for i := range d {
			d[i] += "/..."
		}
		if strings.HasSuffix(depotPath, "...") && len(depotPath) > 5 {
			d = append(d, depotPath[:len(depotPath)-3]+"*")
		}
		if sizes, err = p4.Sizes(d...); err != nil {
			return err
		}

		var totalFileCount uint64
		sort.Slice(sizes.Sizes, func(i, j int) bool {
			return sizes.Sizes[i].FileCount < sizes.Sizes[j].FileCount
		})
		for _, fs := range sizes.Sizes {
			// individual size > limit, need to subdivide
			if fs.FileCount > limit {
				ctx.wg.Add(1)
				go func(d string) {
					defer ctx.wg.Done()
					grepChunker(p4, ctx, pattern, d, caseSensitive)
				}(fs.DepotPath)
				continue
			}
			if totalFileCount+fs.FileCount > limit {
				// if already have dirs under limit, process them
				if len(dirs) > 0 {
					groups = append(groups, dirs)
				}
				totalFileCount = 0
				totalFileCount = 0
				dirs = nil
			}
			if fs.FileCount > 0 {
				dirs = append(dirs, fs)
				totalFileCount += fs.FileCount
			}
		}
		if len(dirs) > 0 {
			groups = append(groups, dirs)
		}
	}
	for _, g := range groups {
		var dps = make([]string, len(g))
		for i, entry := range g {
			atomic.AddUint64(&ctx.status.FilesChecked, entry.FileCount)
			atomic.AddUint64(&ctx.status.BytesChecked, entry.FileSize)
			dps[i] = entry.DepotPath
		}
		ctx.wg.Add(1)
		go func(dps []string) {
			defer ctx.wg.Done()
			g, e := p4.Grep(pattern, caseSensitive, dps...)
			if e == nil {
				ctx.status.GrepsChan <- g
			} else {
				log.Println("Grep error", e)
			}
		}(dps)
	}
	return nil
}

// GrepLarge operates on a large dataset, and will chunk up the dataset and issue subcalls
// results of all subcalls are collated and returned via a channel in GrepStatus
func (p4 *impl) GrepLarge(pattern string, depotPath string, caseSensitive bool, status *GrepStatus) error {
	var ctx grepContext
	status.BytesChecked = 0
	status.FilesChecked = 0
	ctx.status = status
	st, err := p4.Sizes(fmt.Sprintf("%s/...", depotPath))
	if err != nil {
		return err
	}
	status.Total.FileCount = st.TotalFileCount
	status.Total.FileSize = st.TotalFileSize
	grepChunker(p4, &ctx, pattern, fmt.Sprintf("%s/...", depotPath), caseSensitive)
	ctx.wg.Wait()
	return nil
}

// Index adds keywords to the p4 index identified by name/attrib.
func (p4 *impl) Index(name string, attrib int, values ...string) error {
	input := bytes.NewBufferString(strings.Join(values, " "))
	b := buffer{input: input}
	return p4.runCmdCb(&b, "index", "-a", fmt.Sprintf("%d", attrib), name)
}

// IndexDelete removes keywords from the p4 index identified by name/attrib.
func (p4 *impl) IndexDelete(name string, attrib int, values ...string) error {
	input := bytes.NewBufferString(strings.Join(values, " "))
	b := buffer{input: input}
	return p4.runCmdCb(&b, "index", "-a", fmt.Sprintf("%d", attrib), "-d", name)
}

// Info executes the "p4 info" command which returns details about the current session
func (p4 *impl) Info() (*Info, error) {
	out, err := p4.ExecCmd("info")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(out, "\n")
	config := make(map[string]string)
	for _, line := range lines {
		if index := strings.Index(line, ":"); index >= 0 {
			key := line[:index]
			value := strings.TrimSpace(line[index+1:])
			config[key] = value
		}
	}
	return &Info{
		Client: config["Client name"],
		Host:   config["Client host"],
		Root:   config["Client root"],
		User:   config["User name"],
	}, nil
}

// Line is "<DEPOT_PATH>#<REVISION> - <LOCAL_PATH>"
var haveRegex = regexp.MustCompile(`^(.+)#(\d+) - (.+)`)

// haveParse only obtains the files in which Perforce returns a structured output for them
// in the format "<DEPOT_PATH>#<REVISION> - <LOCAL_PATH>", which corresponds to a local file
// correctly synced. Outside of that case, Perforce has a lot of different failure cases that expose
// different output: file not found, file not in client, etc. and tracking each little difference
// is error prone and fragile. Instead we search for the known success case and assume that non
// conforming output equals to a non-have.
func haveParse(have string) ([]File, error) {
	input := strings.ReplaceAll(have, "\r", "")
	var files []File
	for i, line := range strings.Split(strings.TrimSuffix(input, "\n"), "\n") {
		matches := haveRegex.FindStringSubmatch(line)
		// We are best effort in trying to parse lines.
		if matches == nil {
			continue
		}
		revision, err := strconv.Atoi(matches[2])
		if err != nil {
			return nil, fmt.Errorf("wrong revision at line %d (%s): %v", i, line, err)
		}
		local, err := filepath.Abs(matches[3])
		if err != nil {
			return nil, fmt.Errorf("wrong local path at line %d (%s): %v", i, line, err)
		}
		files = append(files, File{
			DepotPath: matches[1],
			Revision:  revision,
			LocalPath: local,
		})
	}
	return files, nil
}

func (p4 *impl) Have(patterns ...string) ([]File, error) {
	// Users can call this with thousands of patterns which would blow the command line limit for
	// Windows, so we create a temp file for holding the command.
	file, err := ioutil.TempFile("", "have_invocation")
	if err != nil {
		return nil, fmt.Errorf("could not create temp file for have invocation: %v", err)
	}
	defer os.Remove(file.Name())
	abs, err := filepath.Abs(file.Name())
	if err != nil {
		return nil, fmt.Errorf("could not obtain abs path for temp file: %v", err)
	}
	// Write the files to check into the temp file.
	for _, pattern := range patterns {
		_, err = file.WriteString(fmt.Sprintf("%s\n", pattern))
		if err != nil {
			return nil, fmt.Errorf("could not write into temp file: %v", err)
		}
	}
	// -x is a flag to load arguments from a file.
	out, err := p4.ExecCmd("-x", abs, "have")
	if err != nil {
		return nil, fmt.Errorf("error running have (%v): %s", err, out)
	}
	return haveParse(out)
}

var p4OpenedRe = regexp.MustCompile(`(//[^#]+)#[0-9]+ - (\S+) (\S+) (\S+) \((\S+)\)`)

func (p4 *impl) Opened(change string) ([]OpenedFile, error) {
	args := []string{"opened"}
	if change != "" {
		args = append(args, "-c", change)
	}
	out, err := p4.ExecCmd(args...)
	if err != nil {
		return nil, err
	}
	var ret []OpenedFile
	for _, line := range strings.Split(out, "\n") {
		if m := p4OpenedRe.FindStringSubmatch(line); m != nil {
			if len(m) != 6 {
				return nil, fmt.Errorf("incorrect number of subgroups in %s", m[0])
			}
			at, err := GetActionType(m[2])
			if err != nil {
				return nil, fmt.Errorf("unhandled action type %s", line)
			}
			var cl int
			change := m[3]
			switch change {
			case "change":
				cl, err = strconv.Atoi(m[4])
				if err != nil {
					return nil, fmt.Errorf("could not parse %s: %v", line, err)
				}
			case "default":
				cl = 0
			default:
				return nil, fmt.Errorf("could not parse %s", line)
			}
			ft, err := GetFileType(m[5])
			ret = append(ret, OpenedFile{
				Path:   m[1],
				Status: at,
				CL:     cl,
				Type:   ft,
			})
		}
	}
	return ret, nil
}

func (p4 *impl) Ignores(paths []string) (string, error) {
	args := []string{"ignores"}

	args = append(args, "-i")

	args = append(args, paths...)
	return p4.ExecCmd(args...)
}

// Reconcile invokes "p4 reconcile" and marks the inconsistencies between the workspace and the depot
func (p4 *impl) Reconcile(paths []string, cl int) (string, error) {
	args := []string{"reconcile"}
	if cl != 0 {
		args = append(args, "-c", strconv.Itoa(cl))
	}
	args = append(args, paths...)
	return p4.ExecCmd(args...)
}

// Revert invokes "p4 revert" on the given files.
func (p4 *impl) Revert(paths []string, opts ...string) (string, error) {
	args := []string{"revert"}
	args = append(args, opts...)
	args = append(args, paths...)
	return p4.ExecCmd(args...)
}

func (p4 *impl) Set(key, value string) error {
	cmd := []string{"set", fmt.Sprintf("%s=%s", key, value)}
	if out, err := p4.ExecCmd(cmd...); err != nil {
		return fmt.Errorf("%s", out)
	}
	return nil
}

// Sizes invokes "p4 sizes" and returns info about file sizes and counts
func (p4 *impl) Sizes(dirs ...string) (*SizeCollection, error) {
	sc := &SizeCollection{}

	cmd := []string{"sizes", "-s"}
	cmd = append(cmd, dirs...)
	out, err := p4.ExecCmd(cmd...)
	lines := strings.Split(out, "\n")
	if len(lines) < 1 {
		return nil, fmt.Errorf("couldn't read sizes")
	}
	for _, line := range lines {
		words := strings.Split(line, " ")
		if len(words) < 5 {
			continue
		}
		fc, err := strconv.Atoi(words[len(words)-5])
		if err != nil {
			continue
		}
		fs, err := strconv.Atoi(words[len(words)-3])
		if err != nil {
			continue
		}
		sc.Sizes = append(sc.Sizes, Size{
			DepotPath: words[0],
			FileCount: uint64(fc),
			FileSize:  uint64(fs),
		})
		sc.TotalFileCount += sc.Sizes[len(sc.Sizes)-1].FileCount
		sc.TotalFileSize += sc.Sizes[len(sc.Sizes)-1].FileSize
	}
	return sc, err
}

func (p4 *impl) Submit(cl int, options ...string) (string, error) {
	cmd := []string{"submit"}
	cmd = append(cmd, "-c", strconv.Itoa(cl))
	cmd = append(cmd, options...)
	return p4.ExecCmd(cmd...)
}

func (p4 *impl) Sync(targets []string, options ...string) (string, error) {
	cmd := []string{"sync"}
	cmd = append(cmd, options...)
	cmd = append(cmd, targets...)
	return p4.ExecCmd(cmd...)
}

// SyncSize implements the p4lib.SyncSize interface method.
func (p4 *impl) SyncSize(targets []string) (*SyncSize, error) {
	if targets == nil {
		targets = []string{"//..."}
	}
	cmd := []string{"sync", "-N"}
	cmd = append(cmd, targets...)
	out, err := p4.ExecCmd(cmd...)
	if err != nil {
		return nil, err
	}
	return syncSizeParse(out)
}

var syncSizeRegex = regexp.MustCompile(`=(\d+)/(\d+)/(\d+), .+=(\d+)/(\d+)`)

func syncSizeParse(line string) (*SyncSize, error) {
	line = strings.TrimSpace(line)
	matches := syncSizeRegex.FindStringSubmatch(line)
	if matches == nil {
		return nil, fmt.Errorf("invalid sync size line: %s", line)
	}
	filesAdded, _ := strconv.ParseInt(matches[1], 10, 64)
	filesUpdated, _ := strconv.ParseInt(matches[2], 10, 64)
	filesDeleted, _ := strconv.ParseInt(matches[3], 10, 64)
	bytesAdded, _ := strconv.ParseInt(matches[4], 10, 64)
	bytesDeleted, _ := strconv.ParseInt(matches[5], 10, 64)
	return &SyncSize{
		FilesAdded:   filesAdded,
		FilesUpdated: filesUpdated,
		FilesDeleted: filesDeleted,
		BytesAdded:   bytesAdded,
		BytesDeleted: bytesDeleted,
	}, nil
}

// Tickets invokes "p4 tickets" and returns a list of open tickets
func (p4 *impl) Tickets(args ...string) ([]Ticket, error) {
	cmd := []string{"tickets"}
	cmd = append(cmd, args...)
	out, err := p4.ExecCmd(cmd...)
	if err != nil {
		return nil, err
	}

	var tickets []Ticket
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		words := strings.Split(line, " ")
		if len(words) >= 3 {
			tickets = append(tickets, Ticket{
				Name: words[0],
				User: words[1][1 : len(words[1])-1],
				ID:   strings.TrimSpace(words[2]),
			})
		}
	}

	return tickets, nil
}

func (p4 *impl) Trust(args ...string) error {
	cmd := []string{"trust"}
	cmd = append(cmd, args...)
	if out, err := p4.ExecCmd(cmd...); err != nil {
		return fmt.Errorf("%s", out)
	}
	return nil
}

// Unshelve implements p4.Unshelve interface method.
func (p4 *impl) Unshelve(cl int, args ...string) (string, error) {
	cmdArgs := []string{"unshelve", "-s", fmt.Sprintf("%d", cl)}
	cmdArgs = append(cmdArgs, args...)
	return p4.ExecCmd(cmdArgs...)
}

// Users executes the P4 Users command and returns a list of users belonging to current perforce server
func (p4 *impl) Users() ([]User, error) {
	out, err := p4.ExecCmd("users")
	if err != nil {
		return nil, err
	}

	var users []User
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		// formt of output line is:
		// username <email@domain> (real name) accessed YYYY/MM/DD
		words := strings.Split(line, " ")
		if len(words) >= 5 {
			u := words[0]
			e := words[1]
			n := words[2]
			for i := 3; i < len(words)-2; i++ {
				n += words[i]
			}
			d := words[len(words)-1]
			user := User{
				User:     u,
				Email:    e,
				Name:     n,
				Accessed: d,
			}
			users = append(users, user)
		}
	}

	return users, nil
}

func userClientBuild(combined string) UserClient {
	var uc UserClient

	splits := strings.Split(combined, "@")
	if len(splits) > 1 {
		uc.User = splits[0]
		uc.Client = splits[1]
	}

	return uc
}

// verifyCL compares the files within a CL (clFiles) against the files currently synced in the
// client (clientFiles). If Any clientFile is a newer revision than the one on the CL, this will
// fail.
func verifyCL(clFiles []FileAction, clientFiles []File) error {
	clMap := map[string]FileAction{}
	for _, file := range clFiles {
		clMap[file.DepotPath] = file
	}
	clientMap := map[string]File{}
	for _, file := range clientFiles {
		clientMap[file.DepotPath] = file
	}
	for depotPath, clFile := range clMap {
		clientFile, ok := clientMap[depotPath]
		// If we don't have information about this file, we don't bother in checking for new files.
		if !ok {
			continue
		}
		if clientFile.Revision > clFile.Revision {
			return fmt.Errorf("Change contains out of date files. You must either sync/resolve or revert them before presubmit checks can run. (%s is newer than CL: rev %d vs CL %d)",
				depotPath, clientFile.Revision, clFile.Revision)
		}
	}
	return nil
}

func (p4 *impl) VerifiedUnshelve(cl int) (string, error) {
	describes, err := p4.DescribeShelved(cl)
	if err != nil {
		return "", fmt.Errorf("could not describe CL %d: %v", cl, err)
	}
	describe := describes[0]
	var patterns []string
	for _, file := range describe.Files {
		patterns = append(patterns, file.DepotPath)
	}
	clientFiles, err := p4.Have(patterns...)
	if err != nil {
		return "", fmt.Errorf("have failed: %v", err)
	}
	if err := verifyCL(describe.Files, clientFiles); err != nil {
		return "", fmt.Errorf("could not verify CL: %v", err)
	}
	args := []string{"-f"}
	out, err := p4.Unshelve(cl, args...)
	if err != nil {
		return "", fmt.Errorf("could not unshelve: %v", err)
	}
	return out, nil
}

// Where executes a p4 where and returns the absolute local path that relates to the specified depot path
func (p4 *impl) Move(cl int, from string, to string) (string, error) {
	stdOutErr, err := p4.ExecCmd("move", "-c", fmt.Sprintf("%d", cl), from, to)
	if err != nil {
		return stdOutErr, err
	}

	lines := strings.Split(string(stdOutErr), "\n")
	if len(lines) >= 1 {
		words := strings.Split(lines[0], " ")
		if len(words) >= 3 {
			return strings.TrimSpace(words[2]), nil
		}
	}

	return "", fmt.Errorf("couldn't move files from %s to %s", from, to)
}

// The following methods collect potentially useful statistics about usage.
var lockStats sync.Mutex

func updateStat(key string, execUs int64) {
	lockStats.Lock()
	defer lockStats.Unlock()

	stat, ok := Stats[key]
	if !ok {
		stat.MinUs = math.MaxInt64
	}
	stat.Count = stat.Count + 1
	if execUs < stat.MinUs {
		stat.MinUs = execUs
	}
	if execUs > stat.MaxUs {
		stat.MaxUs = execUs
	}
	stat.TotalUs = stat.TotalUs + execUs
	Stats[key] = stat
}

func updateStats(cmd string, execUs int64, initUs int64) {
	// Update stats asynchronously -- no reason an Exec should stall for this.
	go func() {
		updateStat(cmd, execUs)
		if initUs > 0 {
			updateStat("_initconn_", initUs)
		}
	}()
}

var idxRegexp = regexp.MustCompile(`^(\D+)(\d+)$`)

func setTaggedField(s interface{}, key, value string, mustMatch bool) error {
	r := reflect.Indirect(reflect.ValueOf(s))
	field, err := findField(r, key)
	if err != nil {
		if mustMatch {
			return err
		}
		return nil
	}
	rv, err := reflectValueBuild(field.Type(), value)
	if err != nil {
		return fmt.Errorf("couldn't convert '%s' to '%v': %v", value, field.Type(), err)
	}
	field.Set(rv)

	return nil
}

func findField(rv reflect.Value, key string) (reflect.Value, error) {
	if rv.Kind() != reflect.Struct {
		return reflect.Value{}, fmt.Errorf("value must be struct, got %v", rv.Kind())
	}
	idx := -1
	matches := idxRegexp.FindStringSubmatch(key)
	if matches != nil {
		if i, err := strconv.Atoi(matches[2]); err != nil {
			glog.Infof("'%s' atoi failed: %v", matches[2], err)
		} else {
			idx = i
		}
	}
	for i := 0; i < rv.NumField(); i++ {
		tags := rv.Type().Field(i).Tag.Get("p4")

		if tags == "" {
			tags = rv.Type().Field(i).Name
			tags = strings.ToLower(tags[:1]) + tags[1:]
			if rv.Field(i).Kind() == reflect.Slice {
				tags = "[" + tags + "]"
			}
		}
		if tags == key {
			return rv.Field(i), nil
		}
		if idx >= 0 && tags[0] == '[' && rv.Field(i).Kind() == reflect.Slice {
			// Look for a match in the slice.
			tags = strings.Trim(tags, "[]")
			names := strings.Split(tags, ",")
			for _, name := range names {
				if name == matches[1] {
					// We've found the proper field.
					// Ensure the slice is large enough.
					for idx >= rv.Field(i).Len() {
						rv.Field(i).Set(reflect.Append(rv.Field(i), reflect.Zero(rv.Field(i).Type().Elem())))
					}
					f := rv.Field(i).Index(idx)
					if f.Kind() == reflect.Struct {
						return findField(rv.Field(i).Index(idx), name)
					}
					return f, nil
				}
			}
		}
	}
	return reflect.Value{}, fmt.Errorf("no matching field for %s", key)
}

func reflectValueBuild(t reflect.Type, value string) (reflect.Value, error) {
	switch t.Kind() {
	case reflect.Bool:
		return reflect.ValueOf(true), nil
	case reflect.Int:
		var v int
		var err error
		if value != "" {
			v, err = strconv.Atoi(value)
		}
		if err != nil {
			return reflect.Value{}, fmt.Errorf("couldn't convert to int *%s*", value)
		}
		return reflect.ValueOf(v), nil
	case reflect.Int64:
		var v int64
		var err error
		if value != "" {
			v, err = strconv.ParseInt(value, 0, 64)
		}
		if err != nil {
			return reflect.Value{}, fmt.Errorf("couldn't convert to int64 *%s#", value)
		}
		return reflect.ValueOf(v), nil
	case reflect.String:
		return reflect.ValueOf(value), nil
	}
	return reflect.Value{}, fmt.Errorf("couldn't convert value for type %v with value %v", t, value)
}
