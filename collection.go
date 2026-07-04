// Copyright (c) the go-ruby-mongodb/mongodb authors
//
// SPDX-License-Identifier: BSD-3-Clause

package mongodb

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Collection is Mongo::Collection: a handle to a named collection. Its methods
// mirror the gem's CRUD, aggregation, and index surface.
type Collection struct {
	inner *mongo.Collection
}

// ctx is the context used for a single operation. The mongo gem exposes no
// explicit context, so operations run on a fresh background context.
func ctx() context.Context { return context.Background() }

// Name returns the collection name, the gem's Mongo::Collection#name.
func (c *Collection) Name() string { return c.inner.Name() }

// FindOptions are the keyword options of Mongo::Collection#find. A nil
// *FindOptions means no options, like calling find with no keywords.
type FindOptions struct {
	Sort       Document // the `sort:` document
	Projection Document // the `projection:` document
	Limit      *int64   // the `limit:` count
	Skip       *int64   // the `skip:` count
	BatchSize  *int32   // the `batch_size:` hint
}

// build converts FindOptions into the driver's options list.
func (o *FindOptions) build() *options.FindOptionsBuilder {
	b := options.Find()
	if o == nil {
		return b
	}
	if o.Sort != nil {
		b.SetSort(o.Sort.toD())
	}
	if o.Projection != nil {
		b.SetProjection(o.Projection.toD())
	}
	if o.Limit != nil {
		b.SetLimit(*o.Limit)
	}
	if o.Skip != nil {
		b.SetSkip(*o.Skip)
	}
	if o.BatchSize != nil {
		b.SetBatchSize(*o.BatchSize)
	}
	return b
}

// UpdateOptions are the keyword options shared by update and replace operations.
type UpdateOptions struct {
	Upsert bool // the `upsert:` flag
}

// IndexOptions are the keyword options of Mongo::Collection#create_index.
type IndexOptions struct {
	Name   string // the `name:` override
	Unique bool   // the `unique:` flag
}

// CountOptions are the keyword options of Mongo::Collection#count_documents.
type CountOptions struct {
	Limit *int64 // the `limit:` cap
	Skip  *int64 // the `skip:` offset
}

// InsertOne inserts a single document, the gem's Mongo::Collection#insert_one.
func (c *Collection) InsertOne(doc Document) (*InsertOneResult, error) {
	res, err := c.inner.InsertOne(ctx(), doc.toD())
	if err != nil {
		return nil, mapError(err)
	}
	return &InsertOneResult{InsertedID: toRuby(res.InsertedID)}, nil
}

// InsertMany inserts many documents, the gem's Mongo::Collection#insert_many.
func (c *Collection) InsertMany(docs []Document) (*InsertManyResult, error) {
	payload := make([]any, len(docs))
	for i := range docs {
		payload[i] = docs[i].toD()
	}
	res, err := c.inner.InsertMany(ctx(), payload)
	if err != nil {
		return nil, mapError(err)
	}
	out := &InsertManyResult{InsertedIDs: make([]any, len(res.InsertedIDs))}
	for i, id := range res.InsertedIDs {
		out.InsertedIDs[i] = toRuby(id)
	}
	return out, nil
}

// Find runs a query, the gem's Mongo::Collection#find. It returns a Cursor, an
// Enumerable of ordered Documents.
func (c *Collection) Find(filter Document, opts *FindOptions) (*Cursor, error) {
	cur, err := c.inner.Find(ctx(), filter.toD(), opts.build())
	if err != nil {
		return nil, mapError(err)
	}
	return &Cursor{inner: cur}, nil
}

// FindOne returns the first matching document, the gem's
// Mongo::Collection#find(...).first. It returns a nil Document (and nil error)
// when nothing matches.
func (c *Collection) FindOne(filter Document, opts *FindOptions) (Document, error) {
	b := options.FindOne()
	if opts != nil {
		if opts.Sort != nil {
			b.SetSort(opts.Sort.toD())
		}
		if opts.Projection != nil {
			b.SetProjection(opts.Projection.toD())
		}
		if opts.Skip != nil {
			b.SetSkip(*opts.Skip)
		}
	}
	var raw bson.D
	err := c.inner.FindOne(ctx(), filter.toD(), b).Decode(&raw)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, mapError(err)
	}
	return fromD(raw), nil
}

// UpdateOne applies an update to the first matching document, the gem's
// Mongo::Collection#update_one.
func (c *Collection) UpdateOne(filter, update Document, opts *UpdateOptions) (*UpdateResult, error) {
	res, err := c.inner.UpdateOne(ctx(), filter.toD(), update.toD(), updateOpts(opts))
	return updateResult(res, err)
}

// UpdateMany applies an update to every matching document, the gem's
// Mongo::Collection#update_many.
func (c *Collection) UpdateMany(filter, update Document, opts *UpdateOptions) (*UpdateResult, error) {
	b := options.UpdateMany()
	if opts != nil {
		b.SetUpsert(opts.Upsert)
	}
	res, err := c.inner.UpdateMany(ctx(), filter.toD(), update.toD(), b)
	return updateResult(res, err)
}

