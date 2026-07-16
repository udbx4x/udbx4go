package errors

import (
	stderrors "errors"
	"fmt"

	"github.com/udbx4x/udbx4go/pkg/types"
)

// SpatialQueryError associates a spatial-query failure reason with its cause.
type SpatialQueryError struct {
	Reason types.SpatialQueryReason
	Err    error
}

// Error returns the spatial-query failure message.
func (e *SpatialQueryError) Error() string {
	return fmt.Sprintf("spatial query %s: %v", e.Reason, e.Err)
}

// Unwrap returns the underlying spatial-query failure.
func (e *SpatialQueryError) Unwrap() error {
	return e.Err
}

// NewSpatialQueryError creates a validated spatial-query error.
func NewSpatialQueryError(reason types.SpatialQueryReason, err error) error {
	if reason == "" {
		return fmt.Errorf("spatial query reason must not be empty")
	}
	if err == nil {
		return fmt.Errorf("spatial query cause must not be nil")
	}
	return &SpatialQueryError{Reason: reason, Err: err}
}

// SpatialQueryReasonOf extracts a spatial-query reason from an error chain.
func SpatialQueryReasonOf(err error) (types.SpatialQueryReason, bool) {
	var spatialErr *SpatialQueryError
	if !stderrors.As(err, &spatialErr) || spatialErr.Reason == "" {
		return "", false
	}
	return spatialErr.Reason, true
}
