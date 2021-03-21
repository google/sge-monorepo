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

// Package trie contains trie data structures.
package trie

import "strings"

// PathTrie is a mapping of paths to value.
// The paths are case sensitive and segmented by forward slashes.
type PathTrie struct {
	value interface{}
	nodes map[string]*PathTrie
}

func NewPathTrie() *PathTrie {
	return &PathTrie{nodes: map[string]*PathTrie{}}
}

// Put associates a value with the given path.
func (pt *PathTrie) Put(p string, value interface{}) {
	cur := pt
	for _, part := range strings.Split(p, "/") {
		node, ok := cur.nodes[part]
		if !ok {
			node = NewPathTrie()
			cur.nodes[part] = node
		}
		cur = node
	}
	cur.value = value
}

// Get gets the value associated with the given path.
func (pt *PathTrie) Get(p string) interface{} {
	cur := pt
	for _, part := range strings.Split(p, "/") {
		var ok bool
		if cur, ok = cur.nodes[part]; !ok {
			return nil
		}
	}
	return cur.value
}

// GetLongestPrefix returns the last parent in the trie to the given path.
func (pt *PathTrie) GetLongestPrefix(p string) interface{} {
	cur := pt
	value := pt.value
	for _, part := range strings.Split(p, "/") {
		node, ok := cur.nodes[part]
		if !ok {
			break
		}
		if node.value != nil {
			value = node.value
		}
		cur = node
	}
	return value
}

// GetShortestPrefix returns the first parent in the trie to the given path.
func (pt *PathTrie) GetShortestPrefix(p string) interface{} {
	cur := pt
	for _, part := range strings.Split(p, "/") {
		node, ok := cur.nodes[part]
		if !ok {
			break
		}
		if node.value != nil {
			return node.value
		}
		cur = node
	}
	return nil
}
