// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store

import (
	"context"
	"fmt"
	"sync"

	"cloud.google.com/go/datastore"
	"github.com/gomodule/redigo/redis"
	"github.com/rerost/playground/model/snippet"
)

type Store interface {
	PutSnippet(ctx context.Context, id string, snip *snippet.Snippet) error
	GetSnippet(ctx context.Context, id string, snip *snippet.Snippet) error
	ErrNoSuchEntity() error
}

type cloudDatastoreImp struct {
	client *datastore.Client
}

func NewClienG(client *datastore.Client) Store {
	return &cloudDatastoreImp{client}
}

func (s *cloudDatastoreImp) PutSnippet(ctx context.Context, id string, snip *snippet.Snippet) error {
	key := datastore.NameKey("Snippet", id, nil)
	_, err := s.client.Put(ctx, key, snip)
	return err
}

func (s *cloudDatastoreImp) GetSnippet(ctx context.Context, id string, snip *snippet.Snippet) error {
	key := datastore.NameKey("Snippet", id, nil)
	return s.client.Get(ctx, key, snip)
}

func (s *cloudDatastoreImp) ErrNoSuchEntity() error {
	return datastore.ErrNoSuchEntity
}

// inMemStore is a store backed by a map that should only be used for testing.
type inMemStore struct {
	sync.RWMutex
	m map[string]*snippet.Snippet // key -> snippet
}

func NewClientInMem() Store {
	return &inMemStore{}
}

func (s *inMemStore) PutSnippet(_ context.Context, id string, snip *snippet.Snippet) error {
	s.Lock()
	if s.m == nil {
		s.m = map[string]*snippet.Snippet{}
	}
	b := make([]byte, len(snip.Body))
	copy(b, snip.Body)
	s.m[id] = &snippet.Snippet{Body: b}
	s.Unlock()
	return nil
}

func (s *inMemStore) GetSnippet(_ context.Context, id string, snip *snippet.Snippet) error {
	s.RLock()
	defer s.RUnlock()
	v, ok := s.m[id]
	if !ok {
		return datastore.ErrNoSuchEntity
	}
	*snip = *v
	return nil
}

func (s *inMemStore) ErrNoSuchEntity() error {
	return datastore.ErrNoSuchEntity
}

// redis
type redisStoreImp struct {
	pool *redis.Pool
}

func NewClientRedis(pool *redis.Pool) Store {
	return redisStoreImp{
		pool: pool,
	}
}

func (s redisStoreImp) PutSnippet(ctx context.Context, id string, snip *snippet.Snippet) error {
	_, err := s.pool.Get().Do("SET", id, snippet.Encode(snip))
	return err
}

func (s redisStoreImp) GetSnippet(ctx context.Context, id string, snip *snippet.Snippet) error {
	exists, err := redis.Bool(s.pool.Get().Do("EXISTS", id))

	if err != nil {
		return err
	}

	if !exists {
		return s.ErrNoSuchEntity()
	}

	v, err := redis.Bytes(s.pool.Get().Do("GET", id))
	*snip = *snippet.Decode(v)
	return err
}

func (s redisStoreImp) ErrNoSuchEntity() error {
	return fmt.Errorf("Not found")
}
