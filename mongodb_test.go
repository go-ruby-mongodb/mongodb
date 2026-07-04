// Copyright (c) the go-ruby-mongodb/mongodb authors
//
// SPDX-License-Identifier: BSD-3-Clause

package mongodb

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/drivertest"
)

// Aliases and a sentinel used by the fault-injection seam tests.
type (
	mongoCursor         = mongo.Cursor
	mongoDistinctResult = mongo.DistinctResult
)

var errFault = errors.New("injected fault")

// newTestClient builds a Client backed by a canned MockDeployment, so every
// operation runs the real driver in-process with no live mongod. The returned
// deployment is loaded with the server replies each test needs via AddResponses.
func newTestClient(t *testing.T) (*Client, *drivertest.MockDeployment) {
	t.Helper()
	md := drivertest.NewMockDeployment()
	cli, err := NewClient("mongodb://localhost:27017",
		WithDatabase("testdb"), AppName("go-ruby-mongodb"), withDeployment(md))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = cli.Close() })
	return cli, md
}

// ok wraps a normal command reply.
func ok(extra ...bson.E) bson.D {
	d := bson.D{{Key: "ok", Value: 1}}
	return append(d, extra...)
}

// cursorReply builds an {ok:1, cursor:{id, ns, <batchField>:docs}} reply.
func cursorReply(id int64, batchField string, docs ...bson.D) bson.D {
	arr := bson.A{}
	for _, d := range docs {
		arr = append(arr, d)
	}
	return bson.D{
		{Key: "ok", Value: 1},
		{Key: "cursor", Value: bson.D{
			{Key: "id", Value: id},
			{Key: "ns", Value: "testdb.things"},
			{Key: batchField, Value: arr},
		}},
	}
}

func coll(cli *Client) *Collection { return cli.Database("").Collection("things") }

// newMockForClient returns a bare MockDeployment for tests that construct their
// own Client.
func newMockForClient(t *testing.T) *drivertest.MockDeployment {
	t.Helper()
	return drivertest.NewMockDeployment()
}

func TestInsertOne(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok(bson.E{Key: "n", Value: 1}))
	res, err := coll(cli).InsertOne(Doc("name", "Ada", "age", int32(36)))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := res.InsertedID.(ObjectId); !ok {
		t.Fatalf("InsertedID = %T, want ObjectId", res.InsertedID)
	}
}

func TestInsertOneWithID(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok(bson.E{Key: "n", Value: 1}))
	res, err := coll(cli).InsertOne(Doc("_id", int32(7), "name", "Ada"))
	if err != nil {
		t.Fatal(err)
	}
	if res.InsertedID != int32(7) {
		t.Fatalf("InsertedID = %v, want 7", res.InsertedID)
	}
}

func TestInsertMany(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok(bson.E{Key: "n", Value: 2}))
	res, err := coll(cli).InsertMany([]Document{
		Doc("i", int32(1)),
		Doc("i", int32(2)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.InsertedIDs) != 2 {
		t.Fatalf("InsertedIDs = %v", res.InsertedIDs)
	}
}

func TestFindToArray(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cursorReply(0, "firstBatch",
		bson.D{{Key: "_id", Value: int32(1)}, {Key: "name", Value: "Ada"}},
		bson.D{{Key: "_id", Value: int32(2)}, {Key: "name", Value: "Bob"}},
	))
	limit := int64(10)
	skip := int64(0)
	bs := int32(100)
	cur, err := coll(cli).Find(Doc("age", Doc("$gte", int32(18))), &FindOptions{
		Sort:       Doc("name", int32(1)),
		Projection: Doc("name", int32(1)),
		Limit:      &limit,
		Skip:       &skip,
		BatchSize:  &bs,
	})
	if err != nil {
		t.Fatal(err)
	}
	docs, err := cur.ToArray()
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Fatalf("got %d docs", len(docs))
	}
	if v, _ := docs[0].Get("name"); v != "Ada" {
		t.Fatalf("docs[0].name = %v", v)
	}
}

