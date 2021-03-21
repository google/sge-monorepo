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

package build

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	"sge-monorepo/build/cicd/bep"
	"sge-monorepo/build/cicd/monorepo"
	"sge-monorepo/build/cicd/sgeb/protos/buildpb"
	"sge-monorepo/build/cicd/sgeb/protos/sgebpb"
	"sge-monorepo/environment/envinstall"
	"sge-monorepo/libs/go/files"
	"sge-monorepo/libs/go/log"
	"sge-monorepo/libs/go/log/cloudlog"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/timestamp"
)

// Context contains build operation related data.
type Context interface {
	// Build builds the build unit pointed to by the label.
	// opts override construction time options.
	Build(buLabel monorepo.Label, opts ...Option) (*buildpb.BuildResult, error)

	// Test tests the test unit pointed to by the label.
	// Calling this function with a test suite fails. Call ExpandTestSuite first.
	// opts override construction time options.
	Test(tuLabel monorepo.Label, opts ...Option) (*buildpb.TestResult, error)

	// Publish publishes the publish unit pointed to by the label.
	// opts override construction time options.
	Publish(puLabel monorepo.Label, args []string, opts ...PublishOption) ([]*buildpb.PublishResult, error)

	// BazelArgs returns the arguments of the given unit, if any. Used for sorting by sgep.
	BazelArgs(label monorepo.Label) ([]string, error)

	// ExpandTargetExpression expands a target pattern and any test suites to a flat list of test units.
	// If the label points to a test unit, a slice with only that test unit is returned.
	ExpandTargetExpression(te monorepo.TargetExpression) ([]monorepo.Label, error)

	// ResolveBin checks to see if the supplied string is a build unit reference or a local checked-in binary.
	// If the path contains ':' it is a build unit.
	// If the path is a directory it is assumed to be a build unit with the ':foo' bit omitted.
	// If it is a build unit, the unit is built and the absolute path to the result is returned, and the build result is returned.
	// Otherwise the absolute path to the binary is returned, and the build result will be nil (since no build took place).
	ResolveBin(relTo monorepo.Path, bin string, opts ...Option) (string, *buildpb.BuildResult, error)

	// Cleans up any build caches. Call after the build is completed.
	Cleanup() error

	// LoadBuildUnits loads all build units located at the package
	LoadBuildUnits(pkg monorepo.Path) (*sgebpb.BuildUnits, error)

	// RunCron runs a cron unit. Primarily meant for testing your cron units.
	RunCron(label monorepo.Label, args []string, opts ...Option) error

	// RunTask runs a task unit.
	RunTask(label monorepo.Label, args []string, opts ...Option) error
}

// failed signifies a build/test that executed to the end but had failures.
// When this error is returned, one of the return values from the function
// will be a BuildResult/TestResult with structured information about the failure.
// You are not meant to display this error string to the user.
type failed struct {
	Label monorepo.Label
}

func (f failed) Error() string {
	return fmt.Sprintf("%s failed", f.Label.String())
}

// IsFailed returns whether the error is a "build failed" error
func IsFailed(err error) bool {
	_, ok := err.(*failed)
	return ok
}

func maybeFailError(success bool, label monorepo.Label) error {
	if success {
		return nil
	}
	return &failed{label}
}

type context struct {
	Monorepo     monorepo.Monorepo
	buCache      buCache
	buildCache   map[monorepo.Label]*buildpb.BuildResult
	toolCache    map[monorepo.Label]string
	toolCacheDir string
	options      Options
}

// NewContext returns a new builder in the given pwd.
func NewContext(mr monorepo.Monorepo, opts ...Option) (Context, error) {
	options := Options{
		Logs:     os.Stderr,
		LogLevel: "ERROR",
	}
	for _, opt := range opts {
		opt(&options)
	}
	if options.OutputDir == "" {
		options.OutputDir = mr.ResolvePath("sgeb-out")
	}
	if options.LogsDir == "" {
		options.LogsDir = mr.ResolvePath("sgeb-logs")
	}
	toolCacheDir, err := ioutil.TempDir("", "sgeb")
	if err != nil {
		return nil, err
	}
	return &context{
		Monorepo:     mr,
		buCache:      buCache{},
		buildCache:   map[monorepo.Label]*buildpb.BuildResult{},
		toolCache:    map[monorepo.Label]string{},
		toolCacheDir: toolCacheDir,
		options:      options,
	}, nil
}

// Option is a function that modifies the Options structure on context init.
type Option func(*Options)

// Options contains Context options.
type Options struct {
	// Logs is a writer that any build operation stderr output will be directed to. Defaults to stderr.
	Logs io.Writer

	// BazelStartupArgs is a set of options to be given to Bazel. This are options that are set *before*
	// the command. Eg: bazel {BazelStartupArgs} test ...
	BazelStartupArgs []string

	// BazelBuildArgs is a set of options to be given into bazel on a build command. Note that this are
	// options that are set *after* the command. For bazel startup options, see |StartupArgs|.
	// Eg: bazel build {BazelBuildArgs} //some/target
	BazelBuildArgs []string

	// OutputDir is an absolute output directory for non-Bazel build units.
	// If left blank "<monorepo>//sgeb-out" is used.
	OutputDir string

	// LogsDir is an absolute log directory for non-Bazel units.
	// If left blank "<monorepo>//sgeb-logs" is used.
	LogsDir string

	// LogLevel is a glog severity level. Default is ERROR.
	LogLevel string

	// Additional log labels to add to any build invocation.
	LogLabels map[string]string
}

// PublishOption is a function that modifies either Options or the PublishOptions structure.
type PublishOption func(*Options, *PublishOptions)

// PublishOptions contains options for the publish command.
type PublishOptions struct {
	// BaseCL is the base CL number we are building from.
	BaseCl int64

	// CiResultUrl is a URL pointing to the CI run result URL.
	CiResultUrl string
}

func (c *context) Build(buLabel monorepo.Label, opts ...Option) (*buildpb.BuildResult, error) {
	options := c.cmdOpts(opts...)
	return c.buildWithCache(buLabel, options)
}

func (c *context) buildWithCache(buLabel monorepo.Label, options Options) (*buildpb.BuildResult, error) {
	if buildResult, ok := c.buildCache[buLabel]; ok {
		return buildResult, maybeFailError(buildResult.OverallResult.Success, buLabel)
	}
	buildResult, err := c.build(buLabel, options)
	c.buildCache[buLabel] = buildResult
	return buildResult, err
}

