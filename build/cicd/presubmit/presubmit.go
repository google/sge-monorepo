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

/*
Package presubmit holds the logic with dealing with presubmits.

Here you will find functionality for searching for cicd files, extracting
checks for them and obtaining checks to run.
*/
package presubmit

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
	"syscall"

	"sge-monorepo/build/cicd/cicdfile"
	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/monorepo/p4path"
	"sge-monorepo/build/cicd/monorepo/universe"
	"sge-monorepo/build/cicd/sgeb/build"
	"sge-monorepo/libs/go/p4lib"

	"sge-monorepo/build/cicd/presubmit/check/protos/checkpb"
	"sge-monorepo/build/cicd/presubmit/protos/presubmitpb"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"

	"github.com/golang/protobuf/proto"
	gouuid "github.com/nu7hatch/gouuid"
)

// Runner is a presubmit runner.
type Runner interface {
	// Run executes a presubmit run in the current CL. If there is no error, returns whether the
	// presubmit run was successful or not.
	Run() (bool, error)
}

// Listener is a receiver of presubmit events.
type Listener interface {
	// OnSetStart is called on the start of a presubmit set.
	OnPresubmitStart(mr monorepo.Monorepo, presubmitId string, checks []Check)

	// OnCheckStart is called when a check starts.
	OnCheckStart(check Check)

	// OnCheckResult is called after a check has been completed.
	OnCheckResult(mdPath monorepo.Path, check Check, result *presubmitpb.CheckResult)

	// OnPresubmitEnd is called after the whole presubmit run has been completed.
	OnPresubmitEnd(success bool)
}

// Option is a function that modifies the Options structure on NewRunner.
type Option func(*Options)

// Options contains running options for Runner.
type Options struct {
	// Logs is a writer that all process output goes to. Defaults to stderr.
	Logs io.Writer

	// BazelStartupArgs is a set of options to be given to Bazel. This are options that are set *before*
	// the command. Eg: bazel {BazelStartupArgs} test ...
	BazelStartupArgs []string

	// BazelBuildArgs is a set of options to be given into bazel on a build command. Note that this are
	// options that are set *after* the command. For bazel startup options, see |BazelStartupArgs|.
	// Eg: bazel build {BazelBuildArgs} //some/target
	BazelBuildArgs []string

	// FixOnly filters checks to fix-capable ones.
	FixOnly bool

	// Change filters checks based on only the supplied change.
	// Either a number or default can be passed.
	Change string

	// CLDescription is the description to pass to checks.
	// If empty, checks with 'needs_cl_description' are not run.
	CLDescription string

	// LogLevel is a glog severity level. Default is ERROR.
	LogLevel string

	// PresubmitId is a GUID that identifies the presubmit. It is attached to cloud log entries.
	PresubmitId string

	// Listeners get presubmit events defined by the Listener interface.
	Listeners []Listener
}

// funcWriter is a simple wrapper to enable functions to be exposed as Writers.
type funcWriter func(p []byte) (n int, err error)

func (f funcWriter) Write(p []byte) (n int, err error) {
	return f(p)
}

// NewRunner returns a new presubmit runner.
func NewRunner(u universe.Universe, p4 p4lib.P4, mdProvider cicdfile.Provider, opts ...Option) Runner {
	options := Options{
		Logs:     os.Stderr,
		LogLevel: "ERROR",
	}
	for _, opt := range opts {
		opt(&options)
	}
	return &runner{
		universe:   u,
		p4:         p4,
		mdProvider: mdProvider,
		options:    options,
	}
}

type runner struct {
	universe   universe.Universe
	p4         p4lib.P4
	mdProvider cicdfile.Provider
	options    Options
}

// triggeredSet is a set of triggered presubmits in a monorepo.
type triggeredSet struct {
	runner      *runner
	monorepo    monorepo.Monorepo
	monorepoDef universe.MonorepoDef
	tools       map[string]checkerTool
	triggered   []triggered
}

