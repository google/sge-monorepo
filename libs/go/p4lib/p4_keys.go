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
	"strings"
)

// Define callback interface for working with keys.
type keycb map[string]string

func (cb *keycb) outputStat(stats map[string]string) error {
	key, ok := stats["key"]
	if !ok {
		return fmt.Errorf("missing 'key' in %v", stats)
	}
	value, ok := stats["value"]
	if !ok {
		return fmt.Errorf("missing 'value' in %v", stats)
	}
	(*cb)[key] = value
	return nil
}
func (cb *keycb) retry(context, err string) {
	*cb = keycb{}
}
func (cb *keycb) tagProtocol() {}

// KeyGet returns the value of the given key using p4 key.
func (p4 *impl) KeyGet(key string) (string, error) {
	cb := keycb{}
	err := p4.runCmdCb(&cb, "key", key)
	if err != nil {
		return "0", err
	}
	if v, ok := cb[key]; ok {
		return v, nil
	}
	return "0", ErrKeyNotFound
}

// KeySet sets the value of the given key.
func (p4 *impl) KeySet(key, val string) error {
	_, err := p4.ExecCmd("key", key, val)
	return err
}

// KeyInc increments the given integer key, and returns the new value.
func (p4 *impl) KeyInc(key string) (string, error) {
	cb := keycb{}
	err := p4.runCmdCb(&cb, "key", "-i", key)
	if err != nil {
		return "0", err
	}
	if v, ok := cb[key]; ok {
		return v, nil
	}
	return "0", ErrKeyNotFound
}

// KeyCas does a check-and-set of the value at the specified key.
func (p4 *impl) KeyCas(key, oldval, newval string) error {
	_, err := p4.ExecCmd("key", "--from", oldval, "--to", newval, key)
	if err != nil {
		if strings.Contains(err.Error(), fmt.Sprintf("New value for %s not set.", key)) {
			return ErrCasMismatch
		}
		return err
	}
	return nil
}

// Keys returns all key values that match the given pattern
func (p4 *impl) Keys(pattern string) (map[string]string, error) {
	cb := keycb{}
	err := p4.runCmdCb(&cb, "keys", "-e", pattern)
	if err != nil {
		return nil, err
	}
	return cb, nil
}
