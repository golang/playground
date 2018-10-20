package cache

// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/gomodule/redigo/redis"
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
	if err := encode(buf, v); err != nil {
		return err
	}
	return c.client.Set(&memcache.Item{Key: key, Value: buf.Bytes()})
}

func (c *memcacheImp) Get(key string, v interface{}) error {
	if c == nil || c.client == nil {
		return c.ErrCacheMiss()
	}
	item, err := c.client.Get(key)
	if err != nil {
		return err
	}

	return decode(item.Value, v)
}

func (c *memcacheImp) ErrCacheMiss() error {
	return memcache.ErrCacheMiss
}

type redisImp struct {
	pool *redis.Pool
}

func NewGobCacheR(pool *redis.Pool) GobCache {
	return &redisImp{pool}
}

func (c *redisImp) Set(key string, v interface{}) error {
	if c == nil || c.pool == nil {
		return nil
	}

	var buf bytes.Buffer
	if err := encode(buf, v); err != nil {
		return err
	}

	_, err := c.pool.Get().Do("SET", buf.Bytes())
	return err
}

func (c *redisImp) Get(key string, v interface{}) error {
	if c == nil || c.pool == nil {
		return c.ErrCacheMiss()
	}

	value, err := redis.Bytes(c.pool.Get().Do("GET", key))
	if err != nil {
		return err
	}

	return decode(value, v)
}

func (c *redisImp) ErrCacheMiss() error {
	return fmt.Errorf("Cache miss")
}

func encode(buf bytes.Buffer, v interface{}) error {
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return err
	}
	return nil
}

func decode(value []byte, v interface{}) error {
	return gob.NewDecoder(bytes.NewBuffer(value)).Decode(v)
}
