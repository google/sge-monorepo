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

package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unsafe"

	"sge-monorepo/tools/ebert/ebert"
)

var (
	ErrHandlerSig = errors.New("handler has invalid signature")
)

// Handler defines an abstract interface for Ebert handlers.
type Handler interface {
	Serve(ctx *ebert.Context, r *http.Request) (interface{}, error)
}

// handlerFunc is a wrapper for converting a function to a Handler.
type handlerFunc func(ctx *ebert.Context, r *http.Request) (interface{}, error)

// Serve implements the Handler interface by invoking the function.
func (h handlerFunc) Serve(ctx *ebert.Context, r *http.Request) (interface{}, error) {
	return h(ctx, r)
}

// Wrap converts a path pattern and a function with the signature
//   func(*ebert.Context, *http.Request, *Args) (interface{}, error)
// into a Handler which can be used with a Mux.
func Wrap(pattern string, h interface{}) (Handler, error) {
	if handler, ok := h.(Handler); ok {
		return handler, nil
	}
	hv := reflect.ValueOf(h)
	if hv.Kind() != reflect.Func {
		return nil, fmt.Errorf("%w: unexpected type %v for handler", ErrHandlerSig, hv.Kind())
	}

	ht := hv.Type()
	if ht.NumIn() < 2 || ht.NumIn() > 3 {
		return nil, fmt.Errorf("%w: want 2 or 3 args, got %d", ErrHandlerSig, ht.NumIn())
	}
	if ht.NumOut() != 2 {
		return nil, fmt.Errorf("%w: want 2 return values, got %d", ErrHandlerSig, ht.NumOut())
	}

	errInterface := reflect.TypeOf((*error)(nil)).Elem()
	if ht.Out(1).Kind() != reflect.Interface || !ht.Out(1).Implements(errInterface) {
		return nil, fmt.Errorf("%w: handler should return (interface{}, error), got (%v, %v)", ErrHandlerSig, ht.Out(0), ht.Out(1))
	}
	if ht.In(0) != reflect.TypeOf((*ebert.Context)(nil)) {
		return nil, fmt.Errorf("%w: handler 1st arg want *ebert.Context, got %v", ErrHandlerSig, ht.In(0))
	}
	if ht.In(1) != reflect.TypeOf((*http.Request)(nil)) {
		return nil, fmt.Errorf("%w: handler 2nd arg want *http.Request, got %v", ErrHandlerSig, ht.In(1))
	}
	if ht.NumIn() == 3 && (ht.In(2).Kind() != reflect.Ptr || ht.In(2).Elem().Kind() != reflect.Struct) {
		return nil, fmt.Errorf("%w: handler 3rd arg want *struct, got %v", ErrHandlerSig, ht.In(2))
	}

	var makeArg argParser
	at := reflect.TypeOf(nil)
	if ht.NumIn() == 3 {
		at = ht.In(2).Elem()
	}
	switch {
	case ht.NumIn() == 2:
		// Do nothing here -- we leave makeArg nil.
	case at != nil && at.Kind() == reflect.Struct:
		makeArg = makeArgParser(pattern, at)
	default:
		return nil, fmt.Errorf("%w: unexpected type %v for handler arg", ErrHandlerSig, at)
	}

	return handlerFunc(func(ctx *ebert.Context, r *http.Request) (interface{}, error) {
		var out []reflect.Value
		if makeArg == nil {
			out = hv.Call([]reflect.Value{
				reflect.ValueOf(ctx),
				reflect.ValueOf(r),
			})
		} else {
			arg, err := makeArg(r)
			if err != nil {
				return nil, fmt.Errorf("arg parser error: %w", err)
			}
			out = hv.Call([]reflect.Value{
				reflect.ValueOf(ctx),
				reflect.ValueOf(r),
				arg,
			})
		}
		errIf := out[1].Interface()
		if errIf != nil {
			return out[0].Interface(), errIf.(error)
		}
		return out[0].Interface(), nil
	}), nil
}

type argParser func(r *http.Request) (reflect.Value, error)

func makeArgParser(pattern string, argType reflect.Type) argParser {
	segments := []string{}
	split := strings.Index(pattern, ":")
	if split < 0 {
		split = len(pattern)
	} else {
		segments = append(segments, strings.Split(pattern[split:], "/")...)
	}
	prefix := pattern[0:split]

	return func(r *http.Request) (reflect.Value, error) {
		ptr := reflect.New(argType)

		parts := strings.Split(strings.TrimPrefix(r.URL.Path, prefix), "/")
		r.ParseForm()
		args := r.Form

		// Add path segments to args.
		for i, arg := range segments {
			if i > len(parts) {
				break
			}
			if arg != "" && arg[0] == ':' {
				if i == len(segments)-1 {
					args.Add(arg[1:], strings.Join(parts[i:], "/"))
				} else {
					args.Add(arg[1:], parts[i])
				}
			} else if arg != parts[i] {
				return ptr, fmt.Errorf("expected '%s' got '%s'", arg, parts[i])
			}
		}

		value := ptr.Elem()
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			if !field.CanSet() {
				field = reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
			}

			k := argType.Field(i).Name
			v := args.Get(k)
			switch field.Kind() {
			case reflect.Bool:
				if v == "" {
					if _, ok := args[k]; ok {
						v = "true"
					}
				}
				field.SetBool(v == "1" || v == "true" || v == "True")
			case reflect.Int:
				if v == "" {
					continue
				}
				i, err := strconv.Atoi(v)
				if err != nil {
					return ptr, fmt.Errorf("arg %s atoi error: %w", k, err)
				}
				field.SetInt(int64(i))
			case reflect.String:
				field.SetString(v)
			default:
				return ptr, fmt.Errorf("%w: unexpected field type: %v", ErrHandlerSig, field.Type())
			}
		}
		return ptr, nil
	}
}
