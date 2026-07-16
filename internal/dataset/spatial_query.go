package dataset

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/udbx4x/udbx4go/internal/codec"
	"github.com/udbx4x/udbx4go/internal/sqliteutil"
	udbxerrors "github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// Query executes the RTree, envelope-cache, or bounded-sample spatial-query path.
func (q *SpatialQuerier) Query(ctx context.Context, options types.SpatialQueryOptions) (*types.SpatialQueryResult, error) {
	return q.QueryWithEnvelopeCache(ctx, options, nil)
}

// QueryWithEnvelopeCache executes a spatial query using a DataSource-owned cache manager.
func (q *SpatialQuerier) QueryWithEnvelopeCache(
	ctx context.Context,
	options types.SpatialQueryOptions,
	manager *EnvelopeCacheManager,
) (*types.SpatialQueryResult, error) {
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
		detected.IDColumn, detected.GeometryColumn, err = q.detectEnvelopeColumns(ctx)
		if err != nil {
			return nil, mapSpatialQueryExecutionError(ctx, err)
		}
	}

	strategy := types.SpatialQueryStrategyRTree
	degradedReason := types.SpatialQueryReason("")
	var candidateIDs []int
	var hasMore bool
	if detected.Capability.RTreeAvailable {
		candidateIDs, hasMore, err = q.queryRTreeCandidateIDs(ctx, detected, normalized)
	} else {
		candidateIDs, hasMore, strategy, degradedReason, err = q.queryFallbackCandidateIDs(ctx, detected, normalized, manager)
	}
	if err != nil {
		return nil, mapSpatialQueryExecutionError(ctx, err)
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
		Features:       features,
		QueriedBounds:  normalized.Bounds,
		Strategy:       strategy,
		HasMore:        hasMore,
		DegradedReason: degradedReason,
	}, nil
}

func (q *SpatialQuerier) queryFallbackCandidateIDs(
	ctx context.Context,
	detected *detectedSpatialCapability,
	options types.SpatialQueryOptions,
	manager *EnvelopeCacheManager,
) ([]int, bool, types.SpatialQueryStrategy, types.SpatialQueryReason, error) {
	if manager == nil {
		var err error
		manager, err = NewEnvelopeCacheManager(types.DefaultSpatialQueryPolicy())
		if err != nil {
			return nil, false, "", "", err
		}
		defer manager.Close()
	}

	cacheKey := q.info.TableName + "\x00" + detected.IDColumn + "\x00" + detected.GeometryColumn
	cache, err := manager.GetOrBuild(ctx, cacheKey, q.info.ObjectCount, func(buildCtx context.Context) ([]envelopeEntry, error) {
		return q.buildEnvelopeEntries(buildCtx, detected)
	})
	if err != nil {
		if stderrors.Is(err, errEnvelopeCacheBudgetExceeded) {
			ids, sampleErr := q.queryBoundedSampleIDs(ctx, detected.IDColumn, options.Limit)
			if sampleErr != nil {
				return nil, false, "", "", sampleErr
			}
			return ids,
				q.info.ObjectCount > options.Limit,
				types.SpatialQueryStrategyBoundedSample,
				types.SpatialQueryReasonEnvelopeCacheBudgetExceeded,
				nil
		}
		return nil, false, "", "", err
	}

	ids, hasMore, err := cache.CandidateIDs(options.Bounds, options.Limit)
	return ids, hasMore, types.SpatialQueryStrategyEnvelopeCache, "", err
}

func (q *SpatialQuerier) detectEnvelopeColumns(ctx context.Context) (string, string, error) {
	records, err := q.geoColsDao.ListByTableNameContext(ctx, q.info.TableName)
	if err != nil {
		return "", "", err
	}
	if len(records) != 1 || records[0].FTableName != q.info.TableName {
		return "", "", spatialQueryFailure(
			types.SpatialQueryReasonSpatialIndexUnavailable,
			udbxerrors.UnsupportedError("spatial query geometry metadata is unavailable"),
		)
	}
	geometryColumn := records[0].FGeometryColumn
	if geometryColumn == "" || (registeredGeometryColumn(q.record) != "" && registeredGeometryColumn(q.record) != geometryColumn) {
		return "", "", spatialQueryFailure(
			types.SpatialQueryReasonSpatialIndexUnavailable,
			udbxerrors.UnsupportedError("spatial query geometry column is unavailable"),
		)
	}
	idColumn := registeredIDColumn(q.record)
	tableColumns, err := sqliteTableInfo(ctx, q.db, q.info.TableName)
	if err != nil {
		return "", "", err
	}
	if !hasColumn(tableColumns, idColumn) || !hasColumn(tableColumns, geometryColumn) {
		return "", "", spatialQueryFailure(
			types.SpatialQueryReasonSpatialIndexUnavailable,
			udbxerrors.UnsupportedError("spatial query columns are unavailable"),
		)
	}
	return idColumn, geometryColumn, nil
}

