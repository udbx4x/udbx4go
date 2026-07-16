package dataset

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/codec"
	"github.com/udbx4x/udbx4go/internal/sqliteutil"
	"github.com/udbx4x/udbx4go/internal/system"
	udbxerrors "github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestSpatialQueryRTreeUsesPhysicalRowIDAndStableFeatureID(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, `县级"区划`, `对象"编号`, `空间"对象`)
	defer db.Close()

	insertSpatialPoint(t, db, querier, 3, 10, 10, "edge-max", nil)
	insertSpatialPoint(t, db, querier, 1, 0, 0, "edge-min", nil)
	insertSpatialPoint(t, db, querier, 2, 5, 5, "center", nil)
	insertSpatialPoint(t, db, querier, 99, 50, 50, "outside", nil)

	result, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
		Limit:  2,
	})
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2}, spatialFeatureIDs(result.Features))
	assert.Equal(t, "edge-min", result.Features[0].Attributes["label"])
	assert.Equal(t, types.SpatialQueryStrategyRTree, result.Strategy)
	assert.Equal(t, types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10}, result.QueriedBounds)
	assert.True(t, result.HasMore)
	assert.Empty(t, result.DegradedReason)
}

func TestSpatialQueryRTreeIncludesBoundaryContacts(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "points", "FeatureID", "Geometry")
	defer db.Close()
	insertSpatialPoint(t, db, querier, 7, 10, 5, "touches", nil)
	insertSpatialPoint(t, db, querier, 8, 10.01, 5, "outside", nil)

	result, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
		Limit:  10,
	})
	require.NoError(t, err)
	assert.Equal(t, []int{7}, spatialFeatureIDs(result.Features))
	assert.False(t, result.HasMore)
}

func TestSpatialQueryRequiredIDsAppendInNormalizedInputOrder(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "points", "FeatureID", "Geometry")
	defer db.Close()
	insertSpatialPoint(t, db, querier, 3, 3, 3, "third", nil)
	insertSpatialPoint(t, db, querier, 1, 1, 1, "first", nil)
	insertSpatialPoint(t, db, querier, 2, 2, 2, "second", nil)
	insertSpatialPoint(t, db, querier, 99, 50, 50, "required", nil)
	insertSpatialPoint(t, db, querier, 50, 60, 60, "second-required", nil)

	result, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds:      types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
		Limit:       2,
		RequiredIDs: []int{99, 50, 2, 99},
	})
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 99, 50}, spatialFeatureIDs(result.Features))
	assert.True(t, result.HasMore)
}

func TestInitialCandidateCapacityIsBounded(t *testing.T) {
	assert.Equal(t, 1, initialCandidateCapacity(0))
	assert.Equal(t, 11, initialCandidateCapacity(10))
	assert.Equal(t, 1024, initialCandidateCapacity(math.MaxInt-1))
}

func TestSpatialQueryHugeLimitDoesNotPreallocateRequestedCapacity(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "empty_points", "FeatureID", "Geometry")
	defer db.Close()

	result, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{},
		Limit:  math.MaxInt - 1,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Features)
	assert.False(t, result.HasMore)
}

func TestSpatialQueryEmptyCandidatesReturnsEmptyResult(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "empty_points", "FeatureID", "Geometry")
	defer db.Close()

	result, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 1, MinY: 1, MaxX: 2, MaxY: 2},
		Limit:  10,
	})
	require.NoError(t, err)
	assert.Empty(t, result.Features)
	assert.False(t, result.HasMore)
}

func TestSpatialQueryLoadsMoreThanOneFeatureBatchInRequiredOrder(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "batch_points", "FeatureID", "Geometry")
	defer db.Close()

	requiredIDs := make([]int, 0, 600)
	for id := 600; id >= 1; id-- {
		insertSpatialPoint(t, db, querier, id, float64(1000+id), float64(1000+id), fmt.Sprintf("feature-%d", id), nil)
		requiredIDs = append(requiredIDs, id)
	}

	result, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds:      types.BoundingBox{},
		Limit:       1,
		RequiredIDs: requiredIDs,
	})
	require.NoError(t, err)
	require.Len(t, result.Features, 600)
	assert.Equal(t, requiredIDs, spatialFeatureIDs(result.Features))
	assert.Equal(t, "feature-600", result.Features[0].Attributes["label"])
	assert.Equal(t, "feature-1", result.Features[599].Attributes["label"])
	assert.False(t, result.HasMore)
}

func TestSpatialQueryRequiredIDsIgnoreMissingFeatures(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "points", "FeatureID", "Geometry")
	defer db.Close()
	insertSpatialPoint(t, db, querier, 1, 1, 1, "first", nil)

	result, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds:      types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
		Limit:       10,
		RequiredIDs: []int{404},
	})
	require.NoError(t, err)
	assert.Equal(t, []int{1}, spatialFeatureIDs(result.Features))
	assert.False(t, result.HasMore)
}

func TestSpatialQueryCorruptGeometryReturnsFormatError(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "points", "FeatureID", "Geometry")
	defer db.Close()
	insertSpatialPoint(t, db, querier, 1, 1, 1, "broken", []byte{0x00, 0x01})

	_, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
		Limit:  10,
	})
	require.Error(t, err)
	reason, ok := udbxerrors.SpatialQueryReasonOf(err)
	require.True(t, ok)
	assert.Equal(t, types.SpatialQueryReasonCorruptGeometry, reason)
	var udbxErr udbxerrors.UdbxError
	require.True(t, stderrors.As(err, &udbxErr))
	assert.Equal(t, udbxerrors.CodeFormatError, udbxErr.Code())
}

func TestSpatialQueryMalformedGeometryColumnReturnsCorruptGeometry(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "points", "FeatureID", "Geometry")
	defer db.Close()
	insertSpatialPoint(t, db, querier, 1, 1, 1, "wrong-type", nil)
	_, err := db.Exec(`UPDATE "points" SET "Geometry" = ? WHERE "FeatureID" = ?`, "not-a-blob", 1)
	require.NoError(t, err)

	_, err = querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
		Limit:  10,
	})
	assertSpatialQueryError(t, err, types.SpatialQueryReasonCorruptGeometry, udbxerrors.CodeFormatError)
}

func TestSpatialQueryMalformedCandidateIDIsFormatErrorWithoutSpatialReason(t *testing.T) {
	db, querier := createSpatialQueryFixtureWithIDType(t, "points", "FeatureID", "TEXT", "Geometry")
	defer db.Close()
	insertSpatialRow(t, db, querier, 1001, "not-an-integer", 1, 1, "bad-candidate", nil)

	_, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
		Limit:  10,
	})
	require.Error(t, err)
	assert.True(t, udbxerrors.IsFormatError(err))
	_, hasReason := udbxerrors.SpatialQueryReasonOf(err)
	assert.False(t, hasReason)
}

func TestSpatialQueryMalformedFeatureIDIsFormatErrorWithoutSpatialReason(t *testing.T) {
	db, querier := createSpatialQueryFixtureWithIDType(t, "points", "FeatureID", "TEXT", "Geometry")
	defer db.Close()
	insertSpatialRow(t, db, querier, 1001, "1", 50, 50, "bad-feature", nil)

	_, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds:      types.BoundingBox{},
		Limit:       10,
		RequiredIDs: []int{1},
	})
	require.Error(t, err)
	assert.True(t, udbxerrors.IsFormatError(err))
	_, hasReason := udbxerrors.SpatialQueryReasonOf(err)
	assert.False(t, hasReason)
}

func TestSpatialQueryEnvelopeCacheFiltersWithoutDecodingUnmatchedGeometry(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "points", "FeatureID", "Geometry")
	defer db.Close()
	insertSpatialPoint(t, db, querier, 3, 10, 10, "edge-max", nil)
	insertSpatialPoint(t, db, querier, 1, 0, 0, "edge-min", nil)
	insertSpatialPoint(t, db, querier, 2, 5, 5, "center", nil)
	insertSpatialPoint(t, db, querier, 99, 50, 50, "required", nil)
	insertSpatialPoint(t, db, querier, 50, 60, 60, "second-required", nil)
	insertSpatialPoint(t, db, querier, 88, 70, 70, "unmatched-corrupt-body", codec.WriteGaiaHeader(4326, [4]float64{70, 70, 70, 70}, codec.GeoTypePoint))
	removeSpatialRTree(t, db, querier)
	setSpatialObjectCount(querier, 6)
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 10), testEnvelopeCacheRSSCharge(t, 20))

	result, err := querier.QueryWithEnvelopeCache(context.Background(), types.SpatialQueryOptions{
		Bounds:      types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
		Limit:       2,
		RequiredIDs: []int{99, 50, 2, 99},
	}, manager)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 99, 50}, spatialFeatureIDs(result.Features))
	assert.Equal(t, types.SpatialQueryStrategyEnvelopeCache, result.Strategy)
	assert.True(t, result.HasMore)
	assert.Empty(t, result.DegradedReason)
	assert.Equal(t, 1, manager.EntryCount())

	again, err := querier.QueryWithEnvelopeCache(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
		Limit:  10,
	}, manager)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, spatialFeatureIDs(again.Features))
	assert.Equal(t, 1, manager.EntryCount(), "subsequent queries must reuse the complete cache")
}

func TestSpatialQueryAcceptsCaseDifferencesInSQLiteSpatialMetadata(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "MixedCasePoints", "SmID", "SmGeometry")
	defer db.Close()
	insertSpatialPoint(t, db, querier, 1, 1, 1, "point", nil)

	_, err := db.Exec(`UPDATE geometry_columns
		SET f_table_name = lower(f_table_name), f_geometry_column = lower(f_geometry_column)
		WHERE f_table_name = ?`, querier.info.TableName)
	require.NoError(t, err)

	result, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 2, MaxY: 2},
		Limit:  10,
	})
	require.NoError(t, err)
	assert.Equal(t, []int{1}, spatialFeatureIDs(result.Features))
	assert.Equal(t, types.SpatialQueryStrategyRTree, result.Strategy)
}

func TestSpatialQueryBoundedSampleOnlyOnEnvelopeBudgetRejection(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "points", "FeatureID", "Geometry")
	defer db.Close()
	insertSpatialPoint(t, db, querier, 30, 30, 30, "thirty", nil)
	insertSpatialPoint(t, db, querier, 10, 10, 10, "ten", nil)
	insertSpatialPoint(t, db, querier, 20, 20, 20, "twenty", nil)
	insertSpatialPoint(t, db, querier, 99, 99, 99, "required", nil)
	removeSpatialRTree(t, db, querier)
	setSpatialObjectCount(querier, 4)
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 3), testEnvelopeCacheRSSCharge(t, 6))

	result, err := querier.QueryWithEnvelopeCache(context.Background(), types.SpatialQueryOptions{
		Bounds:      types.BoundingBox{MinX: -100, MinY: -100, MaxX: -50, MaxY: -50},
		Limit:       2,
		RequiredIDs: []int{99, 30, 99},
	}, manager)

	require.NoError(t, err)
	assert.Equal(t, []int{10, 20, 99, 30}, spatialFeatureIDs(result.Features))
	assert.Equal(t, types.SpatialQueryStrategyBoundedSample, result.Strategy)
	assert.Equal(t, types.SpatialQueryReasonEnvelopeCacheBudgetExceeded, result.DegradedReason)
	assert.True(t, result.HasMore, "hasMore describes the ordinary objectCount/limit sample only")
	assert.Zero(t, manager.EntryCount())
}

func TestEnvelopeCacheSQLiteBuildStopsBeforeUnderreportedObjectCountExceedsBudget(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "points", "FeatureID", "Geometry")
	defer db.Close()
	for id := 1; id <= 5; id++ {
		insertSpatialPoint(t, db, querier, id, float64(id), float64(id), fmt.Sprintf("point-%d", id), nil)
	}
	insertSpatialPoint(t, db, querier, 6, 6, 6, "corrupt-after-budget", []byte{0x00, 0x01})
	removeSpatialRTree(t, db, querier)
	setSpatialObjectCount(querier, 1)
	manager := newTestEnvelopeCacheManager(t, testEnvelopeCacheRSSCharge(t, 4), testEnvelopeCacheRSSCharge(t, 4))
	idColumn, geometryColumn, err := querier.detectEnvelopeColumns(context.Background())
	require.NoError(t, err)
	detected := &detectedSpatialCapability{IDColumn: idColumn, GeometryColumn: geometryColumn}

	cache, err := manager.GetOrBuild(context.Background(), "points", 1, func(
		ctx context.Context,
		buffer *envelopeCacheBuildBuffer,
	) error {
		return querier.buildEnvelopeEntries(ctx, detected, buffer)
	})

	assert.Nil(t, cache)
	assert.ErrorIs(t, err, errEnvelopeCacheBudgetExceeded)
	assert.Zero(t, manager.TotalBytes())
	assert.Zero(t, manager.ReservedBytes())
	assert.Zero(t, manager.EntryCount())
}

func TestSpatialQueryEnvelopeCorruptHeaderReturnsCorruptGeometry(t *testing.T) {
	db, info, record := createSpatialCapabilityFixture(t, types.DatasetKindPoint, "points", "SmGeometry")
	defer db.Close()
	_, err := db.Exec(`INSERT INTO "points" (SmID, SmGeometry) VALUES (?, ?)`, 1, []byte{0x00, 0x01})
	require.NoError(t, err)
	info.ObjectCount = 1
	record.SmObjectCount = 1

	_, err = NewSpatialQuerier(db, info, record).Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{},
		Limit:  1,
	})
	assertSpatialQueryError(t, err, types.SpatialQueryReasonCorruptGeometry, udbxerrors.CodeFormatError)
}

func TestSpatialQueryEnvelopeCorruptFullGeometryReturnsCorruptGeometry(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "points", "FeatureID", "Geometry")
	defer db.Close()
	insertSpatialPoint(t, db, querier, 1, 1, 1, "broken-body", codec.WriteGaiaHeader(4326, [4]float64{1, 1, 1, 1}, codec.GeoTypePoint))
	removeSpatialRTree(t, db, querier)
	setSpatialObjectCount(querier, 1)

	_, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 2, MaxY: 2},
		Limit:  1,
	})
	assertSpatialQueryError(t, err, types.SpatialQueryReasonCorruptGeometry, udbxerrors.CodeFormatError)
}

func TestSpatialQueryEnvelopeBuildTimeoutDoesNotSample(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "points", "FeatureID", "Geometry")
	defer db.Close()
	insertSpatialPoint(t, db, querier, 1, 1, 1, "point", nil)
	removeSpatialRTree(t, db, querier)
	setSpatialObjectCount(querier, 1)
	manager, err := NewEnvelopeCacheManager(types.SpatialQueryPolicy{
		MaxDatasetCacheBytes: testEnvelopeCacheRSSCharge(t, 2),
		MaxTotalCacheBytes:   testEnvelopeCacheRSSCharge(t, 4),
		BuildTimeout:         10 * time.Millisecond,
	})
	require.NoError(t, err)
	defer manager.Close()
	manager.testHooks = &envelopeCacheManagerTestHooks{
		beforePublish: func(ctx context.Context) { <-ctx.Done() },
	}

	_, err = querier.QueryWithEnvelopeCache(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{},
		Limit:  1,
	}, manager)
	assertSpatialQueryError(t, err, types.SpatialQueryReasonQueryTimeout, udbxerrors.CodeUdbxError)
}

func TestSpatialQueryTypedNilCacheManagerUsesDefaultPolicy(t *testing.T) {
	db, info, record := createSpatialCapabilityFixture(t, types.DatasetKindPoint, "points", "SmGeometry")
	defer db.Close()
	var manager *EnvelopeCacheManager

	result, err := NewSpatialQuerier(db, info, record).QueryWithEnvelopeCache(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{},
		Limit:  1,
	}, manager)

	require.NoError(t, err)
	assert.Equal(t, types.SpatialQueryStrategyEnvelopeCache, result.Strategy)
	assert.Empty(t, result.Features)
}

func TestSpatialQueryRejectsUnsupportedDatasetKinds(t *testing.T) {
	for _, kind := range []types.DatasetKind{types.DatasetKindTabular, types.DatasetKindText, types.DatasetKindCAD} {
		t.Run(kind.String(), func(t *testing.T) {
			db, info, record := createSpatialCapabilityFixture(t, kind, "features", "SmGeometry")
			defer db.Close()

			_, err := NewSpatialQuerier(db, info, record).Query(context.Background(), types.SpatialQueryOptions{
				Bounds: types.BoundingBox{},
				Limit:  1,
			})
			assertSpatialQueryError(t, err, types.SpatialQueryReasonUnsupportedDatasetKind, udbxerrors.CodeUnsupported)
		})
	}
}

func TestSpatialQueryRejectsInvalidOptionsBeforeSQL(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "points", "FeatureID", "Geometry")
	defer db.Close()

	_, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 10, MaxX: 1},
		Limit:  1,
	})
	assertSpatialQueryError(t, err, types.SpatialQueryReasonInvalidViewport, udbxerrors.CodeConstraint)
}

func TestSpatialQueryContextCancellationReturnsTimeoutReason(t *testing.T) {
	db, querier := createSpatialQueryFixture(t, "points", "FeatureID", "Geometry")
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := querier.Query(ctx, types.SpatialQueryOptions{
		Bounds: types.BoundingBox{},
		Limit:  math.MaxInt - 1,
	})
	assertSpatialQueryError(t, err, types.SpatialQueryReasonQueryTimeout, udbxerrors.CodeUdbxError)
}

func createSpatialQueryFixture(t *testing.T, tableName, idColumn, geometryColumn string) (*sql.DB, *SpatialQuerier) {
	return createSpatialQueryFixtureWithIDType(t, tableName, idColumn, "INTEGER", geometryColumn)
}

func createSpatialQueryFixtureWithIDType(
	t *testing.T,
	tableName string,
	idColumn string,
	idType string,
	geometryColumn string,
) (*sql.DB, *SpatialQuerier) {
	t.Helper()
	db := setupTestDB(t)

	quotedTable := mustQuoteSpatialIdentifier(t, tableName)
	quotedID := mustQuoteSpatialIdentifier(t, idColumn)
	quotedGeometry := mustQuoteSpatialIdentifier(t, geometryColumn)
	_, err := db.Exec(fmt.Sprintf(
		"CREATE TABLE %s (%s %s NOT NULL UNIQUE, %s BLOB, %s TEXT)",
		quotedTable,
		quotedID,
		idType,
		quotedGeometry,
		mustQuoteSpatialIdentifier(t, "label"),
	))
	require.NoError(t, err)

	record := &system.SmRegisterRecord{
		SmDatasetID:   1,
		SmDatasetName: "fixture",
		SmTableName:   tableName,
		SmDatasetType: int(types.DatasetKindPoint),
		SmIDColName:   sql.NullString{String: idColumn, Valid: true},
		SmGeoColName:  sql.NullString{String: geometryColumn, Valid: true},
		SmSRID:        sql.NullInt32{Int32: 4326, Valid: true},
	}
	require.NoError(t, system.NewGeometryColumnsDao(db).Insert(&system.GeometryColumnsRecord{
		FTableName:          tableName,
		FGeometryColumn:     geometryColumn,
		GeometryType:        types.DatasetKindPoint.GeometryType(),
		CoordDimension:      2,
		SRID:                4326,
		SpatialIndexEnabled: 1,
	}))
	createSpatialRTree(t, db, tableName, geometryColumn, "pkid, xmin, xmax, ymin, ymax")

	return db, NewSpatialQuerier(db, record.ToDatasetInfo(), record)
}

func insertSpatialPoint(
	t *testing.T,
	db *sql.DB,
	querier *SpatialQuerier,
	id int,
	x float64,
	y float64,
	label string,
	geometryOverride []byte,
) {
	t.Helper()
	insertSpatialRow(t, db, querier, int64(1000+id), id, x, y, label, geometryOverride)
}

func insertSpatialRow(
	t *testing.T,
	db *sql.DB,
	querier *SpatialQuerier,
	rowID int64,
	id interface{},
	x float64,
	y float64,
	label string,
	geometryOverride []byte,
) {
	t.Helper()
	geometry := geometryOverride
	if geometry == nil {
		var err error
		geometry, err = codec.NewGaiaGeometryCodec().Encode(&types.PointGeometry{
			Type:        "Point",
			Coordinates: []float64{x, y},
		}, 4326)
		require.NoError(t, err)
	}

	_, err := db.Exec(fmt.Sprintf(
		"INSERT INTO %s (rowid, %s, %s, %s) VALUES (?, ?, ?, ?)",
		mustQuoteSpatialIdentifier(t, querier.info.TableName),
		mustQuoteSpatialIdentifier(t, registeredIDColumn(querier.record)),
		mustQuoteSpatialIdentifier(t, registeredGeometryColumn(querier.record)),
		mustQuoteSpatialIdentifier(t, "label"),
	), rowID, id, geometry, label)
	require.NoError(t, err)

	_, err = db.Exec(fmt.Sprintf(
		"INSERT INTO %s (pkid, xmin, xmax, ymin, ymax) VALUES (?, ?, ?, ?, ?)",
		mustQuoteSpatialIdentifier(t, spatialRTreeName(querier.info.TableName, registeredGeometryColumn(querier.record))),
	), rowID, x, x, y, y)
	require.NoError(t, err)
}

func spatialFeatureIDs(features []*types.Feature) []int {
	ids := make([]int, len(features))
	for i, feature := range features {
		ids[i] = feature.ID
	}
	return ids
}

func assertSpatialQueryError(
	t *testing.T,
	err error,
	wantReason types.SpatialQueryReason,
	wantCode string,
) {
	t.Helper()
	require.Error(t, err)
	reason, ok := udbxerrors.SpatialQueryReasonOf(err)
	require.True(t, ok)
	assert.Equal(t, wantReason, reason)
	var udbxErr udbxerrors.UdbxError
	require.True(t, stderrors.As(err, &udbxErr))
	assert.Equal(t, wantCode, udbxErr.Code())
}

func mustQuoteSpatialIdentifier(t *testing.T, name string) string {
	t.Helper()
	quoted, err := sqliteutil.QuoteIdentifier(name)
	require.NoError(t, err)
	return quoted
}

func removeSpatialRTree(t *testing.T, db *sql.DB, querier *SpatialQuerier) {
	t.Helper()
	_, err := db.Exec("DROP TABLE " + mustQuoteSpatialIdentifier(t, spatialRTreeName(querier.info.TableName, registeredGeometryColumn(querier.record))))
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE geometry_columns SET spatial_index_enabled = 0 WHERE f_table_name = ?`, querier.info.TableName)
	require.NoError(t, err)
}

func setSpatialObjectCount(querier *SpatialQuerier, count int) {
	querier.info.ObjectCount = count
	querier.record.SmObjectCount = count
}
