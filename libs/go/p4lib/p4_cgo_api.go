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
	"io"
	"io/ioutil"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/golang/glog"
)

// #cgo CPPFLAGS: -I${SRCDIR}/../../../third_party/p4api/include
// #cgo LDFLAGS: -L${SRCDIR}/../../../third_party/lib -lp4api -lssl -lcrypto
// #cgo windows LDFLAGS: -lws2_32
// #include "p4_cgo_bridge.h"
// strview p4str(_GoString_ s);
import "C"

// Clients should implement one or more of the following interfaces to
// receive callbacks from perforce.

// BinaryHandler handles the output of 'p4 print' on a binary file.
type BinaryHandler interface {
	outputBinary(data []byte) error
}

// TextHandler handles the output of 'p4 print' on a text file.
type TextHandler interface {
	outputText(data string) error
}

// InfoHandler handles info output from perforce.
type InfoHandler interface {
	outputInfo(level int, info string) error
}

// StatHandler handles tagged output from perforce.
type StatHandler interface {
	outputStat(stats map[string]string) error
}

// RetryHandler handles retry notifications.  Implementations may implement
// this to reset internal state before a command is retried.
type RetryHandler interface {
	onRetry(context, err string)
}

// Clients may implement the Tagger interface to indicate that the tag protocol
// should be used.  This is roughly equivalent to 'p4 -Ztag'.
type Tagger interface {
	tagProtocol()
}

type handler struct {
	err string
	cb  interface{}
}

func (h *handler) handleError(err string) {
	if len(h.err) > 0 {
		h.err = fmt.Sprintf("%v : %v", h.err, err)
	} else {
		h.err = err
	}
}

func (h *handler) outputBinary(data []byte) {
	if cb, ok := h.cb.(BinaryHandler); ok {
		if err := cb.outputBinary(data); err != nil {
			h.handleError(err.Error())
		}
	} else {
		glog.Warning("no handler for outputBinary")
	}
}

func (h *handler) outputText(data string) {
	if cb, ok := h.cb.(TextHandler); ok {
		if err := cb.outputText(data); err != nil {
			h.handleError(err.Error())
		}
	} else {
		glog.Warning("no handler for outputText")
	}
}

func (h *handler) outputInfo(level int, info string) {
	if cb, ok := h.cb.(InfoHandler); ok {
		if err := cb.outputInfo(level, info); err != nil {
			h.handleError(err.Error())
		}
	} else {
		glog.Warning("no handler for outputInfo")
	}
}

func (h *handler) outputStat(stats map[string]string) {
	if cb, ok := h.cb.(StatHandler); ok {
		if err := cb.outputStat(stats); err != nil {
			h.handleError(err.Error())
		}
	} else {
		glog.Warning("no handler for outputStat")
	}
}

func (h *handler) retry(context, err string) {
	if cb, ok := h.cb.(RetryHandler); ok {
		cb.onRetry(context, err)
	} else {
		glog.Infof("retrying because: %v: %v", context, err)
		h.err = ""
	}
}

type handlerMap struct {
	lock     sync.RWMutex
	next     int
	handlers map[int]*handler
}

func (h *handlerMap) register(cb interface{}) (int, *handler) {
	h.lock.Lock()
	defer h.lock.Unlock()
	cbid := h.next
	h.next++
	h.handlers[cbid] = &handler{cb: cb}
	return cbid, h.handlers[cbid]
}
func (h *handlerMap) unregister(cbid int) {
	h.lock.Lock()
	defer h.lock.Unlock()
	delete(h.handlers, cbid)
}
func (h *handlerMap) get(cbid int) (*handler, bool) {
	h.lock.RLock()
	defer h.lock.RUnlock()
	cb, ok := h.handlers[cbid]
	return cb, ok
}

var handlers = handlerMap{
	next:     1,
	handlers: map[int]*handler{},
}

// C callable wrappers for the callback interface.

//export gop4apiHandleError
func gop4apiHandleError(cbid int, err *C.char, len C.int) {
	if cb, ok := handlers.get(cbid); ok {
		cb.handleError(C.GoStringN(err, len))
	}
}

//export gop4apiOutputBinary
func gop4apiOutputBinary(cbid int, data *C.char, len C.int) {
	if cb, ok := handlers.get(cbid); ok {
		cb.outputBinary(C.GoBytes(unsafe.Pointer(data), len))
	}
}

//export gop4apiOutputText
func gop4apiOutputText(cbid int, data *C.char, len C.int) {
	if cb, ok := handlers.get(cbid); ok {
		cb.outputText(C.GoStringN(data, len))
	}
}

//export gop4apiOutputInfo
func gop4apiOutputInfo(cbid int, level C.int, info *C.char) {
	if cb, ok := handlers.get(cbid); ok {
		cb.outputInfo(int(level), C.GoString(info))
	}
}

//export gop4apiOutputStat
func gop4apiOutputStat(cbid int, count int, keys, values *C.strview) {
	if cb, ok := handlers.get(cbid); ok {
		// We convert keys & values to Go slices by casting the pointer to
		// a pointer to a very large array of C.strview, the slicing the valid
		// bit (from https://github.com/golang/go/wiki/cgo#turning-c-arrays-into-go-slices).
		keyslice := (*[1 << 28]C.strview)(unsafe.Pointer(keys))[:count:count]
		valslice := (*[1 << 28]C.strview)(unsafe.Pointer(values))[:count:count]
		stats := map[string]string{}
		for i := 0; i < count; i++ {
			key := C.GoStringN(keyslice[i].p, keyslice[i].len)
			val := C.GoStringN(valslice[i].p, valslice[i].len)
			stats[key] = val
		}
		cb.outputStat(stats)
	}
}

//export gop4apiRetry
func gop4apiRetry(cbid int, context, err *C.char, len C.int) {
	if cb, ok := handlers.get(cbid); ok {
		cb.retry(C.GoString(context), C.GoStringN(err, len))
	}
}

// runCmdApiCb invokes the given p4 command, invoking the callback interface
// as appropriate to handle output.
// TODO: rename to something more descriptive.
func (p4 *impl) runCmdCb(cb interface{}, cmd string, args ...string) error {
	if p4.tracer != nil {
		endtrace := p4.tracer(cmd)
		defer endtrace()
	}
	start := time.Now()

	joined := strings.Join(args, "")
	argv := make([]C.int, len(args))
	for i, arg := range args {
		argv[i] = C.int(len(arg))
	}

	_, tag := cb.(Tagger)

	input := C.strview{}
	if reader, ok := cb.(io.Reader); ok {
		data, err := ioutil.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("failed to read input: %w", err)
		}
		if data != nil && len(data) > 0 {
			input.p = (*C.char)(unsafe.Pointer(&data[0]))
			input.len = C.int(len(data))
		}
	}
	cbid, handler := handlers.register(cb)
	defer handlers.unregister(cbid)

	init_us := C.p4runcb(C.p4str(cmd), C.p4str(p4.user), C.p4str(p4.passwd), input, C.p4str(joined), C.int(len(argv)), unsafe.Pointer(&argv[0]), C.int(cbid), C.bool(tag))

	duration := time.Since(start)
	updateStats(cmd, duration.Microseconds(), int64(init_us))

	if handler.err != "" {
		return fmt.Errorf("p4 api error: %v", handler.err)
	}
	return nil
}