func (c *context) build(buLabel monorepo.Label, options Options) (*buildpb.BuildResult, error) {
	pkgDir, err := c.Monorepo.ResolveLabelPkgDir(buLabel)
	if err != nil {
		return nil, err
	}
	bus, err := c.LoadBuildUnits(pkgDir)
	if err != nil {
		return nil, err
	}
	bu, ok := c.findBuildUnit(bus, buLabel)
	if !ok {
		return nil, fmt.Errorf("cannot find build unit %q in pkg //%s", buLabel.Target, buLabel.Pkg)
	}
	if bu.Target != "" {
		// Bazel build unit.
		target, err := c.Monorepo.NewLabel(pkgDir, bu.Target)
		if err != nil {
			return nil, err
		}
		targets := []monorepo.TargetExpression{target.TargetExpression()}
		var logs bytes.Buffer
		bepStream, err := c.runBazelCmd("build", targets, bu.Args, &logs, options)
		success := err == nil
		var result *buildpb.BuildInvocationResult
		if bepStream != nil {
			result, _ = buildInvocationResult(bepStream, target.String())
		} else {
			// Cannot get BEP results, meaning the build completely failed. Synthesize a failed build result
			// with the complete logs.
			result = &buildpb.BuildInvocationResult{
				Result: &buildpb.Result{
					Name:    buLabel.String(),
					Success: false,
					Logs:    []*buildpb.Artifact{{Tag: "stderr", Contents: logs.Bytes()}},
				},
				ArtifactSet: &buildpb.ArtifactSet{},
			}
		}
		if IsFailed(err) {
			// Add label to the fail message.
			err = &failed{buLabel}
		}
		return &buildpb.BuildResult{
			OverallResult: &buildpb.Result{
				Name:    buLabel.String(),
				Success: success,
				Logs:    maybeErrorLogs(success, &logs),
			},
			BuildResult: result,
		}, maybeFailError(success, buLabel)
	} else {
		bin, binBuildResult, err := c.resolveBin(pkgDir, bu.Bin, options)
		if err != nil && binBuildResult != nil {
			return inheritBuildFailure(buLabel, binBuildResult)
		} else if err != nil {
			return nil, err
		}
		inputs, depBuildResult, err := c.buildDeps(pkgDir, bu.Deps, options)
		if err != nil && depBuildResult != nil {
			return inheritBuildFailure(buLabel, depBuildResult)
		} else if err != nil {
			return nil, err
		}
		outputStablePath, err := c.outputStablePath("out", buLabel)
		if err != nil {
			return nil, err
		}
		outputDir, err := c.makeDir(options.OutputDir, "out", buLabel)
		if err != nil {
			return nil, err
		}
		logsDir, err := c.makeDir(options.LogsDir, "logs", buLabel)
		if err != nil {
			return nil, err
		}
		ih, err := newInvocationHelper(&buildpb.ToolInvocation{
			BuildUnitDir: string(pkgDir),
			Inputs:       inputs,
			LogsDir:      logsDir,
			LogLabels:    logLabelsFromOptions(&options),
			BuildInvocation: &buildpb.BuildInvocation{
				OutputDir:        outputDir,
				OutputStablePath: outputStablePath,
				OutputBase:       options.OutputDir,
			},
		})
		if err != nil {
			return nil, err
		}
		defer ih.Cleanup()
		args := []string{ih.InvocationArg(), ih.InvocationResultArg()}
		args = append(args, bu.Args...)
		args = AddGlogFlags(buLabel.Target, options.LogLevel, args)
		var logs bytes.Buffer
		cmd := exec.Command(bin, args...)
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		cmd.Dir = c.Monorepo.Root
		writer := io.MultiWriter(&logs, options.Logs)
		cmd.Stdout = writer
		cmd.Stderr = writer
		buildErr := cmd.Run()
		// For a failed build/test (non-zero exit code), improve the error message printed.
		if _, ok := err.(*exec.ExitError); ok {
			err = fmt.Errorf("%s failed", path.Base(bin))
		}
		buildResult, bepErr := ih.ReadBuildResult()
		if buildErr != nil && buildResult != nil {
			return &buildpb.BuildResult{
				OverallResult: &buildpb.Result{
					Name:    buLabel.String(),
					Success: false,
					Logs:    LogsFromString("logs", logs.String()),
				},
				BuildResult: buildResult,
			}, &failed{buLabel}
		} else if buildErr != nil {
			return nil, fmt.Errorf("%v\n%s", buildErr, logs.String())
		} else if bepErr != nil {
			return nil, bepErr
		}
		return &buildpb.BuildResult{
			OverallResult: &buildpb.Result{
				Name:    buLabel.String(),
				Success: true,
			},
			BuildResult: buildResult,
		}, nil
	}
}

func (c *context) ExpandTargetExpression(te monorepo.TargetExpression) ([]monorepo.Label, error) {
	if strings.HasSuffix(string(te), "/...") {
		l, err := c.Monorepo.NewLabel("", string(te)[:len(te)-4])
		if err != nil {
			return nil, err
		}
		pkg, err := c.Monorepo.ResolveLabelPkgDir(l)
		if err != nil {
			return nil, err
		}
		return c.findAllTests(pkg, map[monorepo.Label]bool{})
	} else {
		l, err := c.Monorepo.NewLabel("", string(te))
		if err != nil {
			return nil, err
		}
		return c.expandTestSuite(l, map[monorepo.Label]bool{})
	}
}

func (c *context) expandTestSuite(l monorepo.Label, seen map[monorepo.Label]bool) ([]monorepo.Label, error) {
	if _, ok := seen[l]; ok {
		return nil, nil
	}
	seen[l] = true

	pkgDir, err := c.Monorepo.ResolveLabelPkgDir(l)
	if err != nil {
		return nil, err
	}
	bus, err := c.LoadBuildUnits(pkgDir)
	if err != nil {
		return nil, err
	}
	// Is this just a test unit or build test unit?
	for _, tu := range bus.TestUnit {
		if tu.Name == l.Target {
			return []monorepo.Label{l}, nil
		}
	}
	for _, btu := range bus.BuildTestUnit {
		if btu.Name == l.Target {
			return []monorepo.Label{l}, nil
		}
	}
	var testSuite *sgebpb.TestSuite
	for _, ts := range bus.TestSuite {
		if ts.Name == l.Target {
			testSuite = ts
			break
		}
	}
	if testSuite == nil {
		return nil, fmt.Errorf("cannot find test unit %q in pkg //%s", l.Target, l.Pkg)
	}
	var ret []monorepo.Label
	for _, tu := range testSuite.TestUnit {
		if tu == "..." {
			expandedTestUnits, err := c.findAllTests(pkgDir, seen)
			if err != nil {
				return nil, err
			}
			ret = append(ret, expandedTestUnits...)
			continue
		}
		tuLabel, err := c.Monorepo.NewLabel(pkgDir, tu)
		if err != nil {
			return nil, err
		}
		// The referenced test unit might itself be a test suite.
		expandedTestUnits, err := c.expandTestSuite(tuLabel, seen)
		if err != nil {
			return nil, err
		}
		ret = append(ret, expandedTestUnits...)
	}
	return ret, nil
}

