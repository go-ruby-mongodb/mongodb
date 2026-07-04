// Copyright (c) the go-ruby-mongodb/mongodb authors
//
// SPDX-License-Identifier: BSD-3-Clause

package mongodb

import (
	"bytes"
	"encoding/hex"
	"reflect"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// richDoc is a Document exercising every BSON value class, plus nesting and
// arrays, built through the Ruby surface.
func richDoc(t *testing.T) (Document, bson.D) {
	t.Helper()
	oid, err := ObjectIdFromString("64000000000000000000000a")
	if err != nil {
		t.Fatal(err)
	}
	dec, err := bson.ParseDecimal128("1.50")
	if err != nil {
		t.Fatal(err)
	}
	dt := NewDateTime(time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC))
	bin := Binary{Subtype: 0x00, Data: []byte{1, 2, 3}}
	ts := Timestamp{T: 12345, I: 2}
	re := Regexp{Pattern: "^a", Options: "i"}

	doc := Document{
		{Key: "str", Value: "hello"},
		{Key: "i32", Value: Int32(7)},
		{Key: "i64", Value: Int64(1 << 40)},
		{Key: "dbl", Value: Double(3.5)},
		{Key: "bool", Value: true},
		{Key: "nil", Value: nil},
		{Key: "oid", Value: oid},
		{Key: "bin", Value: bin},
		{Key: "ts", Value: ts},
		{Key: "dec", Value: dec},
		{Key: "re", Value: re},
		{Key: "dt", Value: dt},
		{Key: "sub", Value: Document{{Key: "x", Value: Int32(1)}}},
		{Key: "arr", Value: []any{Int32(1), "two", Document{{Key: "k", Value: true}}}},
		{Key: "docs", Value: []Document{{{Key: "a", Value: Int32(1)}}, {{Key: "b", Value: Int32(2)}}}},
	}

	native := bson.D{
		{Key: "str", Value: "hello"},
		{Key: "i32", Value: int32(7)},
		{Key: "i64", Value: int64(1 << 40)},
		{Key: "dbl", Value: 3.5},
		{Key: "bool", Value: true},
		{Key: "nil", Value: nil},
		{Key: "oid", Value: oid},
		{Key: "bin", Value: bin},
		{Key: "ts", Value: ts},
		{Key: "dec", Value: dec},
		{Key: "re", Value: re},
		{Key: "dt", Value: dt},
		{Key: "sub", Value: bson.D{{Key: "x", Value: int32(1)}}},
		{Key: "arr", Value: bson.A{int32(1), "two", bson.D{{Key: "k", Value: true}}}},
		{Key: "docs", Value: bson.A{bson.D{{Key: "a", Value: int32(1)}}, bson.D{{Key: "b", Value: int32(2)}}}},
	}
	return doc, native
}

// TestToBSONByteExact proves Document#to_bson is byte-identical to the driver's
// canonical bson.Marshal of the hand-built native document, for every type.
func TestToBSONByteExact(t *testing.T) {
	doc, native := richDoc(t)
	got, err := doc.ToBSON()
	if err != nil {
		t.Fatal(err)
	}
	want, err := bson.Marshal(native)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("to_bson mismatch\n got=%s\nwant=%s", hex.EncodeToString(got), hex.EncodeToString(want))
	}
}

// TestToBSONGolden pins the exact wire bytes of a canonical document so the
// check is not merely self-referential.
func TestToBSONGolden(t *testing.T) {
	got, err := Doc("a", Int32(1)).ToBSON()
	if err != nil {
		t.Fatal(err)
	}
	// {"a": <int32> 1}: len=0x0c, 0x10 int32, "a\0", 01000000, terminator.
	want := "0c0000001061000100000000"
	if hex.EncodeToString(got) != want {
		t.Fatalf("golden mismatch: got %s want %s", hex.EncodeToString(got), want)
	}
}

// TestBSONRoundTrip proves from_bson(to_bson(doc)) is stable: re-encoding the
// decoded document yields identical bytes.
func TestBSONRoundTrip(t *testing.T) {
	doc, _ := richDoc(t)
	b1, err := doc.ToBSON()
	if err != nil {
		t.Fatal(err)
	}
	back, err := DocumentFromBSON(b1)
	if err != nil {
		t.Fatal(err)
	}
	b2, err := back.ToBSON()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b1, b2) {
		t.Fatalf("round-trip not stable")
	}
	// The decoded top-level document preserves order and key set.
	if !reflect.DeepEqual(doc.Keys(), back.Keys()) {
		t.Fatalf("keys differ: %v vs %v", doc.Keys(), back.Keys())
	}
	// A nested document decodes back to a Document.
	sub, ok := back.Get("sub")
	if !ok {
		t.Fatal("sub missing")
	}
	if _, ok := sub.(Document); !ok {
		t.Fatalf("sub = %T, want Document", sub)
	}
	// An array decodes back to []any.
	arr, _ := back.Get("arr")
	if _, ok := arr.([]any); !ok {
		t.Fatalf("arr = %T, want []any", arr)
	}
}

