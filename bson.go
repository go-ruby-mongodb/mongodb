// Copyright (c) the go-ruby-mongodb/mongodb authors
//
// SPDX-License-Identifier: BSD-3-Clause

package mongodb

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// The BSON value classes, mirroring the Ruby bson gem. They are the driver's
// canonical types, so every value placed in a Document encodes byte-for-byte
// the way MongoDB's own BSON codec encodes it.
type (
	// ObjectId is BSON::ObjectId: a 12-byte MongoDB object identifier.
	ObjectId = bson.ObjectID
	// Binary is BSON::Binary: a typed binary blob (Subtype + Data).
	Binary = bson.Binary
	// Timestamp is BSON::Timestamp: the internal MongoDB (T, I) timestamp.
	Timestamp = bson.Timestamp
	// Decimal128 is BSON::Decimal128: an IEEE-754 decimal128 value.
	Decimal128 = bson.Decimal128
	// Regexp is BSON::Regexp: a BSON regular expression (Pattern + Options).
	Regexp = bson.Regex
	// DateTime is the BSON UTC datetime, milliseconds since the Unix epoch.
	DateTime = bson.DateTime
	// Int32 forces the BSON 32-bit integer wire type, like BSON::Int32.
	Int32 = int32
	// Int64 forces the BSON 64-bit integer wire type, like BSON::Int64.
	Int64 = int64
	// Double forces the BSON double wire type, like BSON::Double.
	Double = float64
)

// Element is a single ordered key/value pair inside a Document, the shape the
// bson gem yields when iterating a BSON::Document.
type Element struct {
	Key   string
	Value any
}

// Document is BSON::Document: an ordered, string-keyed document. Insertion order
// is preserved on the wire and on read-back, exactly as the gem's ordered hash.
type Document []Element

// Doc builds a Document from an alternating key/value list, a terse stand-in for
// the gem's BSON::Document[...] / hash literal:
//
//	mongodb.Doc("age", mongodb.Doc("$gte", int32(18)))
//
// It panics if len(kv) is odd or a key is not a string, mirroring how a Ruby
// hash literal with a dangling value is a syntax error rather than a runtime
// value. Use DocumentOf for a checked, error-returning builder.
func Doc(kv ...any) Document {
	d, err := DocumentOf(kv...)
	if err != nil {
		panic(err)
	}
	return d
}

// DocumentOf is the checked form of Doc: it returns an error instead of
// panicking when the key/value list is malformed.
func DocumentOf(kv ...any) (Document, error) {
	if len(kv)%2 != 0 {
		return nil, &Error{Class: ErrInvalidDocument, Message: "mongodb.Doc: odd number of key/value arguments"}
	}
	d := make(Document, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		key, ok := kv[i].(string)
		if !ok {
			return nil, &Error{Class: ErrInvalidDocument, Message: "mongodb.Doc: key is not a string"}
		}
		d = append(d, Element{Key: key, Value: kv[i+1]})
	}
	return d, nil
}

// Get returns the value stored under key and whether it was present. On
// duplicate keys it returns the first, matching Ruby hash semantics.
func (d Document) Get(key string) (any, bool) {
	for _, e := range d {
		if e.Key == key {
			return e.Value, true
		}
	}
	return nil, false
}

// Keys returns the document's keys in order.
func (d Document) Keys() []string {
	ks := make([]string, len(d))
	for i, e := range d {
		ks[i] = e.Key
	}
	return ks
}

// NewObjectId generates a fresh ObjectId, like BSON::ObjectId.new.
func NewObjectId() ObjectId { return bson.NewObjectID() }

// ObjectIdFromString parses a 24-character hex string into an ObjectId, like
// BSON::ObjectId.from_string. The error is a *mongodb.Error of class
// Mongo::Error::InvalidDocument when the string is not valid.
func ObjectIdFromString(s string) (ObjectId, error) {
	id, err := bson.ObjectIDFromHex(s)
	if err != nil {
		return ObjectId{}, &Error{Class: ErrInvalidDocument, Message: err.Error(), Wrapped: err}
	}
	return id, nil
}

// ToBSON serialises the document to BSON bytes, the gem's BSON::Document#to_bson.
// The bytes are produced by the driver's canonical codec and are therefore
// byte-identical to bson.Marshal of the equivalent native document.
func (d Document) ToBSON() ([]byte, error) {
	b, err := bson.Marshal(d.toD())
	if err != nil {
		return nil, &Error{Class: ErrInvalidDocument, Message: err.Error(), Wrapped: err}
	}
	return b, nil
}

// DocumentFromBSON parses BSON bytes into an ordered Document, the gem's
// BSON::Document.from_bson.
func DocumentFromBSON(b []byte) (Document, error) {
	var raw bson.D
	if err := bson.Unmarshal(b, &raw); err != nil {
		return nil, &Error{Class: ErrInvalidDocument, Message: err.Error(), Wrapped: err}
	}
	return fromD(raw), nil
}

// toD converts a Document (and any nested Documents / []any) into the driver's
// ordered bson.D, the representation its codec marshals. This is the outbound
// half of the Ruby<->BSON value mapping.
func (d Document) toD() bson.D {
	if d == nil {
		return bson.D{}
	}
	out := make(bson.D, len(d))
	for i, e := range d {
		out[i] = bson.E{Key: e.Key, Value: fromRuby(e.Value)}
	}
	return out
}

// fromRuby maps an outbound Ruby-surface value to the driver representation:
// Documents become bson.D, slices become bson.A, and scalars pass through (they
// are already the driver's BSON value types).
func fromRuby(v any) any {
	switch x := v.(type) {
	case Document:
		return x.toD()
	case []Document:
		a := make(bson.A, len(x))
		for i := range x {
			a[i] = x[i].toD()
		}
		return a
	case []any:
		a := make(bson.A, len(x))
		for i := range x {
			a[i] = fromRuby(x[i])
		}
		return a
	default:
		return v
	}
}

// fromD converts a driver bson.D into an ordered Document.
func fromD(d bson.D) Document {
	out := make(Document, len(d))
	for i, e := range d {
		out[i] = Element{Key: e.Key, Value: toRuby(e.Value)}
	}
	return out
}

// toRuby maps an inbound driver value to the Ruby surface: bson.D becomes a
// Document, bson.A / bson.RawArray-decoded slices become []any, and scalars pass
// through as the BSON value classes the host binding turns into Ruby objects.
func toRuby(v any) any {
	switch x := v.(type) {
	case bson.D:
		return fromD(x)
	case bson.A:
		out := make([]any, len(x))
		for i := range x {
			out[i] = toRuby(x[i])
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = toRuby(x[i])
		}
		return out
	default:
		return v
	}
}

// NewDateTime builds a BSON DateTime from a time.Time, truncated to
// milliseconds and normalised to UTC, matching how the gem stores Time.
func NewDateTime(t time.Time) DateTime { return bson.NewDateTimeFromTime(t) }
