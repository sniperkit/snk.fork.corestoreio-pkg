// Copyright 2015-2016, Cyrill @ Schumacher.fm and the CoreStore contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package blacklist

import (
	"hash"
	"sync"
	"time"

	"github.com/corestoreio/csfw/util/hashpool"
)

// InMemory creates an in-memory map which holds as a key the ID and as value an
// expiration duration. Once a Set() operation will be called the ID list get
// purged. The map type has been optimized for less GC and can hold millions of
// entries.
type InMemory struct {
	hp hashpool.Tank
	mu sync.RWMutex
	// keys contains a map consisting only of integers which skips scanning a
	// map by the GC.
	keys map[uint64]int64 // int64 unix timestamp
}

// NewInMemory creates a new blacklist map using the Hash for key generation.
// Please choose a Hash function with less collisions.
func NewInMemory(hf func() hash.Hash64) *InMemory {
	return &InMemory{
		hp:   hashpool.New64(hf),
		keys: make(map[uint64]int64),
	}
}

// hash generates a hash value of a byte slice.
func (bl *InMemory) hash(id []byte) uint64 {
	hf := bl.hp.Get()
	_, _ = hf.Write(id)
	s := hf.Sum64()
	bl.hp.Put(hf)
	return s

}

// Has checks if an ID has been stored in the blacklist and may delete the ID if
// expiration time is up.
func (bl *InMemory) Has(id []byte) bool {

	bl.mu.RLock()
	h := bl.hash(id)
	ts, ok := bl.keys[h]
	bl.mu.RUnlock()

	if !ok {
		return false
	}
	isValid := time.Now().Unix() < ts

	if false == isValid {
		bl.mu.Lock()
		delete(bl.keys, h)
		bl.mu.Unlock()
	}
	return isValid
}

// Set adds an ID to the blacklist and may perform a purge operation. Set should
// be called when you log out a user. Set must make sure to copy away the bytes
// or hash them.
func (bl *InMemory) Set(id []byte, expires time.Duration) error {
	h := bl.hash(id)

	bl.mu.Lock()
	defer bl.mu.Unlock()

	now := time.Now().Unix()
	for k, v := range bl.keys {
		if now > v { // todo call purge operation not that often ...
			delete(bl.keys, k)
		}
	}
	bl.keys[h] = time.Now().Add(expires).Unix()
	return nil
}

// Len returns the number of entries in the blacklist
func (bl *InMemory) Len() int {
	bl.mu.RLock()
	l := len(bl.keys)
	bl.mu.RUnlock()
	return l
}