func (ts *triggeredSet) String() string {
	var sb strings.Builder
	_, _ = fmt.Fprintf(&sb, "Monorepo: %+v\n", ts.monorepo)
	_, _ = fmt.Fprintf(&sb, "Def: %+v\n", ts.monorepoDef)
	_, _ = fmt.Fprintf(&sb, "Tools:\n")
	for _, tool := range ts.tools {
		_, _ = fmt.Fprintf(&sb, "- %+v\n", tool)
	}
	_, _ = fmt.Fprintf(&sb, "Triggered Presubmits:\n")
	for _, t := range ts.triggered {
		_, _ = fmt.Fprintf(&sb, "- Dir: %s\n", t.psDir)
		_, _ = fmt.Fprintf(&sb, "  - Files: %v\n", t.matchingFiles)
		for _, c := range t.presubmit.Check {
			_, _ = fmt.Fprintf(&sb, "  - Check: %s, args: %s\n", c.Action, c.Args)
		}
		for _, c := range t.presubmit.CheckBuild {
			_, _ = fmt.Fprintf(&sb, "  - CheckBuild: %s\n", c.BuildUnit)
		}
		for _, c := range t.presubmit.CheckTest {
			_, _ = fmt.Fprintf(&sb, "  - CheckTest: %s\n", c.TestUnit)
		}
	}
	return sb.String()
}

// triggered is a presubmit that was triggered by the changed files.
type triggered struct {
	presubmit     *presubmitpb.Presubmit
	psDir         monorepo.Path
	mdPath        monorepo.Path
	matchingFiles []changedFile
}

type checkerTool struct {
	// dir is the monorepo Path of the tool textpb.
	dir monorepo.Path

	// toolPb is the checker tool from the tool textpb.
	toolPb *checkpb.CheckerTool
}

type changedFile struct {
	path   monorepo.Path
	status p4lib.ActionType
}

func (r *runner) Run() (bool, error) {
	if r.options.PresubmitId == "" {
		r.options.PresubmitId = newUuid()
	}
	sets, err := r.analyzeChange()
	if err != nil {
		return false, err
	}
	overallSuccess := true
	for _, ts := range sets {
		if success, err := ts.run(); err != nil {
			return false, err
		} else {
			overallSuccess = overallSuccess && success
		}
	}
	return overallSuccess, nil
}

// analyzeChange returns all triggered presubmit sets in the depot based on the current p4 state.
func (r *runner) analyzeChange() ([]triggeredSet, error) {
	depotPaths, err := r.p4.Opened(r.options.Change)
	if err != nil {
		return nil, err
	}
	// Sort affected files into sets by monorepo
	// This is O(N^2), but if we don't have very many monorepos it's probably fine.
	var result []triggeredSet
	for _, mrDef := range r.universe.Udef {
		var files []changedFile
		mrPrefix := mrDef.Root + "/"
		for _, dp := range depotPaths {
			if strings.HasPrefix(dp.Path, mrPrefix) {
				files = append(files, changedFile{
					path:   monorepo.NewPath(dp.Path[len(mrDef.Root)+1:]),
					status: dp.Status,
				})
			}
		}
		if len(files) == 0 {
			continue
		}
		mr, err := mrDef.Resolve(r.p4)
		if err != nil {
			return nil, fmt.Errorf("could not resolve monorepo: %v", err)
		}
		triggered, err := r.findTriggered(mr, files)
		if err != nil {
			return nil, err
		}
		if len(triggered) == 0 {
			continue
		}
		tools, err := toolsForMonorepo(mrDef, mr)
		if err != nil {
			return nil, err
		}
		result = append(result, triggeredSet{
			runner:      r,
			monorepo:    mr,
			monorepoDef: mrDef,
			tools:       tools,
			triggered:   triggered,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].monorepo.Root < result[j].monorepo.Root
	})
	return result, nil
}

