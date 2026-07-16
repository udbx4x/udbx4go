package errors

import (
	stderrors "errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestSpatialQueryErrorWrapsCauseAndExposesReason(t *testing.T) {
	cause := stderrors.New("database interrupted")
	err, createErr := NewSpatialQueryError(types.SpatialQueryReasonQueryTimeout, cause)
	require.NoError(t, createErr)
	assert.ErrorIs(t, err, cause)
	assert.Contains(t, err.Error(), "query_timeout")
	assert.Contains(t, err.Error(), cause.Error())
	assert.Equal(t, types.SpatialQueryReasonQueryTimeout, err.Reason())
	assert.Equal(t, cause, err.Unwrap())
	assert.Equal(t, CodeUdbxError, err.Code())
	assert.True(t, IsUdbxError(err))
}

func TestNewSpatialQueryErrorRejectsInvalidArgumentsWithoutPanic(t *testing.T) {
	tests := []struct {
		name   string
		reason types.SpatialQueryReason
		err    error
	}{
		{name: "empty reason", err: stderrors.New("cause")},
		{name: "unknown reason", reason: types.SpatialQueryReason("unknown"), err: stderrors.New("cause")},
		{name: "nil cause", reason: types.SpatialQueryReasonInvalidViewport},
		{name: "both invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				spatialErr, err := NewSpatialQueryError(tt.reason, tt.err)
				assert.Nil(t, spatialErr)
				require.Error(t, err)
			})
		})
	}
}

func TestSpatialQueryErrorPreservesUdbxErrorCode(t *testing.T) {
	tests := []struct {
		name  string
		cause error
		code  string
	}{
		{name: "IO error", cause: IOError("read failed"), code: CodeIOError},
		{name: "format error", cause: FormatError("invalid geometry"), code: CodeFormatError},
		{name: "wrapped IO error", cause: fmt.Errorf("context: %w", IOError("read failed")), code: CodeIOError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err, createErr := NewSpatialQueryError(types.SpatialQueryReasonCorruptGeometry, tt.cause)
			require.NoError(t, createErr)
			assert.Equal(t, tt.code, err.Code())
			assert.True(t, IsUdbxError(err))
		})
	}
}

func TestSpatialQueryReasonOfUsesStandardErrorChain(t *testing.T) {
	cause := stderrors.New("corrupt blob")
	spatialErr, err := NewSpatialQueryError(types.SpatialQueryReasonCorruptGeometry, cause)
	require.NoError(t, err)
	wrapped := fmt.Errorf("read feature: %w", spatialErr)

	reason, ok := SpatialQueryReasonOf(wrapped)
	assert.True(t, ok)
	assert.Equal(t, types.SpatialQueryReasonCorruptGeometry, reason)

	tests := []struct {
		name string
		err  error
	}{
		{name: "nil"},
		{name: "unrelated", err: stderrors.New("other")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, ok := SpatialQueryReasonOf(tt.err)
			assert.False(t, ok)
			assert.Empty(t, reason)
		})
	}
}
