package errors

import (
	stderrors "errors"
	"fmt"

	"github.com/udbx4x/udbx4go/pkg/types"
)

// SpatialQueryError associates a spatial-query failure reason with its cause.
type SpatialQueryError struct {
	reason types.SpatialQueryReason
	cause  error
}

// Error returns the spatial-query failure message.
func (e *SpatialQueryError) Error() string {
	if e == nil {
		return "spatial query error: <nil>"
	}
	return fmt.Sprintf("spatial query %s: %v", e.reason, e.cause)
}

// Unwrap returns the underlying spatial-query failure.
func (e *SpatialQueryError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// Reason returns the spatial-query diagnostic reason.
func (e *SpatialQueryError) Reason() types.SpatialQueryReason {
	if e == nil {
		return ""
	}
	return e.reason
}

// Code returns the wrapped UDBX error code or the general UDBX error code.
func (e *SpatialQueryError) Code() string {
	if e == nil {
		return CodeUdbxError
	}
	var udbxErr UdbxError
	if stderrors.As(e.cause, &udbxErr) {
		return udbxErr.Code()
	}
	return CodeUdbxError
}

// NewSpatialQueryError creates a validated spatial-query error.
func NewSpatialQueryError(reason types.SpatialQueryReason, cause error) (*SpatialQueryError, error) {
	if !reason.Valid() {
		return nil, fmt.Errorf("spatial query reason is invalid: %q", reason)
	}
	if cause == nil {
		return nil, fmt.Errorf("spatial query cause must not be nil")
	}
	return &SpatialQueryError{reason: reason, cause: cause}, nil
}

// SpatialQueryReasonOf extracts a spatial-query reason from an error chain.
func SpatialQueryReasonOf(err error) (types.SpatialQueryReason, bool) {
	var spatialErr *SpatialQueryError
	if !stderrors.As(err, &spatialErr) || spatialErr == nil {
		return "", false
	}
	reason := spatialErr.Reason()
	if !reason.Valid() {
		return "", false
	}
	return reason, true
}