// ReplaceOne replaces the first matching document wholesale, the gem's
// Mongo::Collection#replace_one.
func (c *Collection) ReplaceOne(filter, replacement Document, opts *UpdateOptions) (*UpdateResult, error) {
	b := options.Replace()
	if opts != nil {
		b.SetUpsert(opts.Upsert)
	}
	res, err := c.inner.ReplaceOne(ctx(), filter.toD(), replacement.toD(), b)
	return updateResult(res, err)
}

// updateOpts builds the driver update options from UpdateOptions.
func updateOpts(opts *UpdateOptions) *options.UpdateOneOptionsBuilder {
	b := options.UpdateOne()
	if opts != nil {
		b.SetUpsert(opts.Upsert)
	}
	return b
}

// updateResult maps a driver *mongo.UpdateResult to the gem's counts.
func updateResult(res *mongo.UpdateResult, err error) (*UpdateResult, error) {
	if err != nil {
		return nil, mapError(err)
	}
	return &UpdateResult{
		MatchedCount:  res.MatchedCount,
		ModifiedCount: res.ModifiedCount,
		UpsertedCount: res.UpsertedCount,
		UpsertedID:    toRuby(res.UpsertedID),
	}, nil
}

// DeleteOne removes the first matching document, the gem's
// Mongo::Collection#delete_one.
func (c *Collection) DeleteOne(filter Document) (*DeleteResult, error) {
	res, err := c.inner.DeleteOne(ctx(), filter.toD())
	return deleteResult(res, err)
}

// DeleteMany removes every matching document, the gem's
// Mongo::Collection#delete_many.
func (c *Collection) DeleteMany(filter Document) (*DeleteResult, error) {
	res, err := c.inner.DeleteMany(ctx(), filter.toD())
	return deleteResult(res, err)
}

// deleteResult maps a driver *mongo.DeleteResult to the gem's count.
func deleteResult(res *mongo.DeleteResult, err error) (*DeleteResult, error) {
	if err != nil {
		return nil, mapError(err)
	}
	return &DeleteResult{DeletedCount: res.DeletedCount}, nil
}

// CountDocuments counts documents matching a filter, the gem's
// Mongo::Collection#count_documents.
func (c *Collection) CountDocuments(filter Document, opts *CountOptions) (int64, error) {
	b := options.Count()
	if opts != nil {
		if opts.Limit != nil {
			b.SetLimit(*opts.Limit)
		}
		if opts.Skip != nil {
			b.SetSkip(*opts.Skip)
		}
	}
	n, err := c.inner.CountDocuments(ctx(), filter.toD(), b)
	if err != nil {
		return 0, mapError(err)
	}
	return n, nil
}

// Aggregate runs an aggregation pipeline, the gem's Mongo::Collection#aggregate.
// It returns a Cursor over the stage output.
func (c *Collection) Aggregate(pipeline []Document) (*Cursor, error) {
	stages := make(bson.A, len(pipeline))
	for i := range pipeline {
		stages[i] = pipeline[i].toD()
	}
	cur, err := c.inner.Aggregate(ctx(), stages)
	if err != nil {
		return nil, mapError(err)
	}
	return &Cursor{inner: cur}, nil
}

// distinctDecode is the seam used to decode a distinct result once its command
// error (Err) has been checked. A server array always decodes into []any, so the
// decode-error branch is otherwise unreachable; tests override this to inject a
// decode fault.
var distinctDecode = (*mongo.DistinctResult).Decode

// Distinct returns the distinct values of a field, the gem's
// Mongo::Collection#distinct. The values are mapped to the Ruby surface.
func (c *Collection) Distinct(field string, filter Document) ([]any, error) {
	res := c.inner.Distinct(ctx(), field, filter.toD())
	if err := res.Err(); err != nil {
		return nil, mapError(err)
	}
	var vals []any
	if err := distinctDecode(res, &vals); err != nil {
		return nil, mapError(err)
	}
	out := make([]any, len(vals))
	for i := range vals {
		out[i] = toRuby(vals[i])
	}
	return out, nil
}

// CreateIndex creates an index over the given key spec, the gem's
// Mongo::Collection#create_index. It returns the created index name.
func (c *Collection) CreateIndex(keys Document, opts *IndexOptions) (string, error) {
	model := mongo.IndexModel{Keys: keys.toD()}
	if opts != nil {
		io := options.Index()
		if opts.Name != "" {
			io.SetName(opts.Name)
		}
		if opts.Unique {
			io.SetUnique(true)
		}
		model.Options = io
	}
	name, err := c.inner.Indexes().CreateOne(ctx(), model)
	if err != nil {
		return "", mapError(err)
	}
	return name, nil
}

// Indexes lists the collection's indexes, the gem's Mongo::Collection#indexes.
func (c *Collection) Indexes() ([]Document, error) {
	cur, err := c.inner.Indexes().List(ctx())
	if err != nil {
		return nil, mapError(err)
	}
	return (&Cursor{inner: cur}).ToArray()
}
