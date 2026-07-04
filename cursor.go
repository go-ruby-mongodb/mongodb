// Copyright (c) the go-ruby-mongodb/mongodb authors
//
// SPDX-License-Identifier: BSD-3-Clause

package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Cursor is Mongo::Cursor: a lazily-iterated stream of query results, exposed
// the way the gem's cursor is Enumerable over BSON::Documents. Each yielded
// document is an ordered Document.
type Cursor struct {
	inner *mongo.Cursor
}

// cursorDecode is the seam used to decode the current document. A valid document
// read from the wire always decodes into a bson.D, so the decode-error branch is
// otherwise unreachable; tests override this to inject a decode fault.
var cursorDecode = (*mongo.Cursor).Decode

// Next advances to the next document, the gem's cursor iteration. It returns the
// document and true, or a nil document and false when the cursor is exhausted.
// A server or decode error is returned as a *mongodb.Error.
func (c *Cursor) Next(ctx context.Context) (Document, bool, error) {
	if !c.inner.Next(ctx) {
		if err := c.inner.Err(); err != nil {
			return nil, false, mapError(err)
		}
		return nil, false, nil
	}
	var raw bson.D
	if err := cursorDecode(c.inner, &raw); err != nil {
		return nil, false, mapError(err)
	}
	return fromD(raw), true, nil
}

// Each iterates every remaining document, the gem's Mongo::Cursor#each. Iteration
// stops and the error is returned if fn returns one; the cursor is always closed.
func (c *Cursor) Each(fn func(Document) error) error {
	defer c.Close()
	ctx := context.Background()
	for {
		doc, ok, err := c.Next(ctx)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if err := fn(doc); err != nil {
			return err
		}
	}
}

// ToArray drains the cursor into a slice of ordered Documents, the gem's
// Mongo::Cursor#to_a. The cursor is closed on return.
func (c *Cursor) ToArray() ([]Document, error) {
	out := []Document{}
	err := c.Each(func(d Document) error {
		out = append(out, d)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Close releases the cursor's server-side resources, the gem's
// Mongo::Cursor#close. It is idempotent and safe to defer.
func (c *Cursor) Close() error {
	return mapError(c.inner.Close(context.Background()))
}
