// Copyright (c) the go-ruby-mongodb/mongodb authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package mongodb is a pure-Go (CGO=0), MRI-faithful reimplementation of the
// Ruby mongo gem and the core of the bson gem.
//
// Upstream, the mongo gem drives MongoDB over a hand-written wire protocol and
// the bson gem's fast path is a C extension. This package instead binds
// go.mongodb.org/mongo-driver/v2 — MongoDB's own pure-Go driver and BSON codec —
// and exposes the gem's Ruby surface on top, so the whole stack links
// statically with CGO_ENABLED=0 on every 64-bit target the go-* ecosystem
// supports (amd64, arm64, riscv64, loong64, ppc64le, s390x). The wire protocol
// and BSON encoding are the driver's; this package is the Ruby-shaped facade,
// the result→Ruby value mapping, and the Mongo::Error taxonomy.
//
// # The mongo gem surface
//
//	cli, _ := mongodb.NewClient("mongodb://localhost:27017", mongodb.WithDatabase("test"))
//	db   := cli.Database("test")          // Mongo::Client#database
//	coll := db.Collection("people")       // Database#[] / Client#[]
//	coll.InsertOne(mongodb.Doc("name", "Ada", "age", int32(36)))
//	cur, _ := coll.Find(mongodb.Doc("age", mongodb.Doc("$gte", int32(18))), nil)
//	docs, _ := cur.ToArray()              // []Document, ordered, string-keyed
//
// Collection mirrors the gem: InsertOne/InsertMany, Find/FindOne,
// UpdateOne/UpdateMany/ReplaceOne, DeleteOne/DeleteMany, CountDocuments,
// Aggregate, Distinct, CreateIndex/Indexes.
//
// # The bson gem surface
//
// Document is BSON::Document (an ordered, string-keyed document). ObjectId,
// Binary, Timestamp, Decimal128, Int32, Int64, Double and Regexp mirror the
// BSON:: value classes; nil, time.Time, and slices map to null, UTC datetime,
// and array. Document.ToBSON / DocumentFromBSON are the gem's #to_bson /
// .from_bson byte round-trip, encoded byte-for-byte by the driver's canonical
// BSON codec.
//
// # Determinism and the connection seam
//
// Everything a rbgo binding needs to be correct — BSON encode/decode, ObjectId,
// query/pipeline construction, result→Ruby mapping, and the error taxonomy — is
// deterministic and needs no server. The deterministic test suite drives real
// driver operations in-process against a canned MockDeployment, so it reaches
// 100% coverage with no live mongod. An optional round-trip suite against a real
// server is gated behind the MONGODB_URI environment variable and skips when it
// is unset (so CI, qemu, and Windows lanes stay server-free).
package mongodb
