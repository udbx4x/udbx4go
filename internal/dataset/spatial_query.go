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

// Query executes the RTree or envelope-cache spatial-query path.
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
	if !detected.Capability.RTreeAvailable && !detected.Capability.FallbackAvailable {
		return nil, spatialQueryFailure(
			types.SpatialQueryReasonSpatialIndexUnavailable,
			udbxerrors.UnsupportedError("spatial query columns are unavailable"),
		)
	}
	if manager == nil {
		manager, err = NewEnvelopeCacheManager(types.DefaultSpatialQueryPolicy())
		if err != nil {
			return nil, err
		}
		defer manager.Close()
	}

	strategy := types.SpatialQueryStrategyRTree
	var candidateIDs []int
	var hasMore bool
	if detected.Capability.RTreeAvailable {
		candidateIDs, hasMore, err = q.queryRTreeCandidateIDs(ctx, detected, normalized, manager)
	} else {
		candidateIDs, hasMore, strategy, err = q.queryFallbackCandidateIDs(ctx, detected, normalized, manager)
	}
	if err != nil {
		return nil, mapSpatialQueryExecutionError(ctx, err)
	}
	orderedIDs := appendRequiredSpatialIDs(candidateIDs, normalized.RequiredIDs)

	featuresByID, err := q.loadFeaturesByIDs(ctx, orderedIDs, detected)
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
		Strategy:      strategy,
		HasMore:       hasMore,
	}, nil
}

func (q *SpatialQuerier) loadFeaturesByIDs(
	ctx context.Context,
	ids []int,
	detected *detectedSpatialCapability,
) (map[int]*types.Feature, error) {
	switch q.info.Kind {
	case types.DatasetKindText:
		return NewTextDataset(q.db, q.info).loadFeaturesByIDs(
			ctx, ids, detected.IDColumn, detected.PayloadColumn, detected.EnvelopeColumn,
		)
	case types.DatasetKindCAD:
		return NewCadDataset(q.db, q.info).loadFeaturesByIDs(
			ctx, ids, detected.IDColumn, detected.PayloadColumn, detected.EnvelopeColumn, detected.CADTypeColumn,
		)
	default:
		return NewVectorDataset(q.db, q.info).loadFeaturesByIDs(
			ctx, ids, detected.IDColumn, detected.PayloadColumn,
		)
	}
}

func (q *SpatialQuerier) queryFallbackCandidateIDs(
	ctx context.Context,
	detected *detectedSpatialCapability,
	options types.SpatialQueryOptions,
	manager *EnvelopeCacheManager,
) ([]int, bool, types.SpatialQueryStrategy, error) {
	cacheKey := spatialEnvelopeCacheKey(q.info.TableName, detected.IDColumn, detected.EnvelopeColumn)
	cache, err := manager.GetOrBuild(ctx, cacheKey, q.info.ObjectCount, func(
		buildCtx context.Context,
		buffer *envelopeCacheBuildBuffer,
	) error {
		return q.buildEnvelopeEntries(buildCtx, detected, buffer)
	})
	if err != nil {
		if stderrors.Is(err, errEnvelopeCacheBudgetExceeded) {
			return nil, false, "", spatialQueryFailure(
				types.SpatialQueryReasonEnvelopeCacheBudgetExceeded,
				udbxerrors.ConstraintError("spatial query envelope cache exceeds the configured resource budget", err),
			)
		}
		return nil, false, "", err
	}

	ids, hasMore, err := cache.CandidateIDs(options.Bounds, options.Limit)
	return ids, hasMore, types.SpatialQueryStrategyEnvelopeCache, err
}

func spatialEnvelopeCacheKey(tableName, idColumn, envelopeColumn string) string {
	return tableName + "\x00" + idColumn + "\x00" + envelopeColumn
}

func (q *SpatialQuerier) buildEnvelopeEntries(
	ctx context.Context,
	detected *detectedSpatialCapability,
	buffer *envelopeCacheBuildBuffer,
) error {
	return q.scanSpatialEnvelopeRows(ctx, detected, func(entry *envelopeEntry) error {
		if entry == nil {
			return buffer.SkipRow()
		}
		return buffer.Append(*entry)
	})
}