// findTriggered returns all triggered presubmits within a monorepo.
func (r *runner) findTriggered(mr monorepo.Monorepo, files []changedFile) ([]triggered, error) {
	var paths []monorepo.Path
	for _, p := range files {
		paths = append(paths, p.path)
	}
	mdFiles, err := r.mdProvider.FindCicdFiles(mr, paths)
	if err != nil {
		return nil, err
	}
	var ret []triggered
	for _, md := range mdFiles {
		psDir := md.Path.Dir()
		for _, ps := range md.Proto.Presubmit {
			matcher, err := newMatcher(mr, psDir, ps)
			if err != nil {
				return nil, err
			}
			var matchingFiles []changedFile
			for _, f := range files {
				if match, err := matcher.match(f.path); match {
					matchingFiles = append(matchingFiles, f)
				} else if err != nil {
					return nil, err
				}
			}
			if len(matchingFiles) == 0 {
				continue
			}
			sort.Slice(matchingFiles, func(i, j int) bool {
				return matchingFiles[i].path < matchingFiles[j].path
			})
			ret = append(ret, triggered{
				presubmit:     ps,
				psDir:         psDir,
				matchingFiles: matchingFiles,
				mdPath:        md.Path,
			})
		}
	}
	return ret, nil
}

// matcher provides presubmit path matching.
type matcher struct {
	includes []p4path.Expr
	excludes []p4path.Expr
}

func newMatcher(mr monorepo.Monorepo, dir monorepo.Path, ps *presubmitpb.Presubmit) (matcher, error) {
	var includes []p4path.Expr
	if len(ps.Include) > 0 {
		var err error
		includes, err = pathExprs(mr, dir, ps.Include)
		if err != nil {
			return matcher{}, fmt.Errorf("error in %s/CICD: %v", dir, err)
		}
	} else {
		// If there are no includes expressions, "..." is assumed.
		pe, _ := p4path.NewExpr(mr, dir, "...")
		includes = []p4path.Expr{pe}
	}
	excludes, err := pathExprs(mr, dir, ps.Exclude)
	if err != nil {
		return matcher{}, fmt.Errorf("error in %s/CICD: %v", dir, err)
	}
	return matcher{
		includes: includes,
		excludes: excludes,
	}, nil
}