func (q *SpatialQuerier) buildEnvelopeEntries(
	ctx context.Context,
	detected *detectedSpatialCapability,
) (entries []envelopeEntry, returnErr error) {
	quotedTable, err := sqliteutil.QuoteIdentifier(q.info.TableName)
	if err != nil {
		return nil, udbxerrors.IOError("failed to quote dataset table name", err)
	}
	quotedID, err := sqliteutil.QuoteIdentifier(detected.IDColumn)
	if err != nil {
		return nil, udbxerrors.IOError("failed to quote feature ID column", err)
	}
	quotedGeometry, err := sqliteutil.QuoteIdentifier(detected.GeometryColumn)
	if err != nil {
		return nil, udbxerrors.IOError("failed to quote geometry column", err)
	}

	query := fmt.Sprintf("SELECT %s, substr(%s, 1, %d)", quotedID, quotedGeometry, codec.GaiaHeaderLength) +
		" FROM " + quotedTable + " ORDER BY " + quotedID
	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, udbxerrors.IOError("failed to query GAIA envelope headers", err)
	}
	defer func() {
		if closeErr := rows.Close(); returnErr == nil && closeErr != nil {
			returnErr = udbxerrors.IOError("failed to close GAIA envelope rows", closeErr)
		}
	}()

	entries = make([]envelopeEntry, 0, q.info.ObjectCount)
	for rows.Next() {
		if len(entries)%256 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		var idValue interface{}
		var headerValue interface{}
		if err := rows.Scan(&idValue, &headerValue); err != nil {
			return nil, udbxerrors.IOError("failed to scan GAIA envelope row", err)
		}
		id, err := spatialEnvelopeID(idValue)
		if err != nil {
			return nil, err
		}
		header, ok := headerValue.([]byte)
		if !ok {
			return nil, newSpatialGeometryError("GAIA envelope header is not a BLOB")
		}
		envelope, err := codec.ReadGaiaEnvelope(header)
		if err != nil {
			return nil, &spatialGeometryError{cause: udbxerrors.FormatError("failed to read GAIA envelope", err)}
		}
		entries = append(entries, envelopeEntry{
			ID: id, MinX: envelope.MinX, MinY: envelope.MinY,
			MaxX: envelope.MaxX, MaxY: envelope.MaxY,
		})
	}
	if err := rows.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, udbxerrors.IOError("error iterating GAIA envelope rows", err)
	}
	return entries, nil
}

func spatialEnvelopeID(value interface{}) (int64, error) {
	var id int64
	switch typed := value.(type) {
	case int64:
		id = typed
	case int:
		id = int64(typed)
	default:
		return 0, udbxerrors.FormatError("envelope cache feature ID is not an integer")
	}
	if id <= 0 {
		return 0, udbxerrors.FormatError("envelope cache feature ID must be positive")
	}
	return id, nil
}

func (q *SpatialQuerier) queryBoundedSampleIDs(ctx context.Context, idColumn string, limit int) ([]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	quotedTable, err := sqliteutil.QuoteIdentifier(q.info.TableName)
	if err != nil {
		return nil, udbxerrors.IOError("failed to quote dataset table name", err)
	}
	quotedID, err := sqliteutil.QuoteIdentifier(idColumn)
	if err != nil {
		return nil, udbxerrors.IOError("failed to quote feature ID column", err)
	}
	rows, err := q.db.QueryContext(ctx, "SELECT "+quotedID+" FROM "+quotedTable+" ORDER BY "+quotedID+" LIMIT ?", limit)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, udbxerrors.IOError("failed to query bounded spatial sample", err)
	}
	defer rows.Close()

	ids := make([]int, 0, initialCandidateCapacity(limit))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, udbxerrors.FormatError("bounded sample feature ID is not an integer", err)
		}
		converted := int(id)
		if converted <= 0 || int64(converted) != id {
			return nil, udbxerrors.FormatError("bounded sample feature ID is outside the supported integer range")
		}
		ids = append(ids, converted)
	}
	if err := rows.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
		return nil, udbxerrors.IOError("error iterating bounded spatial sample", err)
	}
	if err := rows.Close(); err != nil {
		return nil, udbxerrors.IOError("failed to close bounded spatial sample rows", err)
	}
	return ids, nil
}

func mapSpatialQueryExecutionError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return spatialQueryFailure(types.SpatialQueryReasonQueryTimeout, ctxErr)
	}
	if stderrors.Is(err, context.Canceled) || stderrors.Is(err, context.DeadlineExceeded) {
		return spatialQueryFailure(types.SpatialQueryReasonQueryTimeout, err)
	}
	var geometryErr *spatialGeometryError
	if stderrors.As(err, &geometryErr) {
		return spatialQueryFailure(types.SpatialQueryReasonCorruptGeometry, err)
	}
	return err
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
	if err := rows.Close(); err != nil {
		return nil, false, udbxerrors.IOError("failed to close spatial index candidates", err)
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
