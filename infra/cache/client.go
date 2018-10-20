package cache

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

import (
	"bytes"
	"encoding/gob"

	"github.com/bradfitz/gomemcache/memcache"
)

// GobCache stores and retrieves values using a memcache client using the gob
// encoding package. It does not currently allow for expiration of items.
// With a nil gobCache, Set is a no-op and Get will always return memcache.ErrCacheMiss.
type GobCache interface {
	Set(key string, v interface{}) error
	Get(key string, v interface{}) error
	ErrCacheMiss() error
}

type memcacheImp struct {
	client *memcache.Client
}

func NewGobCacheM(memcacheClient *memcache.Client) GobCache {
	return &memcacheImp{memcacheClient}
}

func (c *memcacheImp) Set(key string, v interface{}) error {
	if c == nil || c.client == nil {
		return nil
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return err
	}
	return c.client.Set(&memcache.Item{Key: key, Value: buf.Bytes()})
}

func (c *memcacheImp) Get(key string, v interface{}) error {
	if c == nil || c.client == nil {
		return memcache.ErrCacheMiss
	}
	item, err := c.client.Get(key)
	if err != nil {
		return err
	}
	return gob.NewDecoder(bytes.NewBuffer(item.Value)).Decode(v)
}

func (c *memcacheImp) ErrCacheMiss() error {
	return memcache.ErrCacheMiss
}
