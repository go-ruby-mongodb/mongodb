// Copyright (c) the go-ruby-mongodb/mongodb authors
//
// SPDX-License-Identifier: BSD-3-Clause

package mongodb

import (
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// cmdErr is an {ok:0, errmsg, code} reply that makes the driver raise a
// CommandError, which this package maps to Mongo::Error::OperationFailure.
func cmdErr() bson.D {
	return bson.D{
		{Key: "ok", Value: 0},
		{Key: "errmsg", Value: "boom"},
		{Key: "code", Value: int32(26)},
	}
}

// writeErr is an {ok:1, writeErrors:[...]} reply, a per-document write failure
// the driver surfaces as a WriteException.
func writeErr(code int32) bson.D {
	return bson.D{
		{Key: "ok", Value: 1},
		{Key: "n", Value: 0},
		{Key: "writeErrors", Value: bson.A{
			bson.D{{Key: "index", Value: int32(0)}, {Key: "code", Value: code}, {Key: "errmsg", Value: "write failed"}},
		}},
	}
}

func wantErrClass(t *testing.T, err error, class ErrorClass) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error")
	}
	var e *Error
	if !asError(err, &e) {
		t.Fatalf("error is not *mongodb.Error: %T", err)
	}
	if e.Class != class {
		t.Fatalf("class = %v, want %v", e.Class, class)
	}
}

func TestInsertOneError(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(writeErr(121))
	_, err := coll(cli).InsertOne(Doc("x", int32(1)))
	wantErrClass(t, err, ErrOperationFailure)
}

func TestInsertManyDuplicateKey(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(writeErr(DuplicateKeyCode))
	_, err := coll(cli).InsertMany([]Document{Doc("_id", int32(1)), Doc("_id", int32(1))})
	wantErrClass(t, err, ErrBulkWriteError)
	var e *Error
	asError(err, &e)
	if !e.IsDuplicateKey() {
		t.Fatal("expected duplicate-key")
	}
}

func TestFindError(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cmdErr())
	_, err := coll(cli).Find(Doc(), nil)
	wantErrClass(t, err, ErrOperationFailure)
}

func TestFindOneError(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cmdErr())
	_, err := coll(cli).FindOne(Doc(), nil)
	wantErrClass(t, err, ErrOperationFailure)
}

func TestUpdateOneError(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cmdErr())
	_, err := coll(cli).UpdateOne(Doc(), Doc("$set", Doc("a", 1)), nil)
	wantErrClass(t, err, ErrOperationFailure)
}

func TestReplaceOneError(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cmdErr())
	_, err := coll(cli).ReplaceOne(Doc(), Doc("a", 1), nil)
	wantErrClass(t, err, ErrOperationFailure)
}

func TestDeleteOneError(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cmdErr())
	_, err := coll(cli).DeleteOne(Doc())
	wantErrClass(t, err, ErrOperationFailure)
}

func TestCountError(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cmdErr())
	_, err := coll(cli).CountDocuments(Doc(), nil)
	wantErrClass(t, err, ErrOperationFailure)
}

func TestAggregateError(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cmdErr())
	_, err := coll(cli).Aggregate([]Document{Doc("$match", Doc())})
	wantErrClass(t, err, ErrOperationFailure)
}

func TestDistinctErrPath(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cmdErr())
	_, err := coll(cli).Distinct("f", Doc())
	wantErrClass(t, err, ErrOperationFailure)
}

func TestDistinctDecodeErrPath(t *testing.T) {
	cli, md := newTestClient(t)
	// ok command, but "values" is not an array: Decode into []any fails.
	md.AddResponses(bson.D{{Key: "ok", Value: 1}, {Key: "values", Value: int32(5)}})
	_, err := coll(cli).Distinct("f", Doc())
	if err == nil {
		t.Fatal("expected decode error")
	}
}

func TestCreateIndexError(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cmdErr())
	_, err := coll(cli).CreateIndex(Doc("a", int32(1)), nil)
	wantErrClass(t, err, ErrOperationFailure)
}

func TestIndexesError(t *testing.T) {
	cli, md := newTestClient(t)
	// Use Unauthorized (13); the driver special-cases NamespaceNotFound (26) on
	// listIndexes into an empty cursor rather than an error.
	md.AddResponses(bson.D{
		{Key: "ok", Value: 0},
		{Key: "errmsg", Value: "not authorized"},
		{Key: "code", Value: int32(13)},
	})
	_, err := coll(cli).Indexes()
	wantErrClass(t, err, ErrOperationFailure)
}

// TestCursorGetMoreError drives a cursor whose id is non-zero so the second
// Next issues a getMore; with no queued reply the driver errors, exercising the
// cursor error path and its propagation through ToArray.
func TestCursorGetMoreError(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cursorReply(42, "firstBatch", bson.D{{Key: "x", Value: int32(1)}}))
	cur, err := coll(cli).Find(Doc(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cur.ToArray(); err == nil {
		t.Fatal("expected a cursor error on getMore")
	}
}

func TestUpdateOneUpsert(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok(
		bson.E{Key: "n", Value: 1},
		bson.E{Key: "nModified", Value: 0},
		bson.E{Key: "upserted", Value: bson.A{bson.D{{Key: "index", Value: int32(0)}, {Key: "_id", Value: int32(9)}}}},
	))
	res, err := coll(cli).UpdateOne(Doc("_id", int32(9)), Doc("$set", Doc("v", 1)), &UpdateOptions{Upsert: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.UpsertedCount != 1 || res.UpsertedID != int32(9) {
		t.Fatalf("res = %+v", res)
	}
}

// TestCursorDecodeFault injects a decode fault to cover the otherwise-unreachable
// decode-error branch of Cursor.Next.
func TestCursorDecodeFault(t *testing.T) {
	orig := cursorDecode
	cursorDecode = func(*mongoCursor, any) error { return errFault }
	defer func() { cursorDecode = orig }()

	cli, md := newTestClient(t)
	md.AddResponses(cursorReply(0, "firstBatch", bson.D{{Key: "x", Value: int32(1)}}))
	cur, err := coll(cli).Find(Doc(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cur.ToArray(); err == nil {
		t.Fatal("expected a decode fault")
	}
}

// TestDistinctDecodeFault injects a decode fault to cover the otherwise-
// unreachable decode-error branch of Distinct.
func TestDistinctDecodeFault(t *testing.T) {
	orig := distinctDecode
	distinctDecode = func(*mongoDistinctResult, any) error { return errFault }
	defer func() { distinctDecode = orig }()

	cli, md := newTestClient(t)
	md.AddResponses(ok(bson.E{Key: "values", Value: bson.A{"a"}}))
	if _, err := coll(cli).Distinct("f", Doc()); err == nil {
		t.Fatal("expected a decode fault")
	}
}

func TestPing(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok())
	if err := cli.Ping(); err != nil {
		t.Fatal(err)
	}
}

func TestDatabaseDrop(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok())
	if err := cli.Database("").Drop(); err != nil {
		t.Fatal(err)
	}
}

func TestNewClientNoAppName(t *testing.T) {
	md := newMockForClient(t)
	cli, err := NewClient("mongodb://localhost:27017", withDeployment(md))
	if err != nil {
		t.Fatalf("NewClient without app name: %v", err)
	}
	defer cli.Close()
	if cli.Database("").Name() != "" {
		t.Fatal("empty default database expected")
	}
}

func TestNewClientBadURI(t *testing.T) {
	if _, err := NewClient("mongodb://%gh&%ij"); err == nil {
		t.Fatal("expected an error for an invalid URI")
	}
}