func TestFindNilOptions(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cursorReply(0, "firstBatch"))
	cur, err := coll(cli).Find(Doc(), nil)
	if err != nil {
		t.Fatal(err)
	}
	docs, err := cur.ToArray()
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Fatalf("expected empty, got %d", len(docs))
	}
}

func TestFindEach(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cursorReply(0, "firstBatch",
		bson.D{{Key: "n", Value: int32(1)}},
		bson.D{{Key: "n", Value: int32(2)}},
	))
	cur, err := coll(cli).Find(Doc(), nil)
	if err != nil {
		t.Fatal(err)
	}
	sum := int32(0)
	err = cur.Each(func(d Document) error {
		v, _ := d.Get("n")
		sum += v.(int32)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if sum != 3 {
		t.Fatalf("sum = %d", sum)
	}
}

func TestFindEachStopsOnError(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cursorReply(0, "firstBatch",
		bson.D{{Key: "n", Value: int32(1)}},
		bson.D{{Key: "n", Value: int32(2)}},
	))
	cur, err := coll(cli).Find(Doc(), nil)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("stop")
	err = cur.Each(func(d Document) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("err = %v, want sentinel", err)
	}
}

func TestCursorNext(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cursorReply(0, "firstBatch",
		bson.D{{Key: "x", Value: int32(1)}},
	))
	cur, err := coll(cli).Find(Doc(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cur.Close()
	doc, more, err := cur.Next(context.Background())
	if err != nil || !more {
		t.Fatalf("first Next: more=%v err=%v", more, err)
	}
	if v, _ := doc.Get("x"); v != int32(1) {
		t.Fatalf("x = %v", v)
	}
	_, more, err = cur.Next(context.Background())
	if err != nil || more {
		t.Fatalf("second Next: more=%v err=%v", more, err)
	}
}

func TestFindOne(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cursorReply(0, "firstBatch",
		bson.D{{Key: "_id", Value: int32(1)}, {Key: "name", Value: "Ada"}},
	))
	skip := int64(0)
	doc, err := coll(cli).FindOne(Doc("_id", int32(1)), &FindOptions{
		Sort:       Doc("name", int32(1)),
		Projection: Doc("name", int32(1)),
		Skip:       &skip,
	})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := doc.Get("name"); v != "Ada" {
		t.Fatalf("name = %v", v)
	}
}

func TestFindOneNoMatch(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cursorReply(0, "firstBatch"))
	doc, err := coll(cli).FindOne(Doc("_id", int32(999)), nil)
	if err != nil {
		t.Fatal(err)
	}
	if doc != nil {
		t.Fatalf("expected nil doc, got %v", doc)
	}
}

func TestUpdateOne(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok(bson.E{Key: "n", Value: 1}, bson.E{Key: "nModified", Value: 1}))
	res, err := coll(cli).UpdateOne(Doc("_id", int32(1)), Doc("$set", Doc("name", "Eve")), nil)
	if err != nil {
		t.Fatal(err)
	}
	if res.MatchedCount != 1 || res.ModifiedCount != 1 {
		t.Fatalf("res = %+v", res)
	}
}

func TestUpdateManyUpsert(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok(
		bson.E{Key: "n", Value: 1},
		bson.E{Key: "nModified", Value: 0},
		bson.E{Key: "upserted", Value: bson.A{bson.D{{Key: "index", Value: int32(0)}, {Key: "_id", Value: int32(42)}}}},
	))
	res, err := coll(cli).UpdateMany(Doc("_id", int32(42)), Doc("$set", Doc("v", int32(1))), &UpdateOptions{Upsert: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.UpsertedCount != 1 || res.UpsertedID != int32(42) {
		t.Fatalf("res = %+v", res)
	}
}

func TestReplaceOne(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok(bson.E{Key: "n", Value: 1}, bson.E{Key: "nModified", Value: 1}))
	res, err := coll(cli).ReplaceOne(Doc("_id", int32(1)), Doc("name", "Zed"), &UpdateOptions{Upsert: false})
	if err != nil {
		t.Fatal(err)
	}
	if res.ModifiedCount != 1 {
		t.Fatalf("res = %+v", res)
	}
}

