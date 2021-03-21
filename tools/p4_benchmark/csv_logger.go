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
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"time"
)

type CsvLogger struct {
	csvPath string
}

func NewCsvLogger(csvPath string) *CsvLogger {
	return &CsvLogger{csvPath: csvPath}
}

func writeToCsv(csvLogger *CsvLogger, data []string) error {
	file, err := os.OpenFile(csvLogger.csvPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()

	return writer.Write(data)
}

func (csvLogger *CsvLogger) Log(phase string, name string, set string, repetition int, startTime time.Time, endTime time.Time, extras *map[string]string) error {
	if _, err := os.Stat(csvLogger.csvPath); os.IsNotExist(err) {
		// Write the CSV header
		headers := []string{"Phase", "Name", "Set", "Repetition", "StartTime", "EndTime", "Duration (ms)"}
		if extras != nil {
			extraHeaders := make([]string, 0, len(*extras))
			for key := range *extras {
				extraHeaders = append(extraHeaders, key)
			}
			sort.Strings(extraHeaders)
			headers = append(headers, extraHeaders...)
		}
		err = writeToCsv(csvLogger, headers)
		if err != nil {
			return fmt.Errorf("failed to create CSV file: %v", err)
		}
	}

	duration := int64(endTime.Sub(startTime) / time.Millisecond)
	values := []string{phase, name, set, fmt.Sprintf("%v", repetition), startTime.UTC().Format(time.RFC3339), endTime.UTC().Format(time.RFC3339), fmt.Sprintf("%v", duration)}
	if extras != nil {
		extraHeaders := make([]string, 0, len(*extras))
		for key := range *extras {
			extraHeaders = append(extraHeaders, key)
		}
		sort.Strings(extraHeaders)
		for _, key := range extraHeaders {
			values = append(values, (*extras)[key])
		}
	}

	err := writeToCsv(csvLogger, values)
	if err != nil {
		return fmt.Errorf("failed to write to CSV file: %v", err)
	}

	return nil
}
