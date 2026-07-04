// Copyright (c) the go-ruby-mongodb/mongodb authors
//
// SPDX-License-Identifier: BSD-3-Clause

package mongodb

import (
	"context"
	"errors"
	"testing"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

func TestMapErrorNil(t *testing.T) {
	if mapError(nil) != nil {
		t.Fatal("nil should map to nil")
	}
}

func TestMapErrorAlreadyClassified(t *testing.T) {
	in := &Error{Class: ErrInvalidDocument, Message: "boom"}
	if mapError(in) != error(in) {
		t.Fatal("a *mongodb.Error should pass through unchanged")
	}
}

func TestMapErrorCommand(t *testing.T) {
	in := mongo.CommandError{Code: 26, Message: "ns not found"}
	e, ok := mapError(in).(*Error)
	if !ok || e.Class != ErrOperationFailure || e.Code != 26 {
		t.Fatalf("got %+v", e)
	}
}

func TestMapErrorWriteException(t *testing.T) {
	in := mongo.WriteException{
		WriteErrors: mongo.WriteErrors{{Index: 2, Code: 121, Message: "doc failed validation"}},
	}
	e := mapError(in).(*Error)
	if e.Class != ErrOperationFailure || e.Code != 121 || len(e.WriteErrors) != 1 || e.WriteErrors[0].Index != 2 {
		t.Fatalf("got %+v", e)
	}
}

func TestMapErrorWriteExceptionConcernOnly(t *testing.T) {
	in := mongo.WriteException{
		WriteConcernError: &mongo.WriteConcernError{Code: 64, Message: "wtimeout"},
	}
	e := mapError(in).(*Error)
	if e.Class != ErrOperationFailure || e.Code != 64 {
		t.Fatalf("got %+v", e)
	}
}

func TestMapErrorBulkWrite(t *testing.T) {
	in := mongo.BulkWriteException{
		WriteErrors: []mongo.BulkWriteError{
			{WriteError: mongo.WriteError{Index: 0, Code: DuplicateKeyCode, Message: "E11000 dup"}},
		},
	}
	e := mapError(in).(*Error)
	if e.Class != ErrBulkWriteError || e.Code != DuplicateKeyCode {
		t.Fatalf("got %+v", e)
	}
	if !e.IsDuplicateKey() {
		t.Fatal("should be duplicate key")
	}
}

func TestMapErrorBulkWriteConcernOnly(t *testing.T) {
	in := mongo.BulkWriteException{
		WriteConcernError: &mongo.WriteConcernError{Code: 64, Message: "wtimeout"},
	}
	e := mapError(in).(*Error)
	if e.Class != ErrBulkWriteError || e.Code != 64 {
		t.Fatalf("got %+v", e)
	}
}

func TestMapErrorTimeout(t *testing.T) {
	e := mapError(context.DeadlineExceeded).(*Error)
	if e.Class != ErrConnectionFailure {
		t.Fatalf("got %+v", e)
	}
}

func TestMapErrorDisconnected(t *testing.T) {
	e := mapError(mongo.ErrClientDisconnected).(*Error)
	if e.Class != ErrConnectionFailure {
		t.Fatalf("got %+v", e)
	}
}

func TestMapErrorBase(t *testing.T) {
	e := mapError(errors.New("something odd")).(*Error)
	if e.Class != ErrBase {
		t.Fatalf("got %+v", e)
	}
}

func TestErrorInterface(t *testing.T) {
	wrapped := errors.New("inner")
	e := &Error{Class: ErrOperationFailure, Message: "outer", Wrapped: wrapped}
	if e.Error() != "outer" {
		t.Fatalf("Error() = %q", e.Error())
	}
	if !errors.Is(e, wrapped) {
		t.Fatal("Unwrap should expose the wrapped error")
	}
}

func TestIsDuplicateKey(t *testing.T) {
	if (&Error{Code: DuplicateKeyCode}).IsDuplicateKey() != true {
		t.Fatal("top-level code")
	}
	if (&Error{WriteErrors: []WriteError{{Code: DuplicateKeyCode}}}).IsDuplicateKey() != true {
		t.Fatal("write-error code")
	}
	if (&Error{Code: 1}).IsDuplicateKey() != false {
		t.Fatal("non-dup")
	}
}
