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

// Package log contains a logging indirection. Libraries are encouraged to log using this library,
// allowing the main binary to control the log sinks used (glog, cloud logging...)
package log

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/golang/glog"
)

var logger = NewWithDepth(1)

// Logger is the primary interface to the logging system. It logs lines to all of its sinks.
type Logger struct {
	sinks []Sink
	depth int
}

// New returns a new Logger with zero depth.
func New() *Logger {
	return &Logger{}
}

// NewWithDepth returns a new logger with the given depth.
// For log sinks that support is, it means that this many stack frames
// are discarded.
func NewWithDepth(depth int) *Logger {
	return &Logger{depth: depth}
}

// Get returns the global logger with the depth reset to 0.
func Get() *Logger {
	return &Logger{sinks: logger.sinks}
}

// Sink is a sink for log calls.
type Sink interface {
	DebugDepth(depth int, msg string)
	InfoDepth(depth int, msg string)
	WarningDepth(depth int, msg string)
	ErrorDepth(depth int, msg string)
	Close()
}

// AddSink adds one or more log sinks.
// You should call defer Shutdown() after calling this.
func (l *Logger) AddSink(s ...Sink) {
	l.sinks = append(l.sinks, s...)
}

// Shutdown flushes and closes all log sinks.
func (l *Logger) Shutdown() {
	for _, s := range l.sinks {
		s.Close()
	}
}

// Debugf logs an debug level log line.
func (l *Logger) Debugf(format string, args ...interface{}) {
	for _, s := range l.sinks {
		s.DebugDepth(l.depth+2, fmt.Sprintf(format, args...))
	}
}

func (l *Logger) Debug(args ...interface{}) {
	msg := defaultFmt(args...)
	for _, s := range l.sinks {
		s.DebugDepth(l.depth+2, msg)
	}
}

// Infof logs an info level log line.
func (l *Logger) Infof(format string, args ...interface{}) {
	for _, s := range l.sinks {
		s.InfoDepth(l.depth+2, fmt.Sprintf(format, args...))
	}
}

func (l *Logger) Info(args ...interface{}) {
	msg := defaultFmt(args...)
	for _, s := range l.sinks {
		s.InfoDepth(l.depth+2, msg)
	}
}

// Warningf logs a warning level log line.
func (l *Logger) Warningf(format string, args ...interface{}) {
	for _, s := range l.sinks {
		s.WarningDepth(l.depth+2, fmt.Sprintf(format, args...))
	}
}

func (l *Logger) Warning(args ...interface{}) {
	msg := defaultFmt(args...)
	for _, s := range l.sinks {
		s.WarningDepth(l.depth+2, msg)
	}
}

// Errorf logs an error level log line.
func (l *Logger) Errorf(format string, args ...interface{}) {
	for _, s := range l.sinks {
		s.ErrorDepth(l.depth+2, fmt.Sprintf(format, args...))
	}
}

func (l *Logger) Error(args ...interface{}) {
	msg := defaultFmt(args...)
	for _, s := range l.sinks {
		s.ErrorDepth(l.depth+2, msg)
	}
}

// Fatalf logs an error level log line then terminates the program.
func (l *Logger) Fatalf(format string, args ...interface{}) {
	for _, s := range l.sinks {
		s.ErrorDepth(l.depth+2, fmt.Sprintf(format, args...))
	}
	os.Exit(1)
}

func (l *Logger) Fatal(args ...interface{}) {
	msg := defaultFmt(args...)
	for _, s := range l.sinks {
		s.ErrorDepth(l.depth+2, msg)
	}
	os.Exit(1)
}

// AddSink adds one or more global log sinks
// You should call defer log.Shutdown() after calling this.
func AddSink(s ...Sink) {
	logger.AddSink(s...)
}

// Shutdown flushes and closes all global loggers.
func Shutdown() {
	logger.Shutdown()
}

// Debugf logs an debug level log line.
func Debugf(format string, args ...interface{}) {
	logger.Debugf(format, args...)
}

func Debug(args ...interface{}) {
	logger.Debug(args...)
}

// Infof logs an info level log line.
func Infof(format string, args ...interface{}) {
	logger.Infof(format, args...)
}

func Info(args ...interface{}) {
	logger.Info(args...)
}

// Warningf logs a warning level log line.
func Warningf(format string, args ...interface{}) {
	logger.Warningf(format, args...)
}

func Warning(args ...interface{}) {
	logger.Warning(args...)
}

// Errorf logs an error level log line.
func Errorf(format string, args ...interface{}) {
	logger.Errorf(format, args...)
}

func Error(args ...interface{}) {
	logger.Error(args...)
}

// Fatalf logs an error level log line then terminates the program.
func Fatalf(format string, args ...interface{}) {
	logger.Fatalf(format, args...)
}

func Fatal(args ...interface{}) {
	logger.Fatal(args...)
}

// NewGlog returns a glog log sink.
func NewGlog() Sink {
	return &glogSink{}
}

type glogSink struct{}

func (g *glogSink) DebugDepth(depth int, msg string) {
	// Debugf is special because glog does not have a DEBUG level, so we defer to INFO.
	glog.InfoDepth(depth, msg)
}

func (g *glogSink) InfoDepth(depth int, msg string) {
	glog.InfoDepth(depth, msg)
}

func (g *glogSink) WarningDepth(depth int, msg string) {
	glog.WarningDepth(depth, msg)
}

func (g *glogSink) ErrorDepth(depth int, msg string) {
	glog.ErrorDepth(depth, msg)
}

func (g *glogSink) Close() {
	glog.Flush()
}

// NewInfoLogger returns a logger that writes a log.Info to glog for every complete line written to it.
func NewInfoLogger(l *Logger) io.Writer {
	return &infoLogger{logger: l}
}

type infoLogger struct {
	logger *Logger
	str    strings.Builder
}

func (l *infoLogger) Write(p []byte) (n int, err error) {
	str := string(p)
	for _, c := range str {
		if c == '\n' {
			l.writeLine()
			continue
		} else if c == '\r' {
			continue
		}
		l.str.WriteRune(c)
	}
	return len(p), nil
}

func (l *infoLogger) Close() error {
	l.writeLine()
	return nil
}

func (l *infoLogger) writeLine() {
	toWrite := l.str.String()
	if len(toWrite) > 0 {
		l.logger.Info(toWrite)
	}
	l.str.Reset()
}

const defaultMaxArgs = 10

var defaultFmtStr = strings.TrimSpace(strings.Repeat("%v ", defaultMaxArgs))

func defaultFmtInternal(args ...interface{}) (string, []interface{}) {
	n := len(args)
	if n > defaultMaxArgs {
		n = defaultMaxArgs
	}
	return defaultFmtStr[(defaultMaxArgs-n)*3:], args[0:n]
}

func defaultFmt(args ...interface{}) string {
    format, truncatedArgs := defaultFmtInternal(args)
	return fmt.Sprintf(format, truncatedArgs...)
}
