// Copyright (c) the go-ruby-mongodb/mongodb authors
//
// SPDX-License-Identifier: BSD-3-Clause

package mongodb

import (
	"errors"

	"go.mongodb.org/mongo-driver/v2/mongo"
)

// ErrorClass is the Ruby Mongo::Error subclass an error maps to. The rbgo
// binding raises the matching Ruby class; from Go it identifies the error
// category without string matching. Every *Error carries one.
type ErrorClass string

// The Mongo::Error hierarchy, as raised by the mongo gem. Names match the Ruby
// class names exactly so the rbgo binding is a direct lookup.
const (
	// ErrBase is the root Mongo::Error.
	ErrBase ErrorClass = "Mongo::Error"
	// ErrOperationFailure is Mongo::Error::OperationFailure, raised for a command
	// or write that the server rejected (including duplicate-key writes).
	ErrOperationFailure ErrorClass = "Mongo::Error::OperationFailure"
	// ErrConnectionFailure is Mongo::Error::ConnectionFailure, a network- or
	// server-selection-level failure.
	ErrConnectionFailure ErrorClass = "Mongo::Error::ConnectionFailure"
	// ErrBulkWriteError is Mongo::Error::BulkWriteError, raised when a bulk /
	// InsertMany operation reports per-document write errors.
	ErrBulkWriteError ErrorClass = "Mongo::Error::BulkWriteError"
	// ErrInvalidDocument is Mongo::Error::InvalidDocument, a client-side document
	// or argument that cannot be serialised.
	ErrInvalidDocument ErrorClass = "Mongo::Error::InvalidDocument"
	// ErrNoServerAvailable is Mongo::Error::NoServerAvailable, raised when no
	// server matching the read preference could be selected.
	ErrNoServerAvailable ErrorClass = "Mongo::Error::NoServerAvailable"
)

// DuplicateKeyCode is the MongoDB server error code for a duplicate key
// (E11000), the code the gem exposes on a duplicate-key OperationFailure.
const DuplicateKeyCode = 11000

// Error is a MongoDB error carrying the mapped Mongo::Error subclass and, where
// the server provided one, the numeric error code. It corresponds to a raised
// Mongo::Error in the gem; Class names the Ruby class the rbgo binding raises.
type Error struct {
	// Class is the Mongo::Error subclass this error maps to.
	Class ErrorClass
	// Code is the server error code, or 0 when there is none (e.g. a client-side
	// or network error).
	Code int
	// Message is the human-readable error message.
	Message string
	// WriteErrors holds the per-document write errors of a bulk / InsertMany
	// failure; it is empty for other error classes.
	WriteErrors []WriteError
	// Wrapped is the underlying driver error, if any.
	Wrapped error
}

// WriteError is a single per-document write failure inside a BulkWriteError,
// mirroring the gem's per-result error detail.
type WriteError struct {
	// Index is the position of the offending document in the input slice.
	Index int
	// Code is the server error code for this write.
	Code int
	// Message is the human-readable message for this write.
	Message string
}

// Error implements the error interface.
func (e *Error) Error() string { return e.Message }

// Unwrap exposes the underlying driver error for errors.Is / errors.As.
func (e *Error) Unwrap() error { return e.Wrapped }

// IsDuplicateKey reports whether the error is an OperationFailure caused by a
// duplicate key (server code 11000), the gem's common
// Mongo::Error::OperationFailure#/E11000/ check.
func (e *Error) IsDuplicateKey() bool {
	if e.Code == DuplicateKeyCode {
		return true
	}
	for _, we := range e.WriteErrors {
		if we.Code == DuplicateKeyCode {
			return true
		}
	}
	return false
}

// mapError converts an error returned by the mongo-driver into a
// *mongodb.Error with the gem-faithful class. A nil error passes through as nil.
// Errors that already carry a class (client-side validation raised in this
// package) pass through unchanged.
func mapError(err error) error {
	if err == nil {
		return nil
	}

	var already *Error
	if errors.As(err, &already) {
		return already
	}

	var ce mongo.CommandError
	if errors.As(err, &ce) {
		return &Error{
			Class:   ErrOperationFailure,
			Code:    int(ce.Code),
			Message: ce.Message,
			Wrapped: err,
		}
	}

	var bwe mongo.BulkWriteException
	if errors.As(err, &bwe) {
		out := &Error{Class: ErrBulkWriteError, Message: err.Error(), Wrapped: err}
		for _, we := range bwe.WriteErrors {
			out.WriteErrors = append(out.WriteErrors, WriteError{
				Index:   we.Index,
				Code:    we.Code,
				Message: we.Message,
			})
			if out.Code == 0 {
				out.Code = we.Code
			}
		}
		if bwe.WriteConcernError != nil && out.Code == 0 {
			out.Code = bwe.WriteConcernError.Code
		}
		return out
	}

	var we mongo.WriteException
	if errors.As(err, &we) {
		out := &Error{Class: ErrOperationFailure, Message: err.Error(), Wrapped: err}
		for _, w := range we.WriteErrors {
			out.WriteErrors = append(out.WriteErrors, WriteError{
				Index:   w.Index,
				Code:    w.Code,
				Message: w.Message,
			})
			if out.Code == 0 {
				out.Code = w.Code
			}
		}
		if we.WriteConcernError != nil && out.Code == 0 {
			out.Code = we.WriteConcernError.Code
		}
		return out
	}

	if mongo.IsNetworkError(err) || mongo.IsTimeout(err) {
		return &Error{Class: ErrConnectionFailure, Message: err.Error(), Wrapped: err}
	}

	if errors.Is(err, mongo.ErrClientDisconnected) {
		return &Error{Class: ErrConnectionFailure, Message: err.Error(), Wrapped: err}
	}

	// Anything else (server selection, unknown driver error) surfaces as the base
	// Mongo::Error so callers always receive a *mongodb.Error to inspect.
	return &Error{Class: ErrBase, Message: err.Error(), Wrapped: err}
}
