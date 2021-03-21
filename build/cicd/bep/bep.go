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

// Package bep provides utilities for parsing Bazel build event output.
// See https://docs.bazel.build/versions/master/build-event-protocol.html.
package bep

import (
	"fmt"
	"hash/maphash"
	"reflect"

	bepb "bazel.io/src/main/java/com/google/devtools/build/lib/buildeventstream/proto"
	"github.com/golang/protobuf/proto"
	"google.golang.org/protobuf/encoding/protowire"
)

// Stream is a parsed version of the BEP stream.
type Stream struct {
	// events is a map of <id hash> -> build event
	Events map[uint64]*bepb.BuildEvent
	// depsets is a graph of all the NestedSetOfFiles messages.
	Depsets Depsets
}

// Parse parses a build event stream file returned from Bazel and returns the BuildEvent messages in it.
func Parse(buf []byte) (*Stream, error) {
	events, err := readEvents(buf)
	if err != nil {
		return nil, err
	}
	depsets := newDepsets(events)
	idToEvent := map[uint64]*bepb.BuildEvent{}
	for _, event := range events {
		key, err := EventKey(event.Id)
		if err != nil {
			return nil, err
		}
		idToEvent[key] = event
	}
	return &Stream{
		Events:  idToEvent,
		Depsets: depsets,
	}, nil
}

func readEvents(buf []byte) ([]*bepb.BuildEvent, error) {
	var events []*bepb.BuildEvent
	// The build event file format is of the form: (<size of proto: varint><event: BuildEvent>)*.
	// buf always contains the remainder of the buffer.
	for len(buf) > 0 {
		protoSize, intSize := protowire.ConsumeVarint(buf)
		if err := protowire.ParseError(intSize); err != nil {
			return nil, fmt.Errorf("unable to parse build event file: %v", err)
		}
		buf = buf[intSize:]
		if protoSize > uint64(len(buf)) {
			return nil, fmt.Errorf("incomplete build event file [%d of %d bytes]", protoSize, len(buf))
		}
		var be bepb.BuildEvent
		if err := proto.Unmarshal(buf[:protoSize], &be); err != nil {
			return nil, fmt.Errorf("unable to parse build events: %v", err)
		}
		events = append(events, &be)
		buf = buf[protoSize:]
	}
	return events, nil
}

// Needed to return deterministic keys for multiple calls to eventKey.
var fixedSeed = maphash.MakeSeed()

// EventKey computes a map key from the given build event id.
func EventKey(id *bepb.BuildEventId) (uint64, error) {
	bytes, err := proto.Marshal(id)
	if err != nil {
		return 0, err
	}
	var hash maphash.Hash
	hash.SetSeed(fixedSeed)
	_, _ = hash.WriteString(reflect.TypeOf(id).Name()) // Cannot fail
	_, _ = hash.Write(bytes)                           // Cannot fail
	return hash.Sum64(), nil
}

// Depsets is a map of id -> output depsets found in the BEP stream.
// See https://docs.bazel.build/versions/master/skylark/lib/depset.html for information about depsets.
type Depsets map[string]*bepb.NamedSetOfFiles

// NewDepsets returns the map of output depsets found in the BEP stream.
// See https://docs.bazel.build/versions/master/skylark/lib/depset.html for information about depsets.
func newDepsets(events []*bepb.BuildEvent) Depsets {
	depsets := Depsets{}
	for _, be := range events {
		// We only care about depset events.
		if id, ok := be.Id.Id.(*bepb.BuildEventId_NamedSet); ok {
			if nsof, ok := be.Payload.(*bepb.BuildEvent_NamedSetOfFiles); ok {
				depsets[id.NamedSet.Id] = nsof.NamedSetOfFiles
			}
		}
	}
	return depsets
}

// Files returns a set of files given a list of depset ids.
func (d Depsets) Files(depsets []*bepb.BuildEventId_NamedSetOfFilesId) map[string]*bepb.File {
	visited := map[string]bool{}
	files := map[string]*bepb.File{}
	for _, set := range depsets {
		d.collect(visited, files, set.Id)
	}
	return files
}

// collect recurses DFS into the depsets and collects all output files.
func (d Depsets) collect(visited map[string]bool, files map[string]*bepb.File, id string) {
	if _, ok := visited[id]; ok {
		return
	}
	visited[id] = true
	if set, ok := d[id]; ok {
		for _, f := range set.Files {
			files[f.Name] = f
		}
		for _, sub := range set.FileSets {
			d.collect(visited, files, sub.Id)
		}
	}
	return
}
