// Copyright 2026 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"

	"cloud.google.com/go/datastore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const snippetKey = "Snippet"

type entity struct {
	Body []byte `datastore:",noindex"` // golang.org/issues/23253
}

func key(id string) *datastore.Key {
	return datastore.NameKey(snippetKey, id, nil)
}

// datastoreBackend is a backend that stores snippets as Snippet entities.
type datastoreBackend struct {
	client *datastore.Client
}

func (d datastoreBackend) get(ctx context.Context, id string) ([]byte, error) {
	var e entity
	err := d.client.Get(ctx, key(id), &e)
	if errors.Is(err, datastore.ErrNoSuchEntity) {
		return nil, errNotFound
	}
	if err != nil {
		return nil, err
	}
	return e.Body, nil
}

func (d datastoreBackend) insert(ctx context.Context, id string, body []byte) error {
	_, err := d.client.Mutate(ctx, datastore.NewInsert(key(id), &entity{Body: body}))
	if status.Code(err) == codes.AlreadyExists {
		// IDs are derived from the body, so this is almost always the
		// same snippet being shared again. Either way, never overwrite.
		return nil
	}
	return err
}
