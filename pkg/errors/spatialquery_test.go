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
	err := NewSpatialQueryError(types.SpatialQueryReasonQueryTimeout, cause)
	require.Error(t, err)
	assert.ErrorIs(t, err, cause)
	assert.Contains(t, err.Error(), "query_timeout")
	assert.Contains(t, err.Error(), cause.Error())

	var spatialErr *SpatialQueryError
	require.ErrorAs(t, err, &spatialErr)
	assert.Equal(t, types.SpatialQueryReasonQueryTimeout, spatialErr.Reason)
	assert.Equal(t, cause, spatialErr.Err)
	assert.Equal(t, cause, spatialErr.Unwrap())
}

func TestNewSpatialQueryErrorRejectsInvalidArgumentsWithoutPanic(t *testing.T) {
	tests := []struct {
		name   string
		reason types.SpatialQueryReason
		err    error
	}{
		{name: "empty reason", err: stderrors.New("cause")},
		{name: "nil cause", reason: types.SpatialQueryReasonInvalidViewport},
		{name: "both invalid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotPanics(t, func() {
				err := NewSpatialQueryError(tt.reason, tt.err)
				require.Error(t, err)
				var spatialErr *SpatialQueryError
				assert.False(t, stderrors.As(err, &spatialErr))
			})
		})
	}
}

func TestSpatialQueryReasonOfUsesStandardErrorChain(t *testing.T) {
	cause := stderrors.New("corrupt blob")
	spatialErr := NewSpatialQueryError(types.SpatialQueryReasonCorruptGeometry, cause)
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
		{name: "empty reason", err: &SpatialQueryError{Err: cause}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, ok := SpatialQueryReasonOf(tt.err)
			assert.False(t, ok)
			assert.Empty(t, reason)
		})
	}
}
