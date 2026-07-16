package dataset

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"
	"testing"

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

	result, err := querier.Query(context.Background(), types.SpatialQueryOptions{
		Bounds:      types.BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10},
		Limit:       2,
		RequiredIDs: []int{99, 2, 99},
	})
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 99}, spatialFeatureIDs(result.Features))
	assert.True(t, result.HasMore)
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

func TestSpatialQueryReturnsUnavailableWithoutRTree(t *testing.T) {
	db, info, record := createSpatialCapabilityFixture(t, types.DatasetKindPoint, "points", "SmGeometry")
	defer db.Close()

	_, err := NewSpatialQuerier(db, info, record).Query(context.Background(), types.SpatialQueryOptions{
		Bounds: types.BoundingBox{},
		Limit:  1,
	})
	assertSpatialQueryError(t, err, types.SpatialQueryReasonSpatialIndexUnavailable, udbxerrors.CodeUnsupported)
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
		Limit:  1,
	})
	assertSpatialQueryError(t, err, types.SpatialQueryReasonQueryTimeout, udbxerrors.CodeUdbxError)
}

func createSpatialQueryFixture(t *testing.T, tableName, idColumn, geometryColumn string) (*sql.DB, *SpatialQuerier) {
	t.Helper()
	db := setupTestDB(t)

	quotedTable := mustQuoteSpatialIdentifier(t, tableName)
	quotedID := mustQuoteSpatialIdentifier(t, idColumn)
	quotedGeometry := mustQuoteSpatialIdentifier(t, geometryColumn)
	_, err := db.Exec(fmt.Sprintf(
		"CREATE TABLE %s (%s INTEGER NOT NULL UNIQUE, %s BLOB, %s TEXT)",
		quotedTable,
		quotedID,
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
	geometry := geometryOverride
	if geometry == nil {
		var err error
		geometry, err = codec.NewGaiaGeometryCodec().Encode(&types.PointGeometry{
			Type:        "Point",
			Coordinates: []float64{x, y},
		}, 4326)
		require.NoError(t, err)
	}

	rowID := int64(1000 + id)
	_, err := db.Exec(fmt.Sprintf(
		"INSERT INTO %s (rowid, %s, %s, %s) VALUES (?, ?, ?, ?)",
		mustQuoteSpatialIdentifier(t, querier.info.TableName),
		mustQuoteSpatialIdentifier(t, registeredIDColumn(querier.record)),
		mustQuoteSpatialIdentifier(t, registeredGeometryColumn(querier.record)),
		mustQuoteSpatialIdentifier(t, "label"),
	), rowID, id, geometry, label)
	require.NoError(t, err)
	require.NotEqual(t, int64(id), rowID, "fixture must not rely on pkid equaling the feature ID")

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