func (m matcher) match(p monorepo.Path) (bool, error) {
	// First check for exclusion.
	for _, exclude := range m.excludes {
		match, err := exclude.Matches(p)
		if err != nil {
			return false, err
		}
		if match {
			return false, nil
		}
	}
	// Search over the inclusion expressions.
	for _, include := range m.includes {
		match, err := include.Matches(p)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

func pathExprs(mr monorepo.Monorepo, dir monorepo.Path, patterns []string) ([]p4path.Expr, error) {
	var ret []p4path.Expr
	for _, p := range patterns {
		expr, err := p4path.NewExpr(mr, dir, p)
		if err != nil {
			return nil, nil
		}
		ret = append(ret, expr)
	}
	return ret, nil
}

// toolsForMonorepo loads all checker tool definitions pointed to by the monorepo def.
func toolsForMonorepo(mrDef universe.MonorepoDef, mr monorepo.Monorepo) (map[string]checkerTool, error) {
	ret := map[string]checkerTool{}
	for _, tcPath := range mrDef.ToolConfigs {
		mrp := monorepo.NewPath(tcPath)
		toolBytes, err := ioutil.ReadFile(mr.ResolvePath(mrp))
		if err != nil {
			return nil, fmt.Errorf("could not find checker tools %s: %v", mrp, err)
		}
		toolsProto := checkpb.CheckerTools{}
		if err := proto.UnmarshalText(string(toolBytes), &toolsProto); err != nil {
			return nil, fmt.Errorf("could not unmarshal checker tools %s: %v", mrp, err)
		}
		for _, tool := range toolsProto.CheckerTool {
			ret[tool.Action] = checkerTool{mrp.Dir(), tool}
		}
	}
	return ret, nil
}

// Check is a check that will be run by the presubmit system.
type Check interface {
	// Id is a unique id for the test.
	Id() string

	// Name is the name of the check.
	Name() string

	// CicdFilePath is the path ot the CICD file containing the check.
	CicdFilePath() monorepo.Path

	// Run runs the check.
	Run(bc build.Context) (*presubmitpb.CheckResult, error)

	// SordOrder returns an order for the check to be sorted in.
	// Used to run batch checks by their Bazel flags.
	SortOrder() sortOrder
}

type sortOrder []string

func cmpCheck(lhs, rhs Check) bool {
	lhsOrder := lhs.SortOrder()
	rhsOrder := rhs.SortOrder()
	if len(lhsOrder) < len(rhsOrder) {
		return false
	}
	if len(rhsOrder) < len(lhsOrder) {
		return true
	}
	for i := 0; i < len(lhsOrder); i++ {
		if lhsOrder[i] < rhsOrder[i] {
			return false
		}
	}
	return false
}

// runSet runs all presubmits in a set. If there is no error, returns whether the presubmit run was
// successful or not.
func (ts *triggeredSet) run() (bool, error) {
	bc, err := build.NewContext(ts.monorepo, func(opts *build.Options) {
		opts.Logs = ts.runner.options.Logs
		opts.LogLevel = ts.runner.options.LogLevel
		opts.BazelStartupArgs = ts.runner.options.BazelStartupArgs
		opts.BazelBuildArgs = ts.runner.options.BazelBuildArgs
	})
	if err != nil {
		return false, fmt.Errorf("could not create build context: %v", err)
	}
	defer bc.Cleanup()
	presubmitId := ts.runner.options.PresubmitId

	// Discover the checks that will be run.
	var checks []Check
	seen := map[monorepo.Label]bool{}
	for _, t := range ts.triggered {
		for _, c := range t.presubmit.Check {
			id := newUuid()
			name := fmt.Sprintf("check %s", c.Action)
			tool, ok := ts.tools[c.Action]
			if !ok {
				checks = append(checks, &failCheck{
					checkBase: checkBase{id, presubmitId, name, t.mdPath},
					err:       fmt.Errorf("no such registered action %q", c.Action),
				})
				continue
			}
			// If we are running in fix mode and the check doesn't support it, bail.
			if ts.runner.options.FixOnly && !tool.toolPb.SupportsFix {
				continue
			}
			if ts.runner.options.CLDescription == "" && tool.toolPb.NeedsClDescription {
				continue
			}
			checks = append(checks, &checkAction{
				checkBase:    checkBase{id, presubmitId, name, t.mdPath},
				check:        c,
				tool:         tool,
				triggered:    t,
				triggeredSet: ts,
			})
		}

		// check_build and check_test doesn't support fixes.
		if ts.runner.options.FixOnly {
			continue
		}

		// check_build
		for _, c := range t.presubmit.CheckBuild {
			id := newUuid()
			buLabel, err := ts.monorepo.NewLabel(t.psDir, c.BuildUnit)
			if err != nil {
				id := newUuid()
				name := fmt.Sprintf("check_build %s", c.BuildUnit)
				checks = append(checks, &failCheck{
					checkBase: checkBase{id, presubmitId, name, t.mdPath},
					err:       err,
				})
				continue
			}
			if _, ok := seen[buLabel]; ok {
				// Already ran this check
				continue
			}
			seen[buLabel] = true
			name := fmt.Sprintf("check_build %s", buLabel)
			sortOrder, err := bc.BazelArgs(buLabel)
			if err != nil {
				checks = append(checks, &failCheck{
					checkBase: checkBase{id, presubmitId, name, t.mdPath},
					err:       err,
				})
				continue
			}
			checks = append(checks, &checkBuild{
				checkBase: checkBase{id, presubmitId, name, t.mdPath},
				label:     buLabel,
				sortOrder: sortOrder,
			})
		}

		// check_test
		for _, c := range t.presubmit.CheckTest {
			id := newUuid()
			name := fmt.Sprintf("check_test %s", c.TestUnit)
			tuLabel, err := ts.monorepo.NewLabel(t.psDir, c.TestUnit)
			if err != nil {
				checks = append(checks, &failCheck{
					checkBase: checkBase{id, presubmitId, name, t.mdPath},
					err:       err,
				})
				continue
			}
			if _, ok := seen[tuLabel]; ok {
				// Already ran this check
				continue
			}
			seen[tuLabel] = true
			testUnits, err := bc.ExpandTargetExpression(monorepo.TargetExpression(tuLabel.String()))
			if err != nil {
				checks = append(checks, &failCheck{
					checkBase: checkBase{id, presubmitId, name, t.mdPath},
					err:       err,
				})
				continue
			}
			for _, tu := range testUnits {
				if _, ok := seen[tu]; ok && tu != tuLabel {
					// Already ran this check
					continue
				}
				seen[tu] = true
				id := newUuid()
				name := fmt.Sprintf("check_test %s", tu)
				sortOrder, err := bc.BazelArgs(tu)
				if err != nil {
					checks = append(checks, &failCheck{
						checkBase: checkBase{id, presubmitId, name, t.mdPath},
						err:       err,
					})
					continue
				}
				checks = append(checks, &checkTest{
					checkBase: checkBase{id, presubmitId, name, t.mdPath},
					label:     tu,
					sortOrder: sortOrder,
				})
			}
		}
	}

	sort.Slice(checks, func(i, j int) bool {
		return cmpCheck(checks[i], checks[j])
	})

	// Run checks.
	success := true
	listeners := ts.runner.options.Listeners
	for _, l := range listeners {
		l.OnPresubmitStart(ts.monorepo, presubmitId, checks)
	}
	for _, c := range checks {
		for _, l := range listeners {
			l.OnCheckStart(c)
		}
		result, err := c.Run(bc)
		if err != nil {
			result = errResult(c.Name(), err)
		}
		success = success && result.OverallResult.Success
		for _, l := range listeners {
			l.OnCheckResult(c.CicdFilePath(), c, result)
		}
	}
	for _, l := range listeners {
		l.OnPresubmitEnd(success)
	}
	return success, nil
}

type checkBase struct {
	id          string
	presubmitId string
	name        string
	mdPath      monorepo.Path
}

func (cb *checkBase) Id() string {
	return cb.id
}

func (cb *checkBase) Name() string {
	return cb.name
}

func (cb *checkBase) CicdFilePath() monorepo.Path {
	return cb.mdPath
}

type checkBuild struct {
	checkBase
	label     monorepo.Label
	sortOrder []string
}

func (cb *checkBuild) Run(bc build.Context) (*presubmitpb.CheckResult, error) {
	buildResult, err := bc.Build(cb.label, func(options *build.Options) {
		options.LogLabels = checkLogLabels(cb.id, cb.presubmitId)
	})
	if err != nil && buildResult == nil {
		return nil, err
	} else if buildResult == nil {
		return nil, errors.New("no error or build result returned")
	}
	return &presubmitpb.CheckResult{
		OverallResult: &buildpb.Result{
			Name:    cb.name,
			Success: buildResult.OverallResult.Success,
			Cause:   buildResult.OverallResult.Cause,
			Logs:    buildResult.OverallResult.Logs,
		},
		SubResults: []*buildpb.Result{
			buildResult.BuildResult.Result,
		},
	}, nil
}

func (cb *checkBuild) SortOrder() sortOrder {
	return cb.sortOrder
}

type checkTest struct {
	checkBase
	label     monorepo.Label
	sortOrder []string
}

func (ct *checkTest) Run(bc build.Context) (*presubmitpb.CheckResult, error) {
	testResult, err := bc.Test(ct.label, func(options *build.Options) {
		options.LogLabels = checkLogLabels(ct.id, ct.presubmitId)
	})
	if err != nil && testResult == nil {
		return nil, err
	} else if testResult == nil {
		return nil, errors.New("no error or test result returned")
	}
	return &presubmitpb.CheckResult{
		OverallResult: &buildpb.Result{
			Name:    ct.name,
			Success: testResult.OverallResult.Success,
			Cause:   testResult.OverallResult.Cause,
			Logs:    testResult.OverallResult.Logs,
		},
		SubResults: testResult.TestResult.Results,
	}, nil
}

func (ct *checkTest) SortOrder() sortOrder {
	return ct.sortOrder
}

type checkAction struct {
	checkBase
	check        *checkpb.Check
	tool         checkerTool
	triggered    triggered
	triggeredSet *triggeredSet
}

// runCheck runs a single presubmit check.
func (ca *checkAction) Run(bc build.Context) (*presubmitpb.CheckResult, error) {
	bin, _, err := bc.ResolveBin(ca.tool.dir, ca.tool.toolPb.Bin, func(options *build.Options) {
		options.LogLabels = checkLogLabels(ca.id, ca.presubmitId)
	})
	if err != nil {
		return nil, err
	}
	var files []*checkpb.File
	for _, f := range ca.triggered.matchingFiles {
		files = append(files, &checkpb.File{
			Path:   ca.triggeredSet.monorepo.ResolvePath(f.path),
			Status: statusFromP4Status(f.status),
		})
	}
	var logLabels []*checkpb.LogLabel
	for k, v := range checkLogLabels(ca.id, ca.presubmitId) {
		logLabels = append(logLabels, &checkpb.LogLabel{Key: k, Value: v})
	}
	invocation := checkpb.CheckerInvocation{
		TriggeredChecks: []*checkpb.TriggeredCheck{
			{
				Check: ca.check,
				Dir:   string(ca.triggered.psDir),
				Files: files,
			},
		},
		ClDescription: ca.triggeredSet.runner.options.CLDescription,
		LogLabels:     logLabels,
	}
	invocationBytes, err := proto.Marshal(&invocation)
	if err != nil {
		return nil, err
	}
	tempDir, err := ioutil.TempDir("", "sgep")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	invocationPath := path.Join(tempDir, "invocation.textpb")
	resultPath := path.Join(tempDir, "invocation-result.textpb")
	if err := ioutil.WriteFile(invocationPath, invocationBytes, 0666); err != nil {
		return nil, fmt.Errorf("could not write invocation proto: %v", err)
	}
	args := []string{
		"--checker-invocation=" + invocationPath,
		"--checker-invocation-result=" + resultPath,
	}
	args = append(args, ca.check.Args...)
	args = append(args, ca.tool.toolPb.Args...)
	args = build.AddGlogFlags(ca.check.Action, ca.triggeredSet.runner.options.LogLevel, args)
	cmd := exec.Command(bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Dir = ca.triggeredSet.monorepo.Root
	var logs bytes.Buffer
	writer := io.MultiWriter(&logs, funcWriter(func(p []byte) (n int, err error) {
		return ca.triggeredSet.runner.options.Logs.Write(p)
	}))
	cmd.Stdout = writer
	cmd.Stderr = writer
	cmdErr := cmd.Run()
	resultBytes, err := ioutil.ReadFile(resultPath)
	if err != nil {
		return nil, fmt.Errorf("%v for command %s: %s", cmdErr, cmd.String(), logs.String())
	}
	result := checkpb.CheckerInvocationResult{}
	if err := proto.Unmarshal(resultBytes, &result); err != nil {
		return nil, fmt.Errorf("could not unmarshal invocation result %s: %v", resultPath, err)
	}
	success := cmdErr == nil
	for _, r := range result.Results {
		success = success && r.Success
	}
	return &presubmitpb.CheckResult{
		OverallResult: &buildpb.Result{
			Name:    ca.name,
			Success: success,
			Logs:    build.LogsFromString("stderr", logs.String()),
		},
		SubResults: result.Results,
	}, nil
}

func (ca *checkAction) SortOrder() sortOrder {
	return nil
}

type failCheck struct {
	checkBase
	err error
}

func (fa *failCheck) Run(build.Context) (*presubmitpb.CheckResult, error) {
	return nil, fa.err
}

func (fa *failCheck) SortOrder() sortOrder {
	return nil
}

func statusFromP4Status(status p4lib.ActionType) checkpb.Status {
	switch status {
	case p4lib.ActionAdd, p4lib.ActionMoveAdd, p4lib.ActionBranch:
		return checkpb.Status_Create
	case p4lib.ActionEdit, p4lib.ActionIntegrate:
		return checkpb.Status_Edit
	case p4lib.ActionDelete, p4lib.ActionMoveDelete:
		return checkpb.Status_Delete
	}
	log.Fatalf("unknown open status %s", status)
	return checkpb.Status_Edit
}

// errResult makes a presubmit check result from an error.
// This will often be caused by an internal tool error or some user error
// that renders us unable to even run the checker tool.
func errResult(name string, err error) *presubmitpb.CheckResult {
	msg := fmt.Sprintf("error: %v", err)
	if exitErr, ok := err.(*exec.ExitError); ok {
		msg = fmt.Sprintf("%s\n%s", msg, string(exitErr.Stderr))
	}
	return &presubmitpb.CheckResult{
		OverallResult: &buildpb.Result{
			Name:    name,
			Success: false,
			Logs:    build.LogsFromString("tool_error", msg),
		},
	}
}

// PrinterOpt is an option to the printer.
type PrinterOpt func(*PrinterOpts)

// LogFunc is a function that can be set for replacing normal logging.
type LogFunc func(s string)

// PrinterOpts is options to the printer.
type PrinterOpts struct {
	// Logs permits to override the default stderr printing of the |Printer|.
	Logs LogFunc

	// Verbose produces more verbose output.
	Verbose bool
}

// NewPrinter returns a presubmit listener that prints presubmit results to the console.
func NewPrinter(opts ...PrinterOpt) *Printer {
	p := &Printer{}
	for _, opt := range opts {
		opt(&p.opts)
	}
	// Verify if users overwrote the out string writer. If not we print to stderr.
	if p.opts.Logs == nil {
		p.opts.Logs = LogFunc(func(s string) {
			_, _ = fmt.Fprint(os.Stderr, s)
		})
	}
	return p
}

// Printer prints presubmit results to the console.
type Printer struct {
	opts       PrinterOpts
	checkCount int
	checkPass  int
}

func (p *Printer) OnPresubmitStart(mr monorepo.Monorepo, presubmitId string, checks []Check) {
	p.opts.Logs(fmt.Sprintf("Running %d checks in %s\n", len(checks), mr.Root))
}

func (p *Printer) OnCheckStart(check Check) {
	// Result will be printed on the same line in OnCheckResult, unless verbose
	p.opts.Logs(check.Name() + " ")
}

type stderrWriter struct {
	log LogFunc
}

func (sw stderrWriter) Write(p []byte) (n int, err error) {
	sw.log(string(p))
	return len(p), nil
}

func (p *Printer) OnCheckResult(mdPath monorepo.Path, check Check, result *presubmitpb.CheckResult) {
	success := result.OverallResult.Success
	status := "PASSED"
	if !success {
		status = "FAILED"
	}
	// The name was already printed without a newline in OnCheckStart.
	p.opts.Logs(fmt.Sprintf("%s\n", status))
	if !success {
		p.opts.Logs(fmt.Sprintf("  %s\n", mdPath))
	}
	if !success {
		build.PrintFailureResult(stderrWriter{p.opts.Logs}, result.OverallResult, result.SubResults)
	} else if p.opts.Verbose {
		p.opts.Logs(fmt.Sprintf("%v", result.OverallResult.Logs))
	}
	p.checkCount++
	if success {
		p.checkPass++
	}
}

// PresubmitEnd prints a final status message and returns overall success.
func (p *Printer) OnPresubmitEnd(success bool) {
	status := "PASSED"
	if !success {
		status = "FAILED"
	}
	p.opts.Logs(fmt.Sprintf("Presubmit %s. %d checks ran, %d failed.\n", status, p.checkCount, p.checkCount-p.checkPass))
}

func checkLogLabels(id, presubmitId string) map[string]string {
	return map[string]string{
		"presubmit-id":       presubmitId,
		"presubmit-check-id": id,
	}
}

func newUuid() string {
	uuid, err := gouuid.NewV4()
	if err != nil {
		panic("could not construct UUID: " + err.Error())
	}
	return uuid.String()
}
