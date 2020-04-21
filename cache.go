// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/gob"

	"github.com/bradfitz/gomemcache/memcache"
)

// responseCache is a common interface for cache implementations.
type responseCache interface {
	// Set sets the value for a key.
	Set(key string, v interface{}) error
	// Get sets v to the value stored for a key.
	Get(key string, v interface{}) error
}

// gobCache stores and retrieves values using a memcache client using the gob
// encoding package. It does not currently allow for expiration of items.
// With a nil gobCache, Set is a no-op and Get will always return memcache.ErrCacheMiss.
type gobCache struct {
	client *memcache.Client
}

func newGobCache(addr string) *gobCache {
	return &gobCache{memcache.New(addr)}
}

func (c *gobCache) Set(key string, v interface{}) error {
	if c == nil {
		return nil
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return err
	}
	return c.client.Set(&memcache.Item{Key: key, Value: buf.Bytes()})
}

func (c *gobCache) Get(key string, v interface{}) error {
	if c == nil {
		return memcache.ErrCacheMiss
	}
	item, err := c.client.Get(key)
	if err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewBuffer(item.Value)).Decode(v)
}
