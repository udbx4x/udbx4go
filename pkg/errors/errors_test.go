package errors

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFormatError(t *testing.T) {
	err := FormatError("test message")
	assert.NotNil(t, err)
	assert.Equal(t, "test message", err.Error())
	assert.Equal(t, CodeFormatError, err.Code())

	// Test with cause
	cause := errors.New("original error")
	err2 := FormatError("test message", cause)
	assert.Contains(t, err2.Error(), "test message")
	assert.Contains(t, err2.Error(), "original error")
	assert.Equal(t, cause, err2.Unwrap())
}

func TestNewNotFoundError(t *testing.T) {
	err := NotFoundError("resource not found")
	assert.NotNil(t, err)
	assert.Equal(t, "resource not found", err.Error())
	assert.Equal(t, CodeNotFound, err.Code())
}

func TestNotFoundErrorf(t *testing.T) {
	err := NotFoundErrorf("dataset '%s' not found", "cities")
	assert.NotNil(t, err)
	assert.Equal(t, "dataset 'cities' not found", err.Error())
	assert.Equal(t, CodeNotFound, err.Code())
}

func TestNewUnsupportedError(t *testing.T) {
	err := UnsupportedError("feature not supported")
	assert.NotNil(t, err)
	assert.Equal(t, "feature not supported", err.Error())
	assert.Equal(t, CodeUnsupported, err.Code())
}

func TestNewConstraintError(t *testing.T) {
	err := ConstraintError("duplicate key")
	assert.NotNil(t, err)
	assert.Equal(t, "duplicate key", err.Error())
	assert.Equal(t, CodeConstraint, err.Code())
}

func TestNewIOError(t *testing.T) {
	err := IOError("read failed")
	assert.NotNil(t, err)
	assert.Equal(t, "read failed", err.Error())
	assert.Equal(t, CodeIOError, err.Code())
}

func TestIOErrorf(t *testing.T) {
	err := IOErrorf("failed to open file: %s", "test.udbx")
	assert.NotNil(t, err)
	assert.Equal(t, "failed to open file: test.udbx", err.Error())
	assert.Equal(t, CodeIOError, err.Code())
}

func TestIsFormatError(t *testing.T) {
	assert.True(t, IsFormatError(FormatError("test")))
	assert.False(t, IsFormatError(NotFoundError("test")))
	assert.False(t, IsFormatError(errors.New("other error")))
}

func TestIsNotFound(t *testing.T) {
	assert.True(t, IsNotFound(NotFoundError("test")))
	assert.False(t, IsNotFound(FormatError("test")))
	assert.False(t, IsNotFound(errors.New("other error")))
}

func TestIsUnsupported(t *testing.T) {
	assert.True(t, IsUnsupported(UnsupportedError("test")))
	assert.False(t, IsUnsupported(FormatError("test")))
	assert.False(t, IsUnsupported(errors.New("other error")))
}

func TestIsConstraintViolation(t *testing.T) {
	assert.True(t, IsConstraintViolation(ConstraintError("test")))
	assert.False(t, IsConstraintViolation(FormatError("test")))
	assert.False(t, IsConstraintViolation(errors.New("other error")))
}

func TestIsIOError(t *testing.T) {
	assert.True(t, IsIOError(IOError("test")))
	assert.False(t, IsIOError(FormatError("test")))
	assert.False(t, IsIOError(errors.New("other error")))
}

func TestIsUdbxError(t *testing.T) {
	assert.True(t, IsUdbxError(FormatError("test")))
	assert.True(t, IsUdbxError(NotFoundError("test")))
	assert.True(t, IsUdbxError(UnsupportedError("test")))
	assert.True(t, IsUdbxError(ConstraintError("test")))
	assert.True(t, IsUdbxError(IOError("test")))
	assert.False(t, IsUdbxError(errors.New("other error")))
}

func TestDatasetNotFound(t *testing.T) {
	err := DatasetNotFound("cities")
	assert.NotNil(t, err)
	assert.Equal(t, CodeNotFound, err.Code())
	assert.Contains(t, err.Error(), "cities")
}

func TestFeatureNotFound(t *testing.T) {
	err := FeatureNotFound("cities", 42)
	assert.NotNil(t, err)
	assert.Equal(t, CodeNotFound, err.Code())
	assert.Contains(t, err.Error(), "cities")
	assert.Contains(t, err.Error(), "42")
}

func TestFieldNotFound(t *testing.T) {
	err := FieldNotFound("population")
	assert.NotNil(t, err)
	assert.Equal(t, CodeNotFound, err.Code())
	assert.Contains(t, err.Error(), "population")
}

func TestErrorWrapping(t *testing.T) {
	original := errors.New("original error")
	wrapped := FormatError("wrapped message", original)

	assert.Equal(t, original, wrapped.Unwrap())
	assert.True(t, errors.Is(wrapped, original))
}
