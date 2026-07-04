// Copyright (c) the go-ruby-mongodb/mongodb authors
//
// SPDX-License-Identifier: BSD-3-Clause

package mongodb

import (
	"os"
	"testing"
	"time"
)

// liveURI returns the MONGODB_URI connection string, or skips the test. The
// deterministic suite above reaches 100% coverage with no server (it drives the
// real driver against a canned MockDeployment), so this round-trip oracle is
// opt-in: CI, qemu cross-arch, and Windows lanes leave MONGODB_URI unset and
// skip it. Point it at a throwaway mongod to exercise a genuine end-to-end path.
func liveURI(t *testing.T) string {
	t.Helper()
	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		t.Skip("MONGODB_URI unset; skipping live-mongod round-trip oracle")
	}
	return uri
}

// TestLiveRoundTrip inserts, queries, updates, counts, and deletes against a
// real MongoDB, confirming the Ruby surface drives an actual server end-to-end
// and that BSON encoded here decodes back to the same values off the wire.
func TestLiveRoundTrip(t *testing.T) {
	uri := liveURI(t)
	cli, err := NewClient(uri, WithDatabase("go_ruby_mongodb_test"))
	if err != nil {
		t.Fatal(err)
	}
	defer cli.Close()
	if err := cli.Ping(); err != nil {
		t.Fatalf("ping: %v", err)
	}

	c := cli.Collection("roundtrip")
	if _, err := c.DeleteMany(Doc()); err != nil {
		t.Fatal(err)
	}

	oid := NewObjectId()
	when := NewDateTime(time.Now().Truncate(time.Millisecond).UTC())
	if _, err := c.InsertOne(Doc(
		"_id", oid,
		"name", "Ada",
		"age", Int32(36),
		"tags", []any{"math", "logic"},
		"meta", Doc("born", when),
	)); err != nil {
		t.Fatal(err)
	}

	got, err := c.FindOne(Doc("_id", oid), nil)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := got.Get("name"); v != "Ada" {
		t.Fatalf("name = %v", v)
	}
	if v, _ := got.Get("age"); v != int32(36) {
		t.Fatalf("age = %v (%T)", v, v)
	}
	sub, _ := got.Get("meta")
	if _, ok := sub.(Document); !ok {
		t.Fatalf("meta = %T, want Document", sub)
	}

	upd, err := c.UpdateOne(Doc("_id", oid), Doc("$set", Doc("age", Int32(37))), nil)
	if err != nil || upd.ModifiedCount != 1 {
		t.Fatalf("update: %+v err=%v", upd, err)
	}

	n, err := c.CountDocuments(Doc("age", Int32(37)), nil)
	if err != nil || n != 1 {
		t.Fatalf("count = %d err=%v", n, err)
	}

	tags, err := c.Distinct("tags", Doc())
	if err != nil || len(tags) != 2 {
		t.Fatalf("distinct = %v err=%v", tags, err)
	}

	del, err := c.DeleteMany(Doc())
	if err != nil || del.DeletedCount != 1 {
		t.Fatalf("delete: %+v err=%v", del, err)
	}
}
