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

// Binary jenkins_scrapper is a little tool that knows how to query the job endpoints of Jenkins
// And visualize the information.

package main

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"sge-monorepo/build/cicd/jenkins"
	"sge-monorepo/libs/go/files"

	"sge-monorepo/build/cicd/cirunner/protos/cirunnerpb"
)

var workerCount = 20

var flags = struct {
	out string
}{}

func parseFlags() error {
	flag.StringVar(&flags.out, "out", "", "Directory where the CSV will be written to.")
	flag.Parse()
	if flags.out == "" {
		flag.PrintDefaults()
		return errors.New("out flag cannot be empty")
	}
	if !files.DirExists(flags.out) {
		return fmt.Errorf("%q is not valid directory", flags.out)
	}
	return nil
}

// Job represents the Jenkins jobs, which can holds many runs.
// This is where the pipeline would be installed on.
type Job struct {
	Name        string
	Description string
	FullName    string
	JobBuilds   []struct {
		Number int
		Url    string
	} `json:"allBuilds"`
}

// Run is generic interface representing any job run within Jenkins.
type Run interface {
	Number() int
	AsCsv() []string
}

type ScrapFunc func(*cirunnerpb.JenkinsCredentials, int) (Run, error)

// Jobs --------------------------------------------------------------------------------------------

func scrapJob(creds *cirunnerpb.JenkinsCredentials, jobPath string, scrapFunc ScrapFunc) ([]Run, error) {
	path := fmt.Sprintf("%s/api/json", jobPath)
	body, err := jenkins.SendJenkinsRequest(creds, "GET", path, map[string]string{
		"tree": "name,description,fullName,allBuilds[number]",
	})
	if err != nil {
		return nil, fmt.Errorf("could not get %q: %w", path, err)
	}
	job := &Job{}
	if err := json.Unmarshal([]byte(body), &job); err != nil {
		return nil, fmt.Errorf("could not unmarshal body for GET %q: %w", path, err)
	}
	// Because each run is a roundtrip to the server, we create several workers to create parallel
	// requests and make the process faster.
	c := make(chan int, 10)
	runCh := make(chan Run, 10)
	// Create a couple of workers and wait for them to be done.
	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for number := range c {
				run, err := scrapFunc(creds, number)
				if err != nil {
					fmt.Printf("Error on worker: %v", err)
					break
				}
				runCh <- run
			}
		}()
	}
	// Create another worker that reduces it to a range.
	var runs []Run
	var reducerWg sync.WaitGroup
	reducerWg.Add(1)
	go func() {
		defer reducerWg.Done()
		for run := range runCh {
			runs = append(runs, run)
		}
	}()
	// Insert all the numbers into the range.
	for _, b := range job.JobBuilds {
		c <- b.Number
	}
	close(c)
	// Wait for all workers to be done and then close the channel.
	wg.Wait()
	close(runCh)
	// Wait for the reducer to be done.
	reducerWg.Wait()
	// Sort the result.
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].Number() > runs[j].Number()
	})
	return runs, nil
}

func obtainMachineFromLogs(creds *cirunnerpb.JenkinsCredentials, jobPath string, number int) (string, error) {
	// Get the logs to obtain the running machine.
	logPath := fmt.Sprintf("%s/%d/consoleText", jobPath, number)
	logBody, err := jenkins.SendJenkinsRequest(creds, "GET", logPath, map[string]string{})
	if err != nil {
		return "", fmt.Errorf("could not GET %q: %w", logPath, err)
	}
	matches := machineRegex.FindStringSubmatch(logBody)
	machine := ""
	if len(matches) == 0 {
		fmt.Printf("WARNING: could not find machine for job %s number %d\n", jobPath, number)
	} else {
		machine = matches[1]
	}
	return machine, nil
}

// Presubmit ---------------------------------------------------------------------------------------

var presubmitHeaders = []string{
	"Number", "Result", "Duration", "Timestamp", "Review", "Change", "BaseCl", "Machine",
}

type PresubmitRun struct {
	Id        int
	Result    string
	Duration  time.Duration
	Timestamp time.Time
	Review    int
	Change    int
	BaseCl    int
	Machine   string
}