// findAllTests expands a "..." pattern to recursively find all test units.
func (c *context) findAllTests(dir monorepo.Path, seen map[monorepo.Label]bool) ([]monorepo.Label, error) {
	var ret []monorepo.Label
	dirPath := c.Monorepo.ResolvePath(dir)
	err := filepath.Walk(dirPath, func(p string, info os.FileInfo, err error) error {
		if filepath.Base(p) != "BUILDUNIT" || info.IsDir() {
			return nil
		}
		buDir := filepath.Dir(p)
		buDir = strings.ReplaceAll(buDir, "\\", "/") // bu cache doesn't check '/' vs '\'
		bus, err := c.buCache.loadBuildUnits(buDir)
		if err != nil {
			return err
		}
		pkgDir, err := c.Monorepo.RelPath(buDir)
		if err != nil {
			return err
		}
		for _, tu := range bus.TestUnit {
			tuLabel, err := c.Monorepo.NewLabel(pkgDir, ":"+tu.Name)
			if err != nil {
				return err
			}
			if _, ok := seen[tuLabel]; ok {
				continue
			}
			seen[tuLabel] = true
			ret = append(ret, tuLabel)
		}
		for _, ts := range bus.TestSuite {
			tsLabel, err := c.Monorepo.NewLabel(pkgDir, ":"+ts.Name)
			if err != nil {
				return err
			}
			labels, err := c.expandTestSuite(tsLabel, seen)
			if err != nil {
				return err
			}
			ret = append(ret, labels...)
		}
		for _, btu := range bus.BuildTestUnit {
			label, err := c.Monorepo.NewLabel(pkgDir, ":"+btu.Name)
			if err != nil {
				return err
			}
			if _, ok := seen[label]; ok {
				continue
			}
			seen[label] = true
			ret = append(ret, label)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}
func (c *context) Test(tuLabel monorepo.Label, opts ...Option) (*buildpb.TestResult, error) {
	options := c.cmdOpts(opts...)
	pkgDir, err := c.Monorepo.ResolveLabelPkgDir(tuLabel)
	if err != nil {
		return nil, err
	}
	bus, err := c.LoadBuildUnits(pkgDir)
	if err != nil {
		return nil, err
	}
	if btu, ok := c.findBuildTestUnit(bus, tuLabel); ok {
		return c.testBuildTestUnit(tuLabel, btu, options)
	}
	tu, ok := c.findTestUnit(bus, tuLabel)
	if !ok {
		return nil, fmt.Errorf("cannot find test unit %q in pkg //%s", tuLabel.Target, tuLabel.Pkg)
	}
	if len(tu.Target) > 0 {
		// Bazel test unit.
		var targets []monorepo.TargetExpression
		for _, t := range tu.Target {
			te, err := c.Monorepo.NewTargetExpression(pkgDir, t)
			if err != nil {
				return nil, err
			}
			targets = append(targets, te)
		}
		var logs bytes.Buffer
		args := []string{
			"--build_tests_only",
			"--keep_going",
		}
		args = append(args, tu.Args...)
		bepStream, err := c.runBazelCmd("test", targets, args, &logs, options)
		success := err == nil
		if err != nil && !IsFailed(err) {
			return nil, err
		}
		result, err := testInvocationResult(bepStream)
		if err != nil {
			return nil, err
		}
		return &buildpb.TestResult{
			OverallResult: &buildpb.Result{
				Name:    tuLabel.String(),
				Success: success,
				Logs:    maybeErrorLogs(success, &logs),
			},
			TestResult: result,
		}, maybeFailError(success, tuLabel)
	}
	bin, binBuildResult, err := c.resolveBin(pkgDir, tu.Bin, options)
	if err != nil {
		if binBuildResult != nil {
			return inheritBuildFailureAsTestResult(tuLabel, binBuildResult)
		}
		return nil, err
	}
	inputs, depBuildResult, err := c.buildDeps(pkgDir, tu.Deps, options)
	if err != nil && depBuildResult != nil {
		return inheritBuildFailureAsTestResult(tuLabel, depBuildResult)
	} else if err != nil {
		return nil, err
	}
	ih, err := newInvocationHelper(&buildpb.ToolInvocation{
		BuildUnitDir:   string(pkgDir),
		Inputs:         inputs,
		TestInvocation: &buildpb.TestInvocation{},
		LogLabels:      logLabelsFromOptions(&options),
	})
	if err != nil {
		return nil, err
	}
	defer ih.Cleanup()
	args := []string{ih.InvocationArg(), ih.InvocationResultArg()}
	args = append(args, tu.Args...)
	args = AddGlogFlags(tuLabel.Target, options.LogLevel, args)
	cmd := exec.Command(bin, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Dir = c.Monorepo.Root
	logs := &bytes.Buffer{}
	writer := io.MultiWriter(logs, options.Logs)
	cmd.Stdout = writer
	cmd.Stderr = writer
	testErr := cmd.Run()
	// For a failed build/test (non-zero exit code), improve the error message printed.
	if _, ok := testErr.(*exec.ExitError); ok {
		testErr = fmt.Errorf("%s failed", path.Base(bin))
	}
	testResult, bepErr := ih.ReadTestResult()
	if testErr != nil && testResult != nil {
		return &buildpb.TestResult{
			OverallResult: &buildpb.Result{
				Name:    tuLabel.String(),
				Success: false,
				Logs:    LogsFromString("logs", logs.String()),
			},
			TestResult: testResult,
		}, &failed{tuLabel}
	} else if testErr != nil {
		return nil, fmt.Errorf("%v\n%s", testErr, logs.String())
	} else if bepErr != nil {
		return nil, bepErr
	}
	return &buildpb.TestResult{
		OverallResult: &buildpb.Result{
			Success: true,
		},
		TestResult: testResult,
	}, nil
}

func (c *context) testBuildTestUnit(label monorepo.Label, btu *sgebpb.BuildTestUnit, options Options) (*buildpb.TestResult, error) {
	relTo, err := c.Monorepo.ResolveLabelPkgDir(label)
	if err != nil {
		return nil, err
	}
	buLabel, err := c.Monorepo.NewLabel(relTo, btu.BuildUnit)
	if err != nil {
		return nil, err
	}
	buildRes, err := c.buildWithCache(buLabel, options)
	if err != nil {
		fmt.Println(err)
	}
	var testRes *buildpb.TestResult
	if buildRes != nil {
		testRes = &buildpb.TestResult{
			OverallResult: buildRes.OverallResult,
			TestResult: &buildpb.TestInvocationResult{
				Results: []*buildpb.Result{buildRes.BuildResult.Result},
			},
		}
	}
	return testRes, err
}

func (c *context) Publish(puLabel monorepo.Label, args []string, opts ...PublishOption) ([]*buildpb.PublishResult, error) {
	invocationTime := time.Now()
	return c.publish(puLabel, invocationTime, args, opts...)
}

func (c *context) publish(puLabel monorepo.Label, invocationTime time.Time, args []string, opts ...PublishOption) ([]*buildpb.PublishResult, error) {
	pkgDir, err := c.Monorepo.ResolveLabelPkgDir(puLabel)
	if err != nil {
		return nil, err
	}
	bus, err := c.LoadBuildUnits(pkgDir)
	if err != nil {
		return nil, err
	}
	pu, ok := c.findPublishUnit(bus, puLabel)
	if !ok {
		return nil, fmt.Errorf("cannot find publish unit %q in pkg //%s", puLabel.Target, puLabel.Pkg)
	}
	// Regular publish unit or one with dependencies?
	if pu.Bin != "" {
		return c.publishSingle(pu, puLabel, pkgDir, invocationTime, args, opts...)
	} else if len(pu.PublishUnit) > 0 {
		return c.publishDeps(pu, pkgDir, invocationTime, args, opts...)
	} else {
		// Can't happen due to earlier validation.
		return nil, fmt.Errorf("invalid publish_unit: neither bin nor publish_units")
	}
}

func (c *context) publishSingle(pu *sgebpb.PublishUnit, puLabel monorepo.Label, pkgDir monorepo.Path, invocationTime time.Time, args []string, opts ...PublishOption) ([]*buildpb.PublishResult, error) {
	options := c.options
	publishOptions := PublishOptions{}
	for _, opt := range opts {
		opt(&options, &publishOptions)
	}
	bin, binResult, err := c.resolveBin(pkgDir, pu.Bin, options)
	if err != nil {
		if binResult != nil {
			PrintFailedBuildResult(options.Logs, binResult)
		}
		return nil, err
	}
	var artifactSet []*buildpb.ArtifactSet
	for _, bu := range pu.BuildUnit {
		buLabel, err := c.Monorepo.NewLabel(pkgDir, bu)
		if err != nil {
			return nil, err
		}
		buildResult, err := c.Build(buLabel)
		if err != nil {
			if buildResult != nil {
				PrintFailedBuildResult(options.Logs, buildResult)
			}
			return nil, err
		}
		artifactSet = append(artifactSet, buildResult.BuildResult.ArtifactSet)
	}
	logsDir, err := c.makeDir(options.LogsDir, "logs", puLabel)
	if err != nil {
		return nil, err
	}
	ih, err := newInvocationHelper(&buildpb.ToolInvocation{
		BuildUnitDir: string(pkgDir),
		Inputs:       artifactSet,
		LogsDir:      logsDir,
		LogLabels:    logLabelsFromOptions(&options),
		PublishInvocation: &buildpb.PublishInvocation{
			BaseCl:      publishOptions.BaseCl,
			CiResultUrl: publishOptions.CiResultUrl,
			InvocationTime: &timestamp.Timestamp{
				Seconds: invocationTime.Unix(),
			},
		},
	})
	if err != nil {
		return nil, err
	}
	defer ih.Cleanup()
	cmdArgs := []string{ih.InvocationArg(), ih.InvocationResultArg()}
	cmdArgs = append(cmdArgs, pu.Args...)
	cmdArgs = append(cmdArgs, args...)
	cmdArgs = AddGlogFlags(puLabel.Target, options.LogLevel, cmdArgs)
	cmd := exec.Command(bin, cmdArgs...)
	cmd.Dir = c.Monorepo.Root
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdout = options.Logs
	cmd.Stderr = options.Logs
	err = cmd.Run()
	if err != nil {
		return nil, err
	}
	result, err := ih.ReadPublishResult()
	if err != nil {
		return nil, err
	}
	return result.PublishResults, nil
}

func (c *context) publishDeps(pu *sgebpb.PublishUnit, pkgDir monorepo.Path, invocationTime time.Time, args []string, opts ...PublishOption) ([]*buildpb.PublishResult, error) {
	var results []*buildpb.PublishResult
	for _, dpu := range pu.PublishUnit {
		dpuLabel, err := c.Monorepo.NewLabel(pkgDir, dpu)
		if err != nil {
			return nil, err
		}
		publishResults, err := c.publish(dpuLabel, invocationTime, args, opts...)
		if err != nil {
			return nil, err
		}
		results = append(results, publishResults...)
	}
	return results, nil
}

func (c *context) BazelArgs(label monorepo.Label) ([]string, error) {
	args, err := c.bazelArgs(label)
	if err != nil {
		return nil, err
	}
	// Do not allow caller to mutate the arg array.
	return append([]string(nil), args...), nil
}

func (c *context) bazelArgs(label monorepo.Label) ([]string, error) {
	pkgDir, err := c.Monorepo.ResolveLabelPkgDir(label)
	if err != nil {
		return nil, err
	}
	u, err := c.LoadBuildUnits(pkgDir)
	if err != nil {
		return nil, err
	}
	for _, u := range u.BuildUnit {
		if u.Name == label.Target {
			if u.Target != "" {
				return u.Args, nil
			}
			return nil, nil
		}
	}
	for _, u := range u.TestUnit {
		if u.Name == label.Target {
			if len(u.Target) > 0 {
				return u.Args, nil
			}
			bu, err := c.Monorepo.NewLabel(pkgDir, u.Bin)
			if err != nil {
				return nil, err
			}
			return c.BazelArgs(bu)
		}
	}
	for _, u := range u.BuildTestUnit {
		if u.Name == label.Target {
			bu, err := c.Monorepo.NewLabel(pkgDir, u.BuildUnit)
			if err != nil {
				return nil, err
			}
			return c.BazelArgs(bu)
		}
	}
	for _, u := range u.PublishUnit {
		if u.Name == label.Target {
			return nil, nil
		}
	}
	return nil, fmt.Errorf("cannot find unit %s", label)
}

func (c *context) cmdOpts(opts ...Option) Options {
	options := c.options
	for _, opt := range opts {
		opt(&options)
	}
	return options
}

type invocationHelper struct {
	dir                  string
	invocationPath       string
	invocationResultPath string
}

func newInvocationHelper(invocation *buildpb.ToolInvocation) (*invocationHelper, error) {
	data, err := proto.Marshal(invocation)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal invocation proto: %v", err)
	}
	dir, err := ioutil.TempDir("", "sgeb")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %v", err)
	}
	invocationPath := path.Join(dir, "invocation.pb")
	if err := ioutil.WriteFile(invocationPath, data, 0666); err != nil {
		os.RemoveAll(dir)
		return nil, fmt.Errorf("failed to write invocation proto: %v", err)
	}
	return &invocationHelper{
		dir:                  dir,
		invocationPath:       invocationPath,
		invocationResultPath: path.Join(dir, "invocation-result.pb"),
	}, nil
}

func (ih *invocationHelper) ReadBuildResult() (*buildpb.BuildInvocationResult, error) {
	buf, err := ioutil.ReadFile(ih.invocationResultPath)
	if err != nil {
		return nil, err
	}
	result := &buildpb.BuildInvocationResult{}
	if err := proto.Unmarshal(buf, result); err != nil {
		return nil, err
	}
	return result, err
}

func (ih *invocationHelper) ReadTestResult() (*buildpb.TestInvocationResult, error) {
	buf, err := ioutil.ReadFile(ih.invocationResultPath)
	if err != nil {
		return nil, err
	}
	result := &buildpb.TestInvocationResult{}
	if err := proto.Unmarshal(buf, result); err != nil {
		return nil, err
	}
	return result, err
}

func (ih *invocationHelper) ReadPublishResult() (*buildpb.PublishInvocationResult, error) {
	buf, err := ioutil.ReadFile(ih.invocationResultPath)
	if err != nil {
		return nil, err
	}
	result := &buildpb.PublishInvocationResult{}
	if err := proto.Unmarshal(buf, result); err != nil {
		return nil, err
	}
	return result, err
}

func (ih *invocationHelper) InvocationArg() string {
	return fmt.Sprintf("--tool-invocation=%s", ih.invocationPath)
}

func (ih *invocationHelper) InvocationResultArg() string {
	return fmt.Sprintf("--tool-invocation-result=%s", ih.invocationResultPath)
}

func (ih *invocationHelper) Cleanup() {
	os.RemoveAll(ih.dir)
}

func (c *context) buildToolBinaryWithCache(binTarget monorepo.Label, options Options) (string, *buildpb.BuildResult, error) {
	if p, ok := c.toolCache[binTarget]; ok {
		return p, nil, nil
	}
	p, br, err := c.buildToolBinary(binTarget, options)
	if err != nil {
		return "", br, err
	}
	// Create a unique directory to place the binary into.
	// 8 chars should be enough to avoid collision.
	hasher := sha1.New()
	hasher.Write([]byte(p))
	sum := hasher.Sum(nil)
	dirName := hex.EncodeToString(sum)[8:]
	dir := filepath.Join(c.toolCacheDir, dirName)
	if err := os.Mkdir(dir, 0666); err != nil {
		return "", nil, fmt.Errorf("could not create tool dir %s: %v", dir, err)
	}
	bin := filepath.Join(dir, path.Base(p))
	if err := copyBin(p, bin); err != nil {
		return "", nil, fmt.Errorf("could not copy bin from %s to %s: %v", p, bin, err)
	}
	c.toolCache[binTarget] = bin
	return bin, nil, nil
}

// buildToolBinary builds a tool binary and returns its path.
// Upon success, only the path is returned.
// Upon failure, an error is returned, and where possible a build result is returned.
func (c *context) buildToolBinary(binTarget monorepo.Label, options Options) (string, *buildpb.BuildResult, error) {
	br, err := c.buildWithCache(binTarget, options)
	if err != nil {
		// Return build result in case this is a build.Error and we have structured failure information.
		// If is isn't a build.Error, br will be nil.
		return "", br, err
	}
	artifactSet := br.BuildResult.ArtifactSet
	if br == nil || artifactSet == nil || len(artifactSet.Artifacts) == 0 {
		return "", nil, fmt.Errorf("building tool %s did not return any outputs", binTarget)
	}
	// Find all executable outputs
	var execs []string
	for _, a := range artifactSet.Artifacts {
		if !strings.HasPrefix(a.Uri, "file:///") {
			continue
		}
		p := a.Uri[len("file:///"):]
		if ok, err := files.IsExecutable(p); err != nil {
			return "", nil, fmt.Errorf("failed to get executable status of %s: %v", p, err)
		} else if !ok {
			continue
		}
		execs = append(execs, p)
	}
	if len(execs) != 1 || execs == nil /* nil check redundant but silences IDE warning */ {
		return "", nil, fmt.Errorf("build tool %s returned %d executable outputs. Exactly 1 executable output must be produced", binTarget, len(execs))
	}
	return execs[0], nil, nil
}

// runBazelCmd executes a bazel command and parses the BEP stream for a build result.
func (c *context) runBazelCmd(cmdName string, targets []monorepo.TargetExpression, args []string, logs io.Writer, options Options) (*bep.Stream, error) {
	bazelwsp, err := c.Monorepo.NewPath("", "//bin/windows/bazel.exe")
	if err != nil {
		return nil, err
	}
	bazel := c.Monorepo.ResolvePath(bazelwsp)
	var cmdArgs []string
	cmdArgs = append(cmdArgs, options.BazelStartupArgs...)
	cmdArgs = append(cmdArgs, cmdName)
	cmdArgs = append(cmdArgs, options.BazelBuildArgs...)
	cmdArgs = append(cmdArgs, args...)
	bepDir, err := ioutil.TempDir("", "bep")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(bepDir)
	bepFile := path.Join(bepDir, "bep")
	cmdArgs = append(cmdArgs, "--build_event_binary_file", bepFile)
	for _, t := range targets {
		cmdArgs = append(cmdArgs, string(t))
	}
	cmd := exec.Command(bazel, cmdArgs...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Dir = c.Monorepo.Root

	// Set up a non-global logger that respects the log options.
	// glog doesn't have per-instance options, so we have to temporarily modify flags
	glogFlagName := "stderrthreshold"
	f := flag.Lookup(glogFlagName)
	if f == nil {
		return nil, fmt.Errorf("could not look up glog flag %q", glogFlagName)
	}
	oldVal := f.Value.String()
	if err := flag.Set(glogFlagName, options.LogLevel); err != nil {
		return nil, fmt.Errorf("could not set glog flag %q: %v", glogFlagName, err)
	}
	defer flag.Set(glogFlagName, oldVal)
	logger := log.New()
	defer logger.Shutdown()
	logger.AddSink(log.NewGlog())
	var cl cloudlog.CloudLogger
	if envinstall.IsCloud() {
		var err error
		cl, err = cloudlog.New("sgeb", cloudlog.WithLabels(options.LogLabels))
		if err != nil {
			return nil, fmt.Errorf("could not obtain a cloud logger: %v", err)
		}
		logger.AddSink(cl)
	}
	cmd.Stderr = io.MultiWriter(logs, log.NewInfoLogger(logger))

	buildErr := cmd.Run()
	if exitErr, ok := buildErr.(*exec.ExitError); ok {
		switch exitErr.ExitCode() {
		case 1, 3, 4:
			// build/test failed exit code. See:
			// https://docs.bazel.build/versions/master/guide.html#what-exit-code-will-i-get
			buildErr = &failed{}
		default:
			return nil, buildErr
		}
	}
	bepStream, err := readBepStream(bepFile)
	if err != nil && buildErr == nil {
		return nil, err
	}
	return bepStream, buildErr
}

func readBepStream(p string) (*bep.Stream, error) {
	bepBuf, err := ioutil.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("could not read BEP stream from %s: %v", p, err)
	}
	return bep.Parse(bepBuf)
}

func (c *context) ResolveBin(relTo monorepo.Path, bin string, opts ...Option) (string, *buildpb.BuildResult, error) {
	options := c.cmdOpts(opts...)
	return c.resolveBin(relTo, bin, options)
}

func (c *context) resolveBin(relTo monorepo.Path, bin string, options Options) (string, *buildpb.BuildResult, error) {
	isBuildUnit := strings.Contains(bin, ":")
	var binAbsPath string
	var buildResult *buildpb.BuildResult
	if !isBuildUnit {
		binPath, err := c.Monorepo.NewPath(relTo, bin)
		if err != nil {
			return "", nil, err
		}
		binAbsPath = c.Monorepo.ResolvePath(binPath)
		// If it's a directory, then we still think it's a build unit
		if stat, err := os.Stat(binAbsPath); err == nil {
			isBuildUnit = stat.IsDir()
		}
	}
	if isBuildUnit {
		binTarget, err := c.Monorepo.NewLabel(relTo, bin)
		if err != nil {
			return "", nil, err
		}
		// Build unit whose tool is another build unit.
		binAbsPath, buildResult, err = c.buildToolBinaryWithCache(binTarget, options)
		if err != nil {
			// Return the build result in case this is a build.Error.
			return "", buildResult, fmt.Errorf("%v, cannot proceed", err)
		}
	}
	return binAbsPath, buildResult, nil
}

// buildDeps builds the dependencies of the build unit and returns the build results.
// On failure the build result of the cause is returned, else nil is returned.
func (c *context) buildDeps(relTo monorepo.Path, deps []string, options Options) ([]*buildpb.ArtifactSet, *buildpb.BuildResult, error) {
	var result []*buildpb.ArtifactSet
	for _, dbu := range deps {
		dl, err := c.Monorepo.NewLabel(relTo, dbu)
		if err != nil {
			return nil, nil, err
		}
		br, err := c.buildWithCache(dl, options)
		if err != nil {
			return nil, br, err
		}
		if br == nil {
			return nil, nil, fmt.Errorf("buildDeps for %s returned nil error", dl)
		}
		result = append(result, br.BuildResult.ArtifactSet)
	}
	return result, nil, nil
}

func inheritBuildFailure(buLabel monorepo.Label, buildResult *buildpb.BuildResult) (*buildpb.BuildResult, error) {
	// Inherit the failure from the bin dependency
	return &buildpb.BuildResult{
		OverallResult: &buildpb.Result{
			Name:    buLabel.String(),
			Success: false,
			Cause:   buildResult.OverallResult.Name,
			Logs:    buildResult.OverallResult.Logs,
		},
		BuildResult: buildResult.BuildResult,
	}, &failed{buLabel}
}

func inheritBuildFailureAsTestResult(tuLabel monorepo.Label, buildResult *buildpb.BuildResult) (*buildpb.TestResult, error) {
	return &buildpb.TestResult{
		OverallResult: &buildpb.Result{
			Name:    tuLabel.String(),
			Success: false,
			Cause:   buildResult.OverallResult.Name,
		},
		TestResult: &buildpb.TestInvocationResult{
			Results: []*buildpb.Result{buildResult.BuildResult.Result},
		},
	}, &failed{tuLabel}
}

func (c *context) Cleanup() error {
	return os.RemoveAll(c.toolCacheDir)
}

func (c *context) findBuildUnit(bus *sgebpb.BuildUnits, l monorepo.Label) (*sgebpb.BuildUnit, bool) {
	for _, bu := range bus.BuildUnit {
		if bu.Name == l.Target {
			return bu, true
		}
	}
	return nil, false
}

func (c *context) findTestUnit(bus *sgebpb.BuildUnits, l monorepo.Label) (*sgebpb.TestUnit, bool) {
	for _, tu := range bus.TestUnit {
		if tu.Name == l.Target {
			return tu, true
		}
	}
	return nil, false
}

func (c *context) findBuildTestUnit(bus *sgebpb.BuildUnits, l monorepo.Label) (*sgebpb.BuildTestUnit, bool) {
	for _, btu := range bus.BuildTestUnit {
		if btu.Name == l.Target {
			return btu, true
		}
	}
	return nil, false
}

func (c *context) findPublishUnit(bus *sgebpb.BuildUnits, l monorepo.Label) (*sgebpb.PublishUnit, bool) {
	for _, pu := range bus.PublishUnit {
		if pu.Name == l.Target {
			return pu, true
		}
	}
	return nil, false
}

func (c *context) findCronUnit(bus *sgebpb.BuildUnits, l monorepo.Label) (*sgebpb.CronUnit, bool) {
	for _, cu := range bus.CronUnit {
		if cu.Name == l.Target {
			return cu, true
		}
	}
	return nil, false
}

func (c *context) findTaskUnit(bus *sgebpb.BuildUnits, l monorepo.Label) (*sgebpb.TaskUnit, bool) {
	for _, tu := range bus.TaskUnit {
		if tu.Name == l.Target {
			return tu, true
		}
	}
	return nil, false
}

func (c *context) LoadBuildUnits(p monorepo.Path) (*sgebpb.BuildUnits, error) {
	return c.buCache.loadBuildUnits(c.Monorepo.ResolvePath(p))
}

func (c *context) outputStablePath(name string, label monorepo.Label) (string, error) {
	pkgDir, err := c.Monorepo.ResolveLabelPkgDir(label)
	if err != nil {
		return "", err
	}
	// Ensure target label does not end in ".<name>" (will collide with out directory)
	if strings.HasSuffix(label.Target, fmt.Sprintf(".%s", name)) {
		return "", fmt.Errorf("invalid label %s: must not end with '.%s'", label, name)
	}
	// Construct unique output directory.
	// Example: //foo/bar:baz -> foo/bar/baz.<name>
	dir := path.Join(string(pkgDir), fmt.Sprintf("%s.%s", label.Target, name))
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("failed to clean %s directory %s: %v", name, dir, err)
	}
	return dir, nil
}

func (c *context) makeDir(root, name string, label monorepo.Label) (string, error) {
	outputStablePath, err := c.outputStablePath(name, label)
	if err != nil {
		return "", err
	}
	// Construct unique output directory.
	// Example: //foo/bar:baz -> <root>/foo/bar/baz.<name>
	dir := path.Join(root, outputStablePath)
	if err := os.RemoveAll(dir); err != nil {
		return "", fmt.Errorf("failed to clean %s directory %s: %v", name, dir, err)
	}
	if err := os.MkdirAll(dir, 0664); err != nil {
		return "", fmt.Errorf("failed to make %s directory %s: %v", name, dir, err)
	}
	return dir, nil
}

type buCache map[string]*sgebpb.BuildUnits

func (buc buCache) loadBuildUnits(pkg string) (*sgebpb.BuildUnits, error) {
	if bu, ok := buc[pkg]; ok {
		return bu, nil
	}

	buFile := path.Join(pkg, "BUILDUNIT")
	if !fileExists(buFile) {
		return nil, fmt.Errorf("cannot find %s/BUILDUNIT", pkg)
	}
	content, err := ioutil.ReadFile(buFile)
	if err != nil {
		return nil, fmt.Errorf("error reading file %s: %v", buFile, err)
	}
	bu := &sgebpb.BuildUnits{}
	if err = proto.UnmarshalText(string(content), bu); err != nil {
		return nil, fmt.Errorf("error reading file %s: %v", buFile, err)
	}
	if err := validateBuildUnits(bu); err != nil {
		return nil, fmt.Errorf("error reading file %s: %v", buFile, err)
	}

	buc[pkg] = bu
	return bu, nil
}

type validationUnit struct {
	name       string
	hasTarget  bool
	hasBin     bool
	hasEnvVars bool
	hasDeps    bool
}

func validateBuildUnits(bu *sgebpb.BuildUnits) error {
	var names []string
	var units []validationUnit
	for _, bu := range bu.BuildUnit {
		names = append(names, bu.Name)
		units = append(units, validationUnit{
			name:       bu.Name,
			hasTarget:  bu.Target != "",
			hasBin:     bu.Bin != "",
			hasEnvVars: len(bu.EnvVars) > 0,
			hasDeps:    len(bu.Deps) > 0,
		})
	}
	for _, tu := range bu.TestUnit {
		names = append(names, tu.Name)
		units = append(units, validationUnit{
			name:       tu.Name,
			hasTarget:  len(tu.Target) > 0,
			hasBin:     tu.Bin != "",
			hasEnvVars: len(tu.EnvVars) > 0,
			hasDeps:    len(tu.Deps) > 0,
		})
	}
	for _, ts := range bu.TestSuite {
		names = append(names, ts.Name)
	}
	for _, pu := range bu.PublishUnit {
		names = append(names, pu.Name)
		hasBuildUnits := pu.Bin != "" && len(pu.BuildUnit) > 0
		hasDepUnits := len(pu.PublishUnit) > 0
		hasOnlyOne := (hasBuildUnits || hasDepUnits) && !(hasBuildUnits && hasDepUnits)
		if !hasOnlyOne {
			return fmt.Errorf("publish unit %q must have either a bin and build_units or dependent publish_units, but not both", pu.Name)
		}
		if pu.PostSubmit != nil {
			if pu.PostSubmit.TriggerPaths != nil && pu.PostSubmit.Frequency != nil {
				return fmt.Errorf("publish unit %q must not have both trigger_paths and frequency in its postsubmit", pu.Name)
			}
		}
	}

	// Ensure no name conflicts.
	seen := map[string]bool{}
	for _, name := range names {
		if _, ok := seen[name]; ok {
			return fmt.Errorf("two or more units with same name %q", name)
		}
		seen[name] = true
	}
	// Must have one of target and bin, but not both
	for _, u := range units {
		if u.hasTarget && u.hasBin {
			return fmt.Errorf("build/test unit %q must not have both target and bin", u.name)
		}
		if !(u.hasTarget || u.hasBin) {
			return fmt.Errorf("build/test test unit %q must have either target or bin", u.name)
		}
	}
	// Bazel targets must not have env vars or deps
	for _, u := range units {
		if !u.hasTarget {
			continue
		}
		if u.hasEnvVars {
			return fmt.Errorf("build/test unit %q must not have env vars", u.name)
		}
		if u.hasDeps {
			return fmt.Errorf("build/test unit %q must not have deps", u.name)
		}
	}
	return nil
}

func fileExists(p string) bool {
	stat, err := os.Stat(p)
	if err != nil {
		return false
	}
	return !stat.IsDir()
}

func maybeErrorLogs(success bool, logs *bytes.Buffer) []*buildpb.Artifact {
	if success {
		return nil
	}
	return []*buildpb.Artifact{
		{
			Tag:      "logs",
			Contents: logs.Bytes(),
		},
	}
}

func copyBin(sp, dp string) error {
	src, err := os.Open(sp)
	if err != nil {
		return err
	}
	defer src.Close()
	dest, err := os.OpenFile(dp, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0777)
	if err != nil {
		return err
	}
	defer dest.Close()
	_, err = io.Copy(dest, src)
	if err != nil {
		return err
	}
	return nil
}

// PrintBuildResult prints the overall result for a Build execution.
// If there are <= maxResults artifacts or maxResults == -1 we print the produced artifacts.
func PrintBuildResult(logs io.Writer, l monorepo.Label, result *buildpb.BuildResult, maxResults int) {
	if result.OverallResult.Success {
		artifacts := result.BuildResult.GetArtifactSet().GetArtifacts()
		if len(artifacts) > 0 {
			fmt.Printf("%s successfully built\n", l)
			artifactStr := printArtifacts(artifacts)
			if maxResults == -1 || len(artifacts) <= maxResults {
				for _, line := range strings.Split(artifactStr, "\n") {
					fmt.Printf("  %s\n", line)
				}
			} else {
				f, err := ioutil.TempFile("", "sgeb-outputs-*")
				defer f.Close()
				if err == nil {
					_, err = f.Write([]byte(artifactStr))
				}
				if err == nil {
					fmt.Printf("  %d artifacts produced, too many to display. Full list produced at %s\n", len(artifacts), f.Name())
				} else {
					fmt.Printf("  %d artifacts produced, too many to display. Failed to produce list: %v\n", len(artifacts), err)
				}
			}
		} else {
			fmt.Printf("%s successfully built (no outputs produced)\n", l)
		}
		return
	}
	PrintFailedBuildResult(logs, result)
}

func printArtifacts(artifacts []*buildpb.Artifact) string {
	var result bytes.Buffer
	for _, output := range artifacts {
		uri := output.Uri
		prefix := "file:///"
		if strings.HasPrefix(uri, prefix) {
			uri = uri[len(prefix):]
		}
		// Make it easier to copy-paste result into a Windows command prompt.
		if runtime.GOOS == "windows" {
			uri = strings.ReplaceAll(uri, `/`, `\`)
		}
		var stablePathPrefix string
		if output.StablePath != "" {
			stablePathPrefix = fmt.Sprintf("%s -> ", output.StablePath)
		}
		_, _ = fmt.Fprintf(&result, "%s%s\n", stablePathPrefix, uri)
	}
	return result.String()
}

// PrintFailedBuildResult prints results for a failed build.
func PrintFailedBuildResult(logs io.Writer, result *buildpb.BuildResult) {
	PrintFailureResult(logs, result.OverallResult, []*buildpb.Result{result.BuildResult.Result})
}

// PrintTestResult prints the overall result for a Test execution.
func PrintTestResult(logs io.Writer, l monorepo.Label, result *buildpb.TestResult) {
	if result.OverallResult.Success {
		fmt.Printf("%s PASSED\n", l)
		return
	}
	PrintFailedTestResult(logs, result)
}

// PrintFailedTestResult prints results for a failed test.
func PrintFailedTestResult(logs io.Writer, result *buildpb.TestResult) {
	PrintFailureResult(logs, result.OverallResult, result.TestResult.Results)
}

// PrintFailureResult prints results for a failed build/test.
func PrintFailureResult(logs io.Writer, overallResult *buildpb.Result, subResults []*buildpb.Result) {
	var fixes []string
	for _, subResult := range subResults {
		if !subResult.Success {
			continue
		}
		printIndented(logs, 2, fmt.Sprintf("%s PASSED", subResult.Name))
	}
	for _, subResult := range subResults {
		if subResult.Success {
			continue
		}
		printIndented(logs, 2, fmt.Sprintf("%s FAILED", subResult.Name))
		if subResult.Cause != "" {
			printIndented(logs, 4, fmt.Sprintf("Cause: %s\n", subResult.Cause))
		}
		if subResult.Fix != "" {
			printIndented(logs, 4, fmt.Sprintf("To fix: %s\n", subResult.Fix))
			fixes = append(fixes, subResult.Fix)
		}
		printIndented(logs, 4, resultLogs(subResult))
	}
	// If one or more of the subresults are missing logs and cause, print overall result.
	if len(subResults) == 0 || anyFailureWithMissingLogs(subResults) {
		if overallResult.Cause != "" {
			printIndented(logs, 4, fmt.Sprintf("Cause: %s\n", overallResult.Cause))
		}
		printIndented(logs, 4, resultLogs(overallResult))
	}
	if len(fixes) > 0 {
		fmt.Println("Fixes:")
		for _, fix := range fixes {
			printIndented(logs, 2, fix)
		}
	}
}

// LogsFromString synthesizes a build log from a string.
func LogsFromString(tag, logs string) []*buildpb.Artifact {
	return []*buildpb.Artifact{
		{
			Tag:      tag,
			Contents: []byte(logs),
		},
	}
}

// anyFailureWithMissingLogs returns true if one or more failed results do not have any logs.
func anyFailureWithMissingLogs(results []*buildpb.Result) bool {
	for _, r := range results {
		if !r.Success && len(r.Logs) == 0 && r.Cause == "" {
			return true
		}
	}
	return false
}

func printIndented(logs io.Writer, width int, msg string) {
	var ib strings.Builder
	for i := 0; i < width; i++ {
		ib.WriteString(" ")
	}
	indent := ib.String()
	for _, line := range strings.Split(msg, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		_, _ = fmt.Fprintf(logs, "%s%s\n", indent, line)
	}
}

// resultLogs returns all logs from the given build result.
func resultLogs(result *buildpb.Result) string {
	var sb strings.Builder
	for _, r := range result.Logs {
		if r.Contents != nil {
			sb.Write(r.Contents)
		}
		if r.Uri != "" {
			if strings.HasPrefix(r.Uri, "file:///") {
				p := r.Uri[len("file:///"):]
				str, err := ioutil.ReadFile(p)
				if err != nil {
					return fmt.Sprintf("could not read logs: %v", err)
				}
				sb.Write(str)
			}
		}
	}
	return sb.String()
}

// AddGlogFlags appends appropriate glog flags used by tool binaries.
func AddGlogFlags(name, logLevel string, args []string) []string {
	if ad, err := files.GetAppDir("sge", name); err == nil {
		// set directory for glog to %APPDATA%/sge/<name>
		args = append(args, fmt.Sprintf("-log_dir=%s", ad))
	}
	// glog to both stderr above a certain threshold.
	args = append(args, fmt.Sprintf("-stderrthreshold=%s", logLevel))
	return args
}

func logLabelsFromOptions(opts *Options) []*buildpb.LogLabel {
	var labels []*buildpb.LogLabel
	for k, v := range opts.LogLabels {
		labels = append(labels, &buildpb.LogLabel{
			Key:   k,
			Value: v,
		})
	}
	return labels
}

// UnitFile is a BUILDUNIT file with its directory.
type UnitFile struct {
	Proto *sgebpb.BuildUnits
	Dir   monorepo.Path
}

// DiscoverBuildUnitFiles recursively searches the monorepo for any BUILDUNIT files
func DiscoverBuildUnitFiles(mr monorepo.Monorepo, bc Context) ([]UnitFile, error) {
	var ret []UnitFile
	if err := filepath.Walk(mr.Root, func(p string, info os.FileInfo, err error) error {
		// TODO: Ignore dirs?
		// Experiments show that it this function takes ~8 seconds without ignoring directories,
		// and ~.5 seconds if third_party is excluded.
		if filepath.Base(p) != "BUILDUNIT" {
			return nil
		}
		dir, err := mr.RelPath(filepath.Dir(p))
		if err != nil {
			return err
		}
		bus, err := bc.LoadBuildUnits(dir)
		if err != nil {
			return err
		}
		ret = append(ret, UnitFile{bus, dir})
		return nil
	}); err != nil {
		return nil, err
	}
	return ret, nil
}

func (c *context) RunCron(label monorepo.Label, args []string, opts ...Option) error {
	options := c.cmdOpts(opts...)
	pkgDir, err := c.Monorepo.ResolveLabelPkgDir(label)
	if err != nil {
		return err
	}
	bus, err := c.LoadBuildUnits(pkgDir)
	if err != nil {
		return err
	}
	cu, ok := c.findCronUnit(bus, label)
	if !ok {
		return fmt.Errorf("cannot find cron unit %q in pkg //%s", label.Target, label.Pkg)
	}
	bin, binResult, err := c.resolveBin(pkgDir, cu.Bin, options)
	if err != nil {
		if binResult != nil {
			PrintFailedBuildResult(options.Logs, binResult)
		}
		return err
	}
	ih, err := newInvocationHelper(&buildpb.ToolInvocation{
		BuildUnitDir:   string(pkgDir),
		CronInvocation: &buildpb.CronInvocation{},
	})
	if err != nil {
		return err
	}
	defer ih.Cleanup()
	cmdArgs := []string{ih.InvocationArg()}
	cmdArgs = append(cmdArgs, cu.Args...)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(bin, cmdArgs...)
	cmd.Dir = c.Monorepo.Root
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdout = options.Logs
	cmd.Stderr = options.Logs
	return cmd.Run()
}

func (c *context) RunTask(label monorepo.Label, args []string, opts ...Option) error {
	options := c.cmdOpts(opts...)
	pkgDir, err := c.Monorepo.ResolveLabelPkgDir(label)
	if err != nil {
		return err
	}
	bus, err := c.LoadBuildUnits(pkgDir)
	if err != nil {
		return err
	}
	tu, ok := c.findTaskUnit(bus, label)
	if !ok {
		return fmt.Errorf("cannot find task unit %q in pkg //%s", label.Target, label.Pkg)
	}
	bin, binResult, err := c.resolveBin(pkgDir, tu.Bin, options)
	if err != nil {
		if binResult != nil {
			PrintFailedBuildResult(options.Logs, binResult)
		}
		return err
	}
	ih, err := newInvocationHelper(&buildpb.ToolInvocation{
		BuildUnitDir:   string(pkgDir),
		TaskInvocation: &buildpb.TaskInvocation{},
		LogLabels:      logLabelsFromOptions(&options),
	})
	if err != nil {
		return err
	}
	defer ih.Cleanup()
	cmdArgs := []string{ih.InvocationArg()}
	cmdArgs = append(cmdArgs, tu.Args...)
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(bin, cmdArgs...)
	cmd.Dir = c.Monorepo.Root
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdout = options.Logs
	cmd.Stderr = options.Logs
	return cmd.Run()
}
