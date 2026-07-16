package dataset

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/sqliteutil"
	"github.com/udbx4x/udbx4go/internal/system"
	udbxerrors "github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestDetectSpatialCapabilityAcceptsValidRTree(t *testing.T) {
	db, info, record := createSpatialCapabilityFixture(t, types.DatasetKindPoint, `县级"区划`, `空间"对象`)
	defer db.Close()

	createSpatialRTree(t, db, info.TableName, record.SmGeoColName.String, "pkid, xmin, xmax, ymin, ymax, +payload")

	detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
	require.NoError(t, err)
	assert.Equal(t, &types.SpatialQueryCapability{
		Supported:         true,
		RTreeAvailable:    true,
		FallbackAvailable: true,
	}, detected.Capability)
	assert.Equal(t, `idx_县级"区划_空间"对象`, detected.RTreeName)
	assert.Equal(t, `空间"对象`, detected.GeometryColumn)
	assert.Equal(t, "SmID", detected.IDColumn)
}

func TestDetectSpatialCapabilityReportsUnavailableRTree(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord)
	}{
		{name: "declared but table missing"},
		{
			name: "ordinary table impersonates rtree",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				indexName := spatialRTreeName(info.TableName, record.SmGeoColName.String)
				quoted, err := sqliteutil.QuoteIdentifier(indexName)
				require.NoError(t, err)
				_, err = db.Exec(fmt.Sprintf("CREATE TABLE %s (pkid INTEGER, xmin REAL, xmax REAL, ymin REAL, ymax REAL)", quoted))
				require.NoError(t, err)
			},
		},
		{
			name: "non rtree virtual table",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				indexName := spatialRTreeName(info.TableName, record.SmGeoColName.String)
				quoted, err := sqliteutil.QuoteIdentifier(indexName)
				require.NoError(t, err)
				if _, err := db.Exec(fmt.Sprintf("CREATE VIRTUAL TABLE %s USING fts5(pkid, xmin, xmax, ymin, ymax)", quoted)); err != nil {
					t.Skipf("SQLite build does not provide FTS5: %v", err)
				}
			},
		},
		{
			name: "rtree missing semantic columns",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				createSpatialRTree(t, db, info.TableName, record.SmGeoColName.String, "pkid, xmin, xmax")
			},
		},
		{
			name: "geometry columns uses another physical table",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				_, err := db.Exec("UPDATE geometry_columns SET f_table_name = ? WHERE f_table_name = ?", "logical_name", info.TableName)
				require.NoError(t, err)
			},
		},
		{
			name: "registered geometry column disagrees",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				_, err := db.Exec("UPDATE geometry_columns SET f_geometry_column = ? WHERE f_table_name = ?", "other_geometry", info.TableName)
				require.NoError(t, err)
			},
		},
		{
			name: "multiple geometry column records",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				require.NoError(t, system.NewGeometryColumnsDao(db).Insert(&system.GeometryColumnsRecord{
					FTableName:          info.TableName,
					FGeometryColumn:     "other_geometry",
					GeometryType:        1,
					CoordDimension:      2,
					SRID:                4326,
					SpatialIndexEnabled: 1,
				}))
			},
		},
		{
			name: "spatial index declaration disabled",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				_, err := db.Exec("UPDATE geometry_columns SET spatial_index_enabled = 0 WHERE f_table_name = ?", info.TableName)
				require.NoError(t, err)
			},
		},
		{
			name: "spatial index declaration missing",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				_, err := db.Exec("UPDATE geometry_columns SET spatial_index_enabled = NULL WHERE f_table_name = ?", info.TableName)
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, info, record := createSpatialCapabilityFixture(t, types.DatasetKindPoint, "roads", "SmGeometry")
			defer db.Close()
			if tt.setup != nil {
				tt.setup(t, db, info, record)
			}

			detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
			require.NoError(t, err)
			assert.Equal(t, &types.SpatialQueryCapability{
				Supported:         true,
				RTreeAvailable:    false,
				FallbackAvailable: true,
				DiagnosticReason:  types.SpatialQueryReasonSpatialIndexUnavailable,
			}, detected.Capability)
		})
	}
}

func TestDetectSpatialCapabilityUsesGeometryColumnsWhenRegisteredColumnIsEmpty(t *testing.T) {
	db, info, record := createSpatialCapabilityFixture(t, types.DatasetKindPoint, "roads", "actual_geometry")
	defer db.Close()
	record.SmGeoColName = sql.NullString{}
	record.SmIDColName = sql.NullString{String: "", Valid: true}
	createSpatialRTree(t, db, info.TableName, "actual_geometry", "pkid, xmin, xmax, ymin, ymax")

	detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
	require.NoError(t, err)
	assert.True(t, detected.Capability.RTreeAvailable)
	assert.Equal(t, "actual_geometry", detected.GeometryColumn)
	assert.Equal(t, "SmID", detected.IDColumn)
}