func (run *PresubmitRun) Number() int {
	return run.Id
}

func (run *PresubmitRun) AsCsv() []string {
	return []string{
		strconv.Itoa(run.Id),
		run.Result,
		run.Duration.String(),
		run.Timestamp.String(),
		strconv.Itoa(run.Review),
		strconv.Itoa(run.Change),
		strconv.Itoa(run.BaseCl),
		run.Machine,
	}
}

func scrapPresubmitRunner(creds *cirunnerpb.JenkinsCredentials) ([]Run, error) {
	runs, err := scrapJob(creds, "job/presubmits/job/presubmit", scrapPresubmitRun)
	if err != nil {
		return nil, fmt.Errorf("could not scrap presubmit runs: %w", err)
	}
	return runs, nil
}

func scrapPresubmitRun(creds *cirunnerpb.JenkinsCredentials, number int) (Run, error) {
	jobPath := "job/presubmits/job/presubmit"
	path := fmt.Sprintf("%s/%d/api/json", jobPath, number)
	body, err := jenkins.SendJenkinsRequest(creds, "GET", path, map[string]string{
		"tree": "result,duration,timestamp,actions[parameters[name,value]]",
	})
	if err != nil {
		return nil, fmt.Errorf("could not GET %q: %w", path, err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		return nil, fmt.Errorf("could not unmarshal body for GET %q: %w", path, err)
	}
	run := &PresubmitRun{
		Id:     number,
		Result: result["result"].(string),
	}
	// Timestamp.
	fts := result["timestamp"].(float64) / 1000
	t := time.Unix(int64(fts), 0)
	run.Timestamp = t
	// Duration.
	d := int64(result["duration"].(float64))
	run.Duration = time.Duration(d) * time.Millisecond
	// Go over the parameters
	for _, a := range result["actions"].([]interface{}) {
		action := a.(map[string]interface{})
		val, ok := action["_class"]
		if !ok {
			continue
		}
		class := val.(string)
		if strings.HasSuffix(class, "ParametersAction") {
			parameters := action["parameters"].([]interface{})
			for _, p := range parameters {
				parameter := p.(map[string]interface{})
				name := parameter["name"].(string)
				value := parameter["value"].(string)
				if value == "" {
					continue
				}
				if name == "change" {
					run.Change, _ = strconv.Atoi(value)
				} else if name == "baseCl" {
					run.BaseCl, _ = strconv.Atoi(value)
				} else if name == "Review" {
					run.Review, _ = strconv.Atoi(value)
				}
			}
		}
	}
	if machine, err := obtainMachineFromLogs(creds, jobPath, number); err != nil {
		return nil, fmt.Errorf("coult not obtain machine for %q with id %d: %w", jobPath, number, err)
	} else {
		run.Machine = machine
	}
	fmt.Println(run.AsCsv())
	return run, nil
}

// UnitRunner --------------------------------------------------------------------------------------

var unitRunnerHeaders = []string{
	"Number", "Result", "Duration", "Timestamp", "Change", "BaseCl",
	"BuildUnit", "PublishUnit", "TestUnit", "TaskUnit",
	"Machine",
}

type UnitRunnerRun struct {
	Id          int
	Result      string
	Duration    time.Duration
	Timestamp   time.Time
	Change      int
	BaseCl      int
	BuildUnit   string
	PublishUnit string
	TestUnit    string
	TaskUnit    string
	Machine     string
}

func (run *UnitRunnerRun) Number() int {
	return run.Id
}

func (run *UnitRunnerRun) AsCsv() []string {
	return []string{
		strconv.Itoa(run.Id),
		run.Result,
		run.Duration.String(),
		run.Timestamp.String(),
		strconv.Itoa(run.Change),
		strconv.Itoa(run.BaseCl),
		run.BuildUnit,
		run.PublishUnit,
		run.TestUnit,
		run.TaskUnit,
		run.Machine,
	}
}

var machineRegex = regexp.MustCompile(`Running on (.+) in C`)

func scrapUnitRunner(creds *cirunnerpb.JenkinsCredentials) ([]Run, error) {
	runs, err := scrapJob(creds, "job/unit/job/unit_runner", scrapUnitRunnerRun)
	if err != nil {
		return nil, fmt.Errorf("could not scrap unit runs: %w", err)
	}
	return runs, nil
}

func scrapUnitRunnerRun(creds *cirunnerpb.JenkinsCredentials, number int) (Run, error) {
	jobPath := "job/unit/job/unit_runner"
	path := fmt.Sprintf("%s/%d/api/json", jobPath, number)
	body, err := jenkins.SendJenkinsRequest(creds, "GET", path, map[string]string{
		"tree": "result,duration,timestamp,actions[parameters[name,value]]",
	})
	if err != nil {
		return nil, fmt.Errorf("could not GET %q: %w", path, err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(body), &result); err != nil {
		return nil, fmt.Errorf("could not unmarshal body for GET %q: %w", path, err)
	}
	run := &UnitRunnerRun{
		Id:     number,
		Result: result["result"].(string),
	}
	// Timestamp.
	fts := result["timestamp"].(float64) / 1000
	t := time.Unix(int64(fts), 0)
	run.Timestamp = t
	// Duration.
	d := int64(result["duration"].(float64))
	run.Duration = time.Duration(d) * time.Millisecond
	// Go over the parameters
	for _, a := range result["actions"].([]interface{}) {
		action := a.(map[string]interface{})
		val, ok := action["_class"]
		if !ok {
			continue
		}
		class := val.(string)
		if strings.HasSuffix(class, "ParametersAction") {
			parameters := action["parameters"].([]interface{})
			for _, p := range parameters {
				parameter := p.(map[string]interface{})
				name := parameter["name"].(string)
				value := parameter["value"].(string)
				if value == "" {
					continue
				}
				if name == "change" {
					run.Change, _ = strconv.Atoi(value)
				} else if name == "baseCl" {
					run.BaseCl, _ = strconv.Atoi(value)
				} else if name == "buildUnit" {
					run.BuildUnit = value
				} else if name == "publishUnit" {
					run.PublishUnit = value
				} else if name == "testUnit" {
					run.TestUnit = value
				} else if name == "taskUnit" {
					run.TaskUnit = value
				}
			}
		}
	}
	if machine, err := obtainMachineFromLogs(creds, jobPath, number); err != nil {
		return nil, fmt.Errorf("coult not obtain machine for %q with id %d: %w", jobPath, number, err)
	} else {
		run.Machine = machine
	}
	fmt.Println(run.AsCsv())
	return run, nil
}

func writeCsv(headers []string, runs []Run, dir, filename string) error {
	path := filepath.Join(dir, filename)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("could not open %q: %w", path, err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	w.Write(headers)
	for _, run := range runs {
		w.Write(run.AsCsv())
		w.Flush()
	}
	return nil
}

// internalMain exists because os.Exit doesn't respect defer.
func internalMain() error {
	if err := parseFlags(); err != nil {
		return fmt.Errorf("could not parse flags: %w", err)
	}
	ciProject := "INSERT_PROJECT"
	creds, err := jenkins.CredentialsForProject(ciProject)
	if err != nil {
		return fmt.Errorf("could not get credentials for project %q: %w", ciProject, err)
	}
	// Because we're running locally, we need to change the host. When running in the workstation,
	// we pipe locally.
	creds.Host = "INSERT_HOST"

	runs, err := scrapUnitRunner(creds)
	if err != nil {
		return fmt.Errorf("could not scrap unit runner: %w", err)
	}
	if err := writeCsv(unitRunnerHeaders, runs, flags.out, "unit.csv"); err != nil {
		return fmt.Errorf("could not write unit csv: %w", err)
	}

	runs, err = scrapPresubmitRunner(creds)
	if err != nil {
		return fmt.Errorf("could not scrap presubmit runner: %w", err)
	}
	if err := writeCsv(presubmitHeaders, runs, flags.out, "presubmit.csv"); err != nil {
		return fmt.Errorf("could not write presubmit csv: %w", err)
	}
	return nil
}

func main() {
	if err := internalMain(); err != nil {
		fmt.Println("ERROR:", err)
		os.Exit(1)
	}
}