func TestDeleteOneMany(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok(bson.E{Key: "n", Value: 1}))
	if r, err := coll(cli).DeleteOne(Doc("_id", int32(1))); err != nil || r.DeletedCount != 1 {
		t.Fatalf("DeleteOne r=%+v err=%v", r, err)
	}
	md.AddResponses(ok(bson.E{Key: "n", Value: 3}))
	if r, err := coll(cli).DeleteMany(Doc("stale", true)); err != nil || r.DeletedCount != 3 {
		t.Fatalf("DeleteMany r=%+v err=%v", r, err)
	}
}

func TestCountDocuments(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cursorReply(0, "firstBatch", bson.D{{Key: "n", Value: int32(5)}}))
	limit := int64(100)
	skip := int64(0)
	n, err := coll(cli).CountDocuments(Doc("active", true), &CountOptions{Limit: &limit, Skip: &skip})
	if err != nil {
		t.Fatal(err)
	}
	if n != 5 {
		t.Fatalf("n = %d", n)
	}
}

func TestAggregate(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cursorReply(0, "firstBatch",
		bson.D{{Key: "_id", Value: "a"}, {Key: "total", Value: int32(10)}},
	))
	cur, err := coll(cli).Aggregate([]Document{
		Doc("$match", Doc("active", true)),
		Doc("$group", Doc("_id", "$cat", "total", Doc("$sum", int32(1)))),
	})
	if err != nil {
		t.Fatal(err)
	}
	docs, err := cur.ToArray()
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d", len(docs))
	}
}

func TestDistinct(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok(bson.E{Key: "values", Value: bson.A{"a", "b", "c"}}))
	vals, err := coll(cli).Distinct("cat", Doc("active", true))
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 3 || vals[0] != "a" {
		t.Fatalf("vals = %v", vals)
	}
}

func TestCreateIndex(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok())
	name, err := coll(cli).CreateIndex(Doc("name", int32(1)), &IndexOptions{Unique: true})
	if err != nil {
		t.Fatal(err)
	}
	if name != "name_1" {
		t.Fatalf("name = %q", name)
	}
}

func TestCreateIndexNamed(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok())
	name, err := coll(cli).CreateIndex(Doc("name", int32(1)), &IndexOptions{Name: "by_name"})
	if err != nil {
		t.Fatal(err)
	}
	if name != "by_name" {
		t.Fatalf("name = %q", name)
	}
}

func TestCreateIndexNoOpts(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(ok())
	if _, err := coll(cli).CreateIndex(Doc("a", int32(1)), nil); err != nil {
		t.Fatal(err)
	}
}

func TestIndexes(t *testing.T) {
	cli, md := newTestClient(t)
	md.AddResponses(cursorReply(0, "firstBatch",
		bson.D{{Key: "name", Value: "_id_"}, {Key: "key", Value: bson.D{{Key: "_id", Value: int32(1)}}}},
	))
	idx, err := coll(cli).Indexes()
	if err != nil {
		t.Fatal(err)
	}
	if len(idx) != 1 {
		t.Fatalf("got %d indexes", len(idx))
	}
}

func TestClientAccessors(t *testing.T) {
	cli, _ := newTestClient(t)
	if cli.Database("").Name() != "testdb" {
		t.Fatalf("default db = %q", cli.Database("").Name())
	}
	if cli.Database("other").Name() != "other" {
		t.Fatal("named db")
	}
	if cli.Collection("things").Name() != "things" {
		t.Fatal("client collection")
	}
}
