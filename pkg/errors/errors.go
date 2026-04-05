// Package errors provides UDBX-specific error types.
// All errors implement the UdbxError interface.
package errors

import (
	"errors"
	"fmt"
)

// Error codes for programmatic error handling.
const (
	CodeUdbxError     = "UDBX_ERROR"
	CodeFormatError   = "FORMAT_ERROR"
	CodeNotFound      = "NOT_FOUND"
	CodeUnsupported   = "UNSUPPORTED"
	CodeConstraint    = "CONSTRAINT_VIOLATION"
	CodeIOError       = "IO_ERROR"
)

// UdbxError is the interface for all UDBX errors.
type UdbxError interface {
	error
	Code() string
	Unwrap() error
}

// baseError is the base implementation of UdbxError.
type baseError struct {
	msg   string
	code  string
	cause error
}

// Error returns the error message.
func (e *baseError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.msg, e.cause)
	}
	return e.msg
}

// Code returns the error code.
func (e *baseError) Code() string {
	return e.code
}

// Unwrap returns the underlying cause.
func (e *baseError) Unwrap() error {
	return e.cause
}

// FormatError creates a new base error.
func FormatError(msg string, cause ...error) UdbxError {
	var c error
	if len(cause) > 0 {
		c = cause[0]
	}
	return &baseError{msg: msg, code: CodeFormatError, cause: c}
}

// NotFoundError creates a new not found error.
func NotFoundError(msg string, cause ...error) UdbxError {
	var c error
	if len(cause) > 0 {
		c = cause[0]
	}
	return &baseError{msg: msg, code: CodeNotFound, cause: c}
}

// NotFoundErrorf creates a new not found error with formatting.
func NotFoundErrorf(format string, args ...interface{}) UdbxError {
	return &baseError{msg: fmt.Sprintf(format, args...), code: CodeNotFound}
}

// UnsupportedError creates a new unsupported error.
func UnsupportedError(msg string, cause ...error) UdbxError {
	var c error
	if len(cause) > 0 {
		c = cause[0]
	}
	return &baseError{msg: msg, code: CodeUnsupported, cause: c}
}

// ConstraintError creates a new constraint error.
func ConstraintError(msg string, cause ...error) UdbxError {
	var c error
	if len(cause) > 0 {
		c = cause[0]
	}
	return &baseError{msg: msg, code: CodeConstraint, cause: c}
}

// IOError creates a new I/O error.
func IOError(msg string, cause ...error) UdbxError {
	var c error
	if len(cause) > 0 {
		c = cause[0]
	}
	return &baseError{msg: msg, code: CodeIOError, cause: c}
}

// IOErrorf creates a new I/O error with formatting.
func IOErrorf(format string, args ...interface{}) UdbxError {
	return &baseError{msg: fmt.Sprintf(format, args...), code: CodeIOError}
}

// IsFormatError checks if an error is a format error.
func IsFormatError(err error) bool {
	var e *baseError
	if errors.As(err, &e) {
		return e.code == CodeFormatError
	}
	return false
}

// IsNotFound checks if an error is a not found error.
func IsNotFound(err error) bool {
	var e *baseError
	if errors.As(err, &e) {
		return e.code == CodeNotFound
	}
	return false
}

// IsUnsupported checks if an error is an unsupported error.
func IsUnsupported(err error) bool {
	var e *baseError
	if errors.As(err, &e) {
		return e.code == CodeUnsupported
	}
	return false
}

// IsConstraintViolation checks if an error is a constraint error.
func IsConstraintViolation(err error) bool {
	var e *baseError
	if errors.As(err, &e) {
		return e.code == CodeConstraint
	}
	return false
}

// IsIOError checks if an error is an I/O error.
func IsIOError(err error) bool {
	var e *baseError
	if errors.As(err, &e) {
		return e.code == CodeIOError
	}
	return false
}

// IsUdbxError checks if an error is any UDBX error.
func IsUdbxError(err error) bool {
	var e UdbxError
	return errors.As(err, &e)
}

// Common sentinel errors for use with errors.Is.
var (
	ErrNotFound     = errors.New("not found")
	ErrFormat       = errors.New("format error")
	ErrUnsupported  = errors.New("unsupported")
	ErrConstraint   = errors.New("constraint violation")
	ErrIO           = errors.New("I/O error")
)

// DatasetNotFound creates an error for a missing dataset.
func DatasetNotFound(name string) UdbxError {
	return NotFoundErrorf("dataset '%s' not found", name)
}

// FeatureNotFound creates an error for a missing feature.
func FeatureNotFound(datasetName string, id int) UdbxError {
	return NotFoundErrorf("feature with id=%d not found in dataset '%s'", id, datasetName)
}

// FieldNotFound creates an error for a missing field.
func FieldNotFound(name string) UdbxError {
	return NotFoundErrorf("field '%s' does not exist", name)
}