func TestDocGetAndKeys(t *testing.T) {
	d := Doc("a", 1, "b", 2)
	if v, ok := d.Get("a"); !ok || v != 1 {
		t.Fatalf("Get a = %v %v", v, ok)
	}
	if _, ok := d.Get("missing"); ok {
		t.Fatal("missing should be absent")
	}
	if !reflect.DeepEqual(d.Keys(), []string{"a", "b"}) {
		t.Fatalf("keys = %v", d.Keys())
	}
}

func TestDocumentOfErrors(t *testing.T) {
	if _, err := DocumentOf("a"); err == nil {
		t.Fatal("odd args should error")
	}
	if _, err := DocumentOf(1, 2); err == nil {
		t.Fatal("non-string key should error")
	}
	if _, err := DocumentOf("a", 1); err != nil {
		t.Fatalf("valid should not error: %v", err)
	}
}

func TestDocPanicsOnBadArgs(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	Doc("dangling")
}

func TestNewObjectId(t *testing.T) {
	a := NewObjectId()
	b := NewObjectId()
	if a == b {
		t.Fatal("two generated ObjectIds should differ")
	}
	if a.IsZero() {
		t.Fatal("generated id is zero")
	}
}

func TestObjectIdFromStringError(t *testing.T) {
	if _, err := ObjectIdFromString("not-hex"); err == nil {
		t.Fatal("expected error")
	} else {
		var e *Error
		if !asError(err, &e) || e.Class != ErrInvalidDocument {
			t.Fatalf("class = %v", err)
		}
	}
}

// TestValueMapping exercises the toRuby / fromRuby helpers directly, including
// the array branches the wire codec does not surface on its own.
func TestValueMapping(t *testing.T) {
	// fromRuby: Document, []Document, []any, scalar passthrough.
	if _, ok := fromRuby(Document{{Key: "x", Value: 1}}).(bson.D); !ok {
		t.Fatal("fromRuby(Document) should be bson.D")
	}
	if a, ok := fromRuby([]Document{{{Key: "x", Value: 1}}}).(bson.A); !ok || len(a) != 1 {
		t.Fatal("fromRuby([]Document)")
	}
	if a, ok := fromRuby([]any{1, "s"}).(bson.A); !ok || len(a) != 2 {
		t.Fatal("fromRuby([]any)")
	}
	if fromRuby(42) != 42 {
		t.Fatal("fromRuby scalar")
	}
	// toRuby: bson.D, bson.A, []any, scalar passthrough.
	if _, ok := toRuby(bson.D{{Key: "x", Value: 1}}).(Document); !ok {
		t.Fatal("toRuby(bson.D) should be Document")
	}
	if a, ok := toRuby(bson.A{1, 2}).([]any); !ok || len(a) != 2 {
		t.Fatal("toRuby(bson.A)")
	}
	if a, ok := toRuby([]any{bson.A{1}}).([]any); !ok || len(a) != 1 {
		t.Fatal("toRuby([]any)")
	}
	if toRuby("s") != "s" {
		t.Fatal("toRuby scalar")
	}
	// toD on a nil Document yields an empty bson.D.
	var nilDoc Document
	if len(nilDoc.toD()) != 0 {
		t.Fatal("nil Document should encode to empty bson.D")
	}
}

func TestToBSONError(t *testing.T) {
	// A channel has no BSON representation, so Marshal fails.
	_, err := Document{{Key: "bad", Value: make(chan int)}}.ToBSON()
	if err == nil {
		t.Fatal("expected a marshal error")
	}
	var e *Error
	if !asError(err, &e) || e.Class != ErrInvalidDocument {
		t.Fatalf("class = %v", err)
	}
}

func TestDocumentFromBSONError(t *testing.T) {
	// Truncated / malformed BSON bytes.
	_, err := DocumentFromBSON([]byte{0x05, 0x00, 0x00})
	if err == nil {
		t.Fatal("expected an unmarshal error")
	}
	var e *Error
	if !asError(err, &e) || e.Class != ErrInvalidDocument {
		t.Fatalf("class = %v", err)
	}
}

// asError is errors.As without importing errors in the wide test file.
func asError(err error, target **Error) bool {
	for err != nil {
		if e, ok := err.(*Error); ok {
			*target = e
			return true
		}
		type unwrapper interface{ Unwrap() error }
		u, ok := err.(unwrapper)
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