func TestDetectSpatialCapabilityRejectsMissingPhysicalGeometryColumn(t *testing.T) {
	db, info, record := createSpatialCapabilityFixture(t, types.DatasetKindPoint, "roads", "actual_geometry")
	defer db.Close()
	record.SmGeoColName = sql.NullString{}
	_, err := db.Exec(`ALTER TABLE "roads" RENAME TO "old_roads"`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE "roads" ("SmID" INTEGER PRIMARY KEY)`)
	require.NoError(t, err)

	detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
	require.NoError(t, err)
	assert.False(t, detected.Capability.RTreeAvailable)
	assert.Equal(t, types.SpatialQueryReasonSpatialIndexUnavailable, detected.Capability.DiagnosticReason)
}

func TestDetectSpatialCapabilitySupportsVectorKinds(t *testing.T) {
	kinds := []types.DatasetKind{
		types.DatasetKindPoint,
		types.DatasetKindLine,
		types.DatasetKindRegion,
		types.DatasetKindPointZ,
		types.DatasetKindLineZ,
		types.DatasetKindRegionZ,
	}

	for _, kind := range kinds {
		t.Run(kind.String(), func(t *testing.T) {
			db, info, record := createSpatialCapabilityFixture(t, kind, "features", "SmGeometry")
			defer db.Close()
			createSpatialRTree(t, db, info.TableName, "SmGeometry", "pkid, xmin, xmax, ymin, ymax")

			detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
			require.NoError(t, err)
			assert.True(t, detected.Capability.Supported)
			assert.True(t, detected.Capability.RTreeAvailable)
		})
	}
}

func TestDetectSpatialCapabilityRejectsUnsupportedKinds(t *testing.T) {
	for _, kind := range []types.DatasetKind{types.DatasetKindTabular, types.DatasetKindText, types.DatasetKindCAD} {
		t.Run(kind.String(), func(t *testing.T) {
			db, info, record := createSpatialCapabilityFixture(t, kind, "features", "SmGeometry")
			defer db.Close()

			detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
			require.NoError(t, err)
			assert.Equal(t, &types.SpatialQueryCapability{
				Supported:        false,
				DiagnosticReason: types.SpatialQueryReasonUnsupportedDatasetKind,
			}, detected.Capability)
		})
	}
}

func TestDetectSpatialCapabilityPreservesMetadataIOErrors(t *testing.T) {
	db, info, record := createSpatialCapabilityFixture(t, types.DatasetKindPoint, "roads", "SmGeometry")
	defer db.Close()
	_, err := db.Exec("DROP TABLE geometry_columns")
	require.NoError(t, err)

	_, err = NewSpatialQuerier(db, info, record).detectCapability(context.Background())
	require.Error(t, err)
	assert.True(t, udbxerrors.IsIOError(err))
}

func createSpatialCapabilityFixture(
	t *testing.T,
	kind types.DatasetKind,
	tableName string,
	geometryColumn string,
) (*sql.DB, *types.DatasetInfo, *system.SmRegisterRecord) {
	t.Helper()
	db := setupTestDB(t)

	quotedTable, err := sqliteutil.QuoteIdentifier(tableName)
	require.NoError(t, err)
	quotedID, err := sqliteutil.QuoteIdentifier("SmID")
	require.NoError(t, err)
	quotedGeometry, err := sqliteutil.QuoteIdentifier(geometryColumn)
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf(
		"CREATE TABLE %s (%s INTEGER PRIMARY KEY, %s BLOB)",
		quotedTable,
		quotedID,
		quotedGeometry,
	))
	require.NoError(t, err)

	record := &system.SmRegisterRecord{
		SmDatasetID:   1,
		SmDatasetName: "fixture",
		SmTableName:   tableName,
		SmDatasetType: int(kind),
		SmIDColName:   sql.NullString{String: "SmID", Valid: true},
		SmGeoColName:  sql.NullString{String: geometryColumn, Valid: true},
	}
	require.NoError(t, system.NewGeometryColumnsDao(db).Insert(&system.GeometryColumnsRecord{
		FTableName:          tableName,
		FGeometryColumn:     geometryColumn,
		GeometryType:        kind.GeometryType(),
		CoordDimension:      kind.CoordDimension(),
		SRID:                4326,
		SpatialIndexEnabled: 1,
	}))

	return db, record.ToDatasetInfo(), record
}

func createSpatialRTree(t *testing.T, db *sql.DB, tableName, geometryColumn, columns string) {
	t.Helper()
	quoted, err := sqliteutil.QuoteIdentifier(spatialRTreeName(tableName, geometryColumn))
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf("CREATE VIRTUAL TABLE %s USING rtree(%s)", quoted, columns))
	require.NoError(t, err)
}
