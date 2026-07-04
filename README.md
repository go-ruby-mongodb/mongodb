<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-mongodb/brand/main/social/go-ruby-mongodb-mongodb.png" alt="go-ruby-mongodb/mongodb" width="720"></p>

# mongodb — go-ruby-mongodb

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-mongodb.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo), MRI-faithful reimplementation of the Ruby
[`mongo`](https://github.com/mongodb/mongo-ruby-driver) gem and the core of the
[`bson`](https://github.com/mongodb/bson-ruby) gem.**
Upstream, the `mongo` gem speaks a hand-written wire protocol and the `bson`
gem's fast path is a C extension. This module binds
[`go.mongodb.org/mongo-driver/v2`](https://pkg.go.dev/go.mongodb.org/mongo-driver/v2)
— MongoDB's own pure-Go driver and BSON codec — and exposes the gems'
`Mongo::Client` / `Mongo::Collection` / `BSON::*` surface on top. The result
links statically with `CGO_ENABLED=0` on every 64-bit target the go-\* ecosystem
supports.

It completes the database set alongside
[go-ruby-sqlite3](https://github.com/go-ruby-sqlite3/sqlite3),
[go-ruby-pg](https://github.com/go-ruby-pg/pg), and
[go-ruby-mysql](https://github.com/go-ruby-mysql/mysql), and is a **standalone,
reusable** module — the MongoDB backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby).

> **What it is — and isn't.** The wire protocol and BSON encoding belong to the
> driver; this library does not reinvent them. What lives here is the
> Ruby-shaped facade, the query/pipeline construction, the result→Ruby value
> mapping, the ordered `BSON::Document` with byte-exact `#to_bson`/`.from_bson`,
> and the `Mongo::Error` taxonomy — all deterministic, all the host binding
> needs to be correct.

## Features

Faithful port of the gems' surface, validated against the real driver's BSON
codec on every platform:

- **`Mongo::Client`** — `NewClient(uri, options...)`, `#database` / `#[]`,
  `Ping`, `Close`; `WithDatabase` (`database:`) and `AppName` options.
- **`Mongo::Database`** — `#name`, `#collection` / `#[]`, `#drop`.
- **`Mongo::Collection`** — `insert_one` / `insert_many`, `find` / `find_one`,
  `update_one` / `update_many` / `replace_one`, `delete_one` / `delete_many`,
  `count_documents`, `aggregate`, `distinct`, `create_index` / `indexes`.
- **`Mongo::Cursor`** — an Enumerable of ordered documents: `Each`, `ToArray`,
  `Next`, `Close`.
- **`BSON::Document`** — ordered, string-keyed, with byte-exact `ToBSON` /
  `DocumentFromBSON` and the terse `Doc("k", v, ...)` builder.
- **`BSON::*` values** — `ObjectId` (generate / from-string / to-s), `Binary`,
  `Timestamp`, `Decimal128`, `Int32` / `Int64` / `Double`, `Regexp`, plus
  `nil` / datetime / array mapping.
- **`Mongo::Error`** — the gem's error tree (`OperationFailure`,
  `ConnectionFailure`, `BulkWriteError`, `InvalidDocument`, duplicate-key …)
  mapped from the driver's `CommandError` / `WriteException` /
  `BulkWriteException`.

CGO-free, **100% test coverage**, `gofmt` + `go vet` clean, and green across the
six 64-bit Go targets (amd64, arm64, riscv64, loong64, ppc64le, s390x).

## Install

```sh
go get github.com/go-ruby-mongodb/mongodb
```

## Usage

```go
package main

import (
	"fmt"

	"github.com/go-ruby-mongodb/mongodb"
)

func main() {
	cli, _ := mongodb.NewClient("mongodb://localhost:27017", mongodb.WithDatabase("test"))
	defer cli.Close()

	people := cli.Database("").Collection("people") // Mongo::Client#[]

	people.InsertOne(mongodb.Doc("name", "Ada", "age", mongodb.Int32(36)))

	cur, _ := people.Find(mongodb.Doc("age", mongodb.Doc("$gte", mongodb.Int32(18))), nil)
	docs, _ := cur.ToArray()
	for _, d := range docs {
		name, _ := d.Get("name")
		fmt.Println(name)
	}

	people.UpdateOne(
		mongodb.Doc("name", "Ada"),
		mongodb.Doc("$set", mongodb.Doc("age", mongodb.Int32(37))),
		nil,
	)
}
```

## BSON type mapping

| Ruby (`bson` gem)  | This package (`BSON::*`)            | Driver wire type |
| ------------------ | ----------------------------------- | ---------------- |
| `BSON::Document`   | `Document` (ordered `[]Element`)    | document         |
| `BSON::ObjectId`   | `ObjectId`                          | objectId         |
| `BSON::Binary`     | `Binary`                            | binData          |
| `BSON::Timestamp`  | `Timestamp`                         | timestamp        |
| `BSON::Decimal128` | `Decimal128`                        | decimal128       |
| `BSON::Int32`      | `Int32`                             | int32            |
| `BSON::Int64`      | `Int64`                             | int64            |
| `BSON::Double`     | `Double`                            | double           |
| `BSON::Regexp`     | `Regexp`                            | regex            |
| `nil` / `Time` / `Array` | `nil` / `DateTime` / `[]any`  | null / date / array |

`Document#to_bson` is produced by the driver's canonical codec, so it is
byte-for-byte identical to `bson.Marshal` of the equivalent native document —
verified for every type above, including nested documents and arrays.

## Errors

Errors are `*mongodb.Error`, carrying the mapped `Mongo::Error` class the host
should raise and, where the server sent one, the numeric code:

```go
_, err := coll.InsertMany(docs) // duplicate _id
var e *mongodb.Error
if errors.As(err, &e) {
	fmt.Println(e.Class)             // Mongo::Error::BulkWriteError
	fmt.Println(e.IsDuplicateKey())  // true  (code 11000)
	fmt.Println(e.WriteErrors)       // per-document failures
}
```

`CommandError → OperationFailure`, `WriteException → OperationFailure`,
`BulkWriteException → BulkWriteError`, network/timeout → `ConnectionFailure`,
with unknown driver errors falling back to the base `Mongo::Error`.

## Backend & architectures

The engine is `go.mongodb.org/mongo-driver/v2` — MongoDB's own driver and BSON
codec, pure Go, **no cgo**. Every arch below builds and tests with
`CGO_ENABLED=0`:

| arch    | CGO=0 build | notes                          |
| ------- | ----------- | ------------------------------ |
| amd64   | ✅          | native CI lane                 |
| arm64   | ✅          | native CI lane                 |
| riscv64 | ✅          | qemu-user CI lane              |
| loong64 | ✅          | qemu-user CI lane              |
| ppc64le | ✅          | qemu-user CI lane              |
| s390x   | ✅          | qemu-user CI lane (big-endian) |

BSON is little-endian on the wire; the driver handles the byte order, so the
big-endian s390x lane proves the whole stack round-trips correctly.

## Tests & coverage

The suite is fully deterministic and needs **no live mongod**: the CRUD,
aggregation, and index tests drive the real `mongo-driver` v2 in-process against
a canned `MockDeployment`, so command construction is validated against the real
driver while coverage stays at 100% on every OS and cross-arch lane. BSON
byte-exactness is checked against the driver's canonical `bson.Marshal`
(plus a pinned golden vector), and the `Mongo::Error` taxonomy is unit-tested
against synthesised driver errors.

An optional round-trip oracle against a **real** server is gated behind
`MONGODB_URI` and skips itself when unset:

```sh
COVERPKG=$(go list ./... | paste -sd, -)
go test -race -coverpkg="$COVERPKG" -coverprofile=cover.out ./...
go tool cover -func=cover.out | tail -1        # 100.0%

MONGODB_URI=mongodb://localhost:27017 go test ./...   # + live round-trip
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-mongodb/mongodb authors.