func (q *SpatialQuerier) scanSpatialEnvelopeRows(
	ctx context.Context,
	detected *detectedSpatialCapability,
	consume func(*envelopeEntry) error,
) (returnErr error) {
	quotedTable, err := sqliteutil.QuoteIdentifier(q.info.TableName)
	if err != nil {
		return udbxerrors.IOError("failed to quote dataset table name", err)
	}
	quotedID, err := sqliteutil.QuoteIdentifier(detected.IDColumn)
	if err != nil {
		return udbxerrors.IOError("failed to quote feature ID column", err)
	}
	quotedEnvelope, err := sqliteutil.QuoteIdentifier(detected.EnvelopeColumn)
	if err != nil {
		return udbxerrors.IOError("failed to quote envelope column", err)
	}

	nullablePayload := q.info.Kind == types.DatasetKindText || q.info.Kind == types.DatasetKindCAD
	query := fmt.Sprintf("SELECT %s, substr(%s, 1, %d)", quotedID, quotedEnvelope, codec.GaiaHeaderLength)
	if nullablePayload {
		quotedPayload, err := sqliteutil.QuoteIdentifier(detected.PayloadColumn)
		if err != nil {
			return udbxerrors.IOError("failed to quote payload column", err)
		}
		query += ", " + quotedPayload + " IS NOT NULL"
	}
	query += " FROM " + quotedTable + " ORDER BY " + quotedID
	rows, err := q.db.QueryContext(ctx, query)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return udbxerrors.IOError("failed to query GAIA envelope headers", err)
	}
	defer func() {
		if closeErr := rows.Close(); returnErr == nil && closeErr != nil {
			returnErr = udbxerrors.IOError("failed to close GAIA envelope rows", closeErr)
		}
	}()

	scannedRows := 0
	for rows.Next() {
		if scannedRows%256 == 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
		}
		var idValue interface{}
		var headerValue interface{}
		payloadPresent := 1
		var scanErr error
		if nullablePayload {
			scanErr = rows.Scan(&idValue, &headerValue, &payloadPresent)
		} else {
			scanErr = rows.Scan(&idValue, &headerValue)
		}
		if scanErr != nil {
			return udbxerrors.IOError("failed to scan GAIA envelope row", scanErr)
		}
		id, err := spatialEnvelopeID(idValue)
		if err != nil {
			return err
		}
		if nullablePayload && headerValue == nil {
			if payloadPresent != 0 {
				return spatialQueryFailure(
					types.SpatialQueryReasonSpatialIndexUnavailable,
					udbxerrors.UnsupportedError("spatial payload is missing its SmIndexKey envelope"),
				)
			}
			if err := consume(nil); err != nil {
				return err
			}
			scannedRows++
			continue
		}
		if nullablePayload && payloadPresent == 0 {
			return newSpatialGeometryError("SmIndexKey envelope exists without a spatial payload")
		}
		header, ok := headerValue.([]byte)
		if !ok {
			return newSpatialGeometryError("GAIA envelope header is not a BLOB")
		}
		envelope, err := codec.ReadGaiaEnvelope(header)
		if err != nil {
			return &spatialGeometryError{cause: udbxerrors.FormatError("failed to read GAIA envelope", err)}
		}
		entry := envelopeEntry{
			ID: id, MinX: envelope.MinX, MinY: envelope.MinY,
			MaxX: envelope.MaxX, MaxY: envelope.MaxY,
		}
		if err := consume(&entry); err != nil {
			return err
		}
		scannedRows++
	}
	if err := rows.Err(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		return udbxerrors.IOError("error iterating GAIA envelope rows", err)
	}
	return nil
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

func mapSpatialQueryExecutionError(ctx context.Context, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return spatialQueryFailure(types.SpatialQueryReasonQueryTimeout, ctxErr)
	}
	if stderrors.Is(err, context.Canceled) || stderrors.Is(err, context.DeadlineExceeded) {
		return spatialQueryFailure(types.SpatialQueryReasonQueryTimeout, err)
	}
	if stderrors.Is(err, errEnvelopeCacheClosed) {
		return udbxerrors.IOError("spatial query cache manager is closed", err)
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
	manager *EnvelopeCacheManager,
) ([]int, bool, error) {
	if q.info.Kind == types.DatasetKindText || q.info.Kind == types.DatasetKindCAD {
		cacheKey := spatialEnvelopeCacheKey(q.info.TableName, detected.IDColumn, detected.EnvelopeColumn)
		if err := manager.validateEnvelopeIntegrity(ctx, cacheKey, func(validateCtx context.Context) error {
			return q.scanSpatialEnvelopeRows(validateCtx, detected, func(*envelopeEntry) error { return nil })
		}); err != nil {
			return nil, false, err
		}
	}
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
