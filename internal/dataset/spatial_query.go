package dataset

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/udbx4x/udbx4go/internal/sqliteutil"
	udbxerrors "github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// Query executes the Task 4 RTree spatial-query path.
func (q *SpatialQuerier) Query(ctx context.Context, options types.SpatialQueryOptions) (*types.SpatialQueryResult, error) {
	normalized, err := options.Normalize()
	if err != nil {
		return nil, spatialQueryFailure(
			types.SpatialQueryReasonInvalidViewport,
			udbxerrors.ConstraintError("invalid spatial query options", err),
		)
	}
	if err := ctx.Err(); err != nil {
		return nil, spatialQueryFailure(types.SpatialQueryReasonQueryTimeout, err)
	}

	detected, err := q.detectCapability(ctx)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, spatialQueryFailure(types.SpatialQueryReasonQueryTimeout, ctxErr)
		}
		return nil, err
	}
	if !detected.Capability.Supported {
		return nil, spatialQueryFailure(
			types.SpatialQueryReasonUnsupportedDatasetKind,
			udbxerrors.UnsupportedError(fmt.Sprintf("dataset kind '%s' does not support spatial queries", q.info.Kind.String())),
		)
	}
	if !detected.Capability.RTreeAvailable {
		return nil, spatialQueryFailure(
			types.SpatialQueryReasonSpatialIndexUnavailable,
			udbxerrors.UnsupportedError("spatial index is unavailable"),
		)
	}

	candidateIDs, hasMore, err := q.queryRTreeCandidateIDs(ctx, detected, normalized)
	if err != nil {
		return nil, err
	}
	orderedIDs := appendRequiredSpatialIDs(candidateIDs, normalized.RequiredIDs)

	vector := NewVectorDataset(q.db, q.info)
	featuresByID, err := vector.loadFeaturesByIDs(ctx, orderedIDs, detected.IDColumn, detected.GeometryColumn)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, spatialQueryFailure(types.SpatialQueryReasonQueryTimeout, ctxErr)
		}
		var geometryErr *spatialGeometryError
		if stderrors.As(err, &geometryErr) {
			return nil, spatialQueryFailure(types.SpatialQueryReasonCorruptGeometry, err)
		}
		return nil, err
	}

	features := make([]*types.Feature, 0, len(orderedIDs))
	for _, id := range orderedIDs {
		if feature, exists := featuresByID[id]; exists {
			features = append(features, feature)
		}
	}

	return &types.SpatialQueryResult{
		Features:      features,
		QueriedBounds: normalized.Bounds,
		Strategy:      types.SpatialQueryStrategyRTree,
		HasMore:       hasMore,
	}, nil
}

func (q *SpatialQuerier) queryRTreeCandidateIDs(
	ctx context.Context,
	detected *detectedSpatialCapability,
	options types.SpatialQueryOptions,
) ([]int, bool, error) {
	quotedRTree, err := sqliteutil.QuoteIdentifier(detected.RTreeName)
	if err != nil {
		return nil, false, udbxerrors.IOError("failed to quote spatial index name", err)
	}
	quotedTable, err := sqliteutil.QuoteIdentifier(q.info.TableName)
	if err != nil {
		return nil, false, udbxerrors.IOError("failed to quote dataset table name", err)
	}
	quotedID, err := sqliteutil.QuoteIdentifier(detected.IDColumn)
	if err != nil {
		return nil, false, udbxerrors.IOError("failed to quote feature ID column", err)
	}
	quotedRowID, err := sqliteutil.QuoteIdentifier("rowid")
	if err != nil {
		return nil, false, udbxerrors.IOError("failed to quote SQLite row ID", err)
	}
	quotedPKID, err := sqliteutil.QuoteIdentifier("pkid")
	if err != nil {
		return nil, false, udbxerrors.IOError("failed to quote RTree primary key", err)
	}
	quotedXMin, err := sqliteutil.QuoteIdentifier("xmin")
	if err != nil {
		return nil, false, udbxerrors.IOError("failed to quote RTree minimum X column", err)
	}
	quotedXMax, err := sqliteutil.QuoteIdentifier("xmax")
	if err != nil {
		return nil, false, udbxerrors.IOError("failed to quote RTree maximum X column", err)
	}
	quotedYMin, err := sqliteutil.QuoteIdentifier("ymin")
	if err != nil {
		return nil, false, udbxerrors.IOError("failed to quote RTree minimum Y column", err)
	}
	quotedYMax, err := sqliteutil.QuoteIdentifier("ymax")
	if err != nil {
		return nil, false, udbxerrors.IOError("failed to quote RTree maximum Y column", err)
	}

	query := "SELECT d." + quotedID +
		" FROM " + quotedRTree + " AS r" +
		" JOIN " + quotedTable + " AS d ON d." + quotedRowID + " = r." + quotedPKID +
		" WHERE r." + quotedXMax + " >= ? AND r." + quotedXMin + " <= ?" +
		" AND r." + quotedYMax + " >= ? AND r." + quotedYMin + " <= ?" +
		" ORDER BY d." + quotedID +
		" LIMIT ?"
	rows, err := q.db.QueryContext(
		ctx,
		query,
		options.Bounds.MinX,
		options.Bounds.MaxX,
		options.Bounds.MinY,
		options.Bounds.MaxY,
		options.Limit+1,
	)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, spatialQueryFailure(types.SpatialQueryReasonQueryTimeout, ctxErr)
		}
		return nil, false, udbxerrors.IOError("failed to query spatial index", err)
	}
	defer rows.Close()

	ids := make([]int, 0, initialCandidateCapacity(options.Limit))
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, false, udbxerrors.FormatError("spatial index candidate ID is not an integer", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, false, spatialQueryFailure(types.SpatialQueryReasonQueryTimeout, ctxErr)
		}
		return nil, false, udbxerrors.IOError("error iterating spatial index candidates", err)
	}

	hasMore := len(ids) > options.Limit
	if hasMore {
		ids = ids[:options.Limit]
	}
	return ids, hasMore, nil
}

func initialCandidateCapacity(limit int) int {
	const maxInitialCapacity = 1024
	capacity := limit + 1
	if capacity > maxInitialCapacity {
		return maxInitialCapacity
	}
	return capacity
}

func appendRequiredSpatialIDs(candidateIDs, requiredIDs []int) []int {
	ordered := make([]int, 0, len(candidateIDs)+len(requiredIDs))
	seen := make(map[int]struct{}, len(candidateIDs)+len(requiredIDs))
	for _, id := range candidateIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ordered = append(ordered, id)
	}
	for _, id := range requiredIDs {
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		ordered = append(ordered, id)
	}
	return ordered
}

func spatialQueryFailure(reason types.SpatialQueryReason, cause error) error {
	spatialErr, err := udbxerrors.NewSpatialQueryError(reason, cause)
	if err != nil {
		return err
	}
	return spatialErr
}
