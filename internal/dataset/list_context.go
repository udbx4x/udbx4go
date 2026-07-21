package dataset

import (
	"context"
	stderrors "errors"

	"github.com/udbx4x/udbx4go/pkg/types"
)

func mapSpatialListError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return spatialQueryFailure(types.SpatialQueryReasonQueryTimeout, ctxErr)
	}
	var geometryErr *spatialGeometryError
	if stderrors.As(err, &geometryErr) {
		return spatialQueryFailure(types.SpatialQueryReasonCorruptGeometry, err)
	}
	return err
}
