package dataset

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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
	assert.Equal(t, `空间"对象`, detected.EnvelopeColumn)
	assert.Equal(t, `空间"对象`, detected.PayloadColumn)
	assert.Equal(t, "SmID", detected.IDColumn)
}

func TestDetectSpatialCapabilityReportsExecutableFallback(t *testing.T) {
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

func TestDetectSpatialCapabilityRejectsInexecutableFallback(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord)
	}{
		{
			name: "geometry columns record missing",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				_, err := db.Exec("DELETE FROM geometry_columns WHERE f_table_name = ?", info.TableName)
				require.NoError(t, err)
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
			name: "physical geometry column missing",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				_, err := db.Exec(`ALTER TABLE "roads" RENAME TO "old_roads"`)
				require.NoError(t, err)
				_, err = db.Exec(`CREATE TABLE "roads" ("SmID" INTEGER PRIMARY KEY)`)
				require.NoError(t, err)
			},
		},
		{
			name: "physical id column missing when spatial index disabled",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				_, err := db.Exec(`ALTER TABLE "roads" RENAME TO "old_roads"`)
				require.NoError(t, err)
				_, err = db.Exec(`CREATE TABLE "roads" ("SmGeometry" BLOB)`)
				require.NoError(t, err)
				_, err = db.Exec("UPDATE geometry_columns SET spatial_index_enabled = 0 WHERE f_table_name = ?", info.TableName)
				require.NoError(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, info, record := createSpatialCapabilityFixture(t, types.DatasetKindPoint, "roads", "SmGeometry")
			defer db.Close()
			tt.setup(t, db, info, record)

			detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
			require.NoError(t, err)
			assert.Equal(t, &types.SpatialQueryCapability{
				Supported:         true,
				RTreeAvailable:    false,
				FallbackAvailable: false,
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
	assert.Equal(t, "actual_geometry", detected.EnvelopeColumn)
	assert.Equal(t, "actual_geometry", detected.PayloadColumn)
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

func TestDetectSpatialCapabilityUsesIndexKeyForTextAndCAD(t *testing.T) {
	for _, kind := range []types.DatasetKind{types.DatasetKindText, types.DatasetKindCAD} {
		t.Run(kind.String(), func(t *testing.T) {
			db, info, record := createTextCADSpatialCapabilityFixture(t, kind, "features_"+kind.String())
			defer db.Close()
			createSpatialRTree(t, db, info.TableName, "SmIndexKey", "pkid, xmin, xmax, ymin, ymax")

			detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
			require.NoError(t, err)
			assert.Equal(t, &types.SpatialQueryCapability{
				Supported:         true,
				RTreeAvailable:    true,
				FallbackAvailable: true,
			}, detected.Capability)
			assert.Equal(t, spatialRTreeName(info.TableName, "SmIndexKey"), detected.RTreeName)
			assert.Equal(t, "SmIndexKey", detected.EnvelopeColumn)
			assert.Equal(t, "SmGeometry", detected.PayloadColumn)
			assert.Equal(t, "SmID", detected.IDColumn)
		})
	}
}

func TestDetectSpatialCapabilityRejectsCADWithoutGeoTypeColumn(t *testing.T) {
	db, info, record := createTextCADSpatialCapabilityFixture(t, types.DatasetKindCAD, "features")
	defer db.Close()
	replaceTextCADSpatialTable(t, db, info.TableName, "SmID INTEGER PRIMARY KEY, SmGeometry BLOB, SmIndexKey POLYGON")

	detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
	require.NoError(t, err)
	assert.Equal(t, &types.SpatialQueryCapability{
		Supported:        true,
		DiagnosticReason: types.SpatialQueryReasonSpatialIndexUnavailable,
	}, detected.Capability)
}

func TestDetectSpatialCapabilityPreservesPhysicalCADGeoTypeColumn(t *testing.T) {
	db, info, record := createTextCADSpatialCapabilityFixture(t, types.DatasetKindCAD, "features")
	defer db.Close()
	_, err := db.Exec(`ALTER TABLE features RENAME COLUMN SmGeoType TO sMgEoTyPe`)
	require.NoError(t, err)

	detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "sMgEoTyPe", detected.CADTypeColumn)
}

func TestDetectSpatialCapabilityAcceptsTextAndCADGeometryRegistration(t *testing.T) {
	registrations := []sql.NullString{
		{},
		{String: "", Valid: true},
		{String: "smgeometry", Valid: true},
	}
	for _, kind := range []types.DatasetKind{types.DatasetKindText, types.DatasetKindCAD} {
		for index, registration := range registrations {
			t.Run(fmt.Sprintf("%s/registration-%d", kind.String(), index), func(t *testing.T) {
				db, info, record := createTextCADSpatialCapabilityFixture(t, kind, "features")
				defer db.Close()
				record.SmGeoColName = registration

				detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
				require.NoError(t, err)
				assert.True(t, detected.Capability.Supported)
				assert.False(t, detected.Capability.RTreeAvailable)
				assert.True(t, detected.Capability.FallbackAvailable)
				assert.Equal(t, "SmIndexKey", detected.EnvelopeColumn)
				assert.Equal(t, "SmGeometry", detected.PayloadColumn)
			})
		}
	}
}

func TestDetectSpatialCapabilityRejectsInvalidTextAndCADMetadata(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord)
	}{
		{
			name: "missing geometry columns record",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				_, err := db.Exec("DELETE FROM geometry_columns WHERE f_table_name = ?", info.TableName)
				require.NoError(t, err)
			},
		},
		{
			name: "multiple geometry columns records",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				require.NoError(t, system.NewGeometryColumnsDao(db).Insert(&system.GeometryColumnsRecord{
					FTableName:          info.TableName,
					FGeometryColumn:     "SmGeometry",
					GeometryType:        3,
					CoordDimension:      2,
					SRID:                4326,
					SpatialIndexEnabled: 0,
				}))
			},
		},
		{
			name: "geometry columns points at payload",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				updateSpatialGeometryMetadata(t, db, info.TableName, "f_geometry_column", "SmGeometry")
			},
		},
		{
			name: "registered payload column is invalid",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				record.SmGeoColName = sql.NullString{String: "SmIndexKey", Valid: true}
			},
		},
		{
			name: "wrong geometry type",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				updateSpatialGeometryMetadata(t, db, info.TableName, "geometry_type", 1)
			},
		},
		{
			name: "wrong coordinate dimension",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				updateSpatialGeometryMetadata(t, db, info.TableName, "coord_dimension", 4)
			},
		},
		{
			name: "missing id column",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				replaceTextCADSpatialTable(t, db, info.TableName, "SmGeometry BLOB, SmIndexKey POLYGON")
			},
		},
		{
			name: "missing envelope column",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				replaceTextCADSpatialTable(t, db, info.TableName, "SmID INTEGER PRIMARY KEY, SmGeometry BLOB")
			},
		},
		{
			name: "missing payload column",
			setup: func(t *testing.T, db *sql.DB, info *types.DatasetInfo, record *system.SmRegisterRecord) {
				replaceTextCADSpatialTable(t, db, info.TableName, "SmID INTEGER PRIMARY KEY, SmIndexKey POLYGON")
			},
		},
	}

	for _, kind := range []types.DatasetKind{types.DatasetKindText, types.DatasetKindCAD} {
		for _, tt := range tests {
			t.Run(kind.String()+"/"+tt.name, func(t *testing.T) {
				db, info, record := createTextCADSpatialCapabilityFixture(t, kind, "features")
				defer db.Close()
				tt.setup(t, db, info, record)

				detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
				require.NoError(t, err)
				assert.Equal(t, &types.SpatialQueryCapability{
					Supported:        true,
					DiagnosticReason: types.SpatialQueryReasonSpatialIndexUnavailable,
				}, detected.Capability)
			})
		}
	}
}

func TestDetectSpatialCapabilityRejectsText3DEnvelope(t *testing.T) {
	db, info, record := createTextCADSpatialCapabilityFixture(t, types.DatasetKindText, "features")
	defer db.Close()
	updateSpatialGeometryMetadata(t, db, info.TableName, "coord_dimension", 3)

	detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
	require.NoError(t, err)
	assert.Equal(t, &types.SpatialQueryCapability{
		Supported:        true,
		DiagnosticReason: types.SpatialQueryReasonSpatialIndexUnavailable,
	}, detected.Capability)
}

func TestDetectSpatialCapabilityAcceptsCAD2DOr3DEnvelopeAndIndependentSRID(t *testing.T) {
	for _, dimension := range []int{2, 3} {
		t.Run(fmt.Sprintf("dimension-%d", dimension), func(t *testing.T) {
			db, info, record := createTextCADSpatialCapabilityFixture(t, types.DatasetKindCAD, "features")
			defer db.Close()
			updateSpatialGeometryMetadata(t, db, info.TableName, "coord_dimension", dimension)
			updateSpatialGeometryMetadata(t, db, info.TableName, "srid", 3857)

			detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
			require.NoError(t, err)
			assert.True(t, detected.Capability.Supported)
			assert.True(t, detected.Capability.FallbackAvailable)
			assert.Equal(t, "SmIndexKey", detected.EnvelopeColumn)
		})
	}
}

func TestDetectSpatialCapabilityAcceptsTextEnvelopeWithIndependentSRID(t *testing.T) {
	db, info, record := createTextCADSpatialCapabilityFixture(t, types.DatasetKindText, "features")
	defer db.Close()
	updateSpatialGeometryMetadata(t, db, info.TableName, "srid", 3857)

	detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
	require.NoError(t, err)
	assert.True(t, detected.Capability.Supported)
	assert.True(t, detected.Capability.FallbackAvailable)
}

func TestDetectSpatialCapabilityPreservesPhysicalColumnAndRTreeNames(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	tableName := `县级"TextLayer`
	idColumn := `sMiD"编号`
	payloadColumn := "sMgEoMeTrY"
	envelopeColumn := "sMiNdExKeY"
	quotedTable, err := sqliteutil.QuoteIdentifier(tableName)
	require.NoError(t, err)
	quotedID, err := sqliteutil.QuoteIdentifier(idColumn)
	require.NoError(t, err)
	quotedPayload, err := sqliteutil.QuoteIdentifier(payloadColumn)
	require.NoError(t, err)
	quotedEnvelope, err := sqliteutil.QuoteIdentifier(envelopeColumn)
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf(
		"CREATE TABLE %s (%s INTEGER PRIMARY KEY, %s BLOB, %s POLYGON)",
		quotedTable, quotedID, quotedPayload, quotedEnvelope,
	))
	require.NoError(t, err)

	record := &system.SmRegisterRecord{
		SmDatasetID:   1,
		SmDatasetName: "fixture",
		SmTableName:   tableName,
		SmDatasetType: int(types.DatasetKindText),
		SmIDColName:   sql.NullString{String: `smid"编号`, Valid: true},
		SmGeoColName:  sql.NullString{String: "smgeometry", Valid: true},
		SmSRID:        sql.NullInt32{Int32: 4326, Valid: true},
	}
	require.NoError(t, system.NewGeometryColumnsDao(db).Insert(&system.GeometryColumnsRecord{
		FTableName:          `县级"textlayer`,
		FGeometryColumn:     "smindexkey",
		GeometryType:        3,
		CoordDimension:      2,
		SRID:                4326,
		SpatialIndexEnabled: 1,
	}))
	createSpatialRTree(t, db, tableName, envelopeColumn, "pkid, xmin, xmax, ymin, ymax")

	detected, err := NewSpatialQuerier(db, record.ToDatasetInfo(), record).detectCapability(context.Background())
	require.NoError(t, err)
	assert.Equal(t, idColumn, detected.IDColumn)
	assert.Equal(t, envelopeColumn, detected.EnvelopeColumn)
	assert.Equal(t, payloadColumn, detected.PayloadColumn)
	assert.Equal(t, spatialRTreeName(tableName, envelopeColumn), detected.RTreeName)
}

func TestDetectSpatialCapabilityReportsTextAndCADFallbackWithoutRTree(t *testing.T) {
	for _, kind := range []types.DatasetKind{types.DatasetKindText, types.DatasetKindCAD} {
		for _, spatialIndexEnabled := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/index-enabled-%d", kind.String(), spatialIndexEnabled), func(t *testing.T) {
				db, info, record := createTextCADSpatialCapabilityFixture(t, kind, "features")
				defer db.Close()
				updateSpatialGeometryMetadata(t, db, info.TableName, "spatial_index_enabled", spatialIndexEnabled)

				detected, err := NewSpatialQuerier(db, info, record).detectCapability(context.Background())
				require.NoError(t, err)
				assert.Equal(t, &types.SpatialQueryCapability{
					Supported:         true,
					FallbackAvailable: true,
					DiagnosticReason:  types.SpatialQueryReasonSpatialIndexUnavailable,
				}, detected.Capability)
				assert.Equal(t, "SmIndexKey", detected.EnvelopeColumn)
				assert.Equal(t, "SmGeometry", detected.PayloadColumn)
			})
		}
	}
}

func TestSpatialCapabilityAcceptsNewCadDatasetMetadata(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	_, record := createCadDataset(t, db)

	detected, err := NewSpatialQuerier(db, record.ToDatasetInfo(), record).detectCapability(context.Background())
	require.NoError(t, err)
	assert.Equal(t, &types.SpatialQueryCapability{
		Supported:         true,
		FallbackAvailable: true,
		DiagnosticReason:  types.SpatialQueryReasonSpatialIndexUnavailable,
	}, detected.Capability)
	assert.Equal(t, "SmID", detected.IDColumn)
	assert.Equal(t, "SmIndexKey", detected.EnvelopeColumn)
	assert.Equal(t, "SmGeometry", detected.PayloadColumn)
}

func TestSpatialCapabilityAcceptsRealSampleDataTextAndCADMetadataCasing(t *testing.T) {
	path := sampleDataSpatialCapabilityPath(t)
	absolutePath, err := filepath.Abs(path)
	require.NoError(t, err)
	dsn := url.URL{Scheme: "file", Path: absolutePath}
	query := dsn.Query()
	query.Set("mode", "ro")
	dsn.RawQuery = query.Encode()
	db, err := sql.Open("sqlite3", dsn.String())
	require.NoError(t, err)
	defer db.Close()

	records, err := system.NewSmRegisterDao(db).ListAll()
	require.NoError(t, err)
	found := make(map[types.DatasetKind]bool)
	for _, record := range records {
		kind := types.DatasetKind(record.SmDatasetType)
		if kind != types.DatasetKindText && kind != types.DatasetKindCAD {
			continue
		}
		found[kind] = true
		detected, err := NewSpatialQuerier(db, record.ToDatasetInfo(), record).detectCapability(context.Background())
		require.NoError(t, err)
		assert.True(t, detected.Capability.Supported)
		assert.False(t, detected.Capability.RTreeAvailable)
		assert.True(t, detected.Capability.FallbackAvailable)
		assert.Equal(t, "SmIndexKey", detected.EnvelopeColumn)
		assert.Equal(t, "SmGeometry", detected.PayloadColumn)
	}
	assert.True(t, found[types.DatasetKindText], "SampleData must include Text metadata")
	assert.True(t, found[types.DatasetKindCAD], "SampleData must include CAD metadata")
}

func TestDetectSpatialCapabilityRejectsUnsupportedKinds(t *testing.T) {
	for _, kind := range []types.DatasetKind{types.DatasetKindTabular, types.DatasetKind(999)} {
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

func createTextCADSpatialCapabilityFixture(
	t *testing.T,
	kind types.DatasetKind,
	tableName string,
) (*sql.DB, *types.DatasetInfo, *system.SmRegisterRecord) {
	t.Helper()
	db := setupTestDB(t)

	quotedTable, err := sqliteutil.QuoteIdentifier(tableName)
	require.NoError(t, err)
	columns := "SmID INTEGER PRIMARY KEY, SmGeometry BLOB, SmIndexKey POLYGON"
	if kind == types.DatasetKindCAD {
		columns = "SmID INTEGER PRIMARY KEY, SmGeoType INTEGER, SmGeometry BLOB, SmIndexKey POLYGON"
	}
	_, err = db.Exec(fmt.Sprintf("CREATE TABLE %s (%s)", quotedTable, columns))
	require.NoError(t, err)

	record := &system.SmRegisterRecord{
		SmDatasetID:   1,
		SmDatasetName: "fixture",
		SmTableName:   tableName,
		SmDatasetType: int(kind),
		SmIDColName:   sql.NullString{String: "SmID", Valid: true},
		SmGeoColName:  sql.NullString{String: "SmGeometry", Valid: true},
		SmSRID:        sql.NullInt32{Int32: 4326, Valid: true},
	}
	require.NoError(t, system.NewGeometryColumnsDao(db).Insert(&system.GeometryColumnsRecord{
		FTableName:          tableName,
		FGeometryColumn:     "SmIndexKey",
		GeometryType:        3,
		CoordDimension:      2,
		SRID:                4326,
		SpatialIndexEnabled: 1,
	}))

	return db, record.ToDatasetInfo(), record
}

func updateSpatialGeometryMetadata(t *testing.T, db *sql.DB, tableName, column string, value interface{}) {
	t.Helper()
	quotedColumn, err := sqliteutil.QuoteIdentifier(column)
	require.NoError(t, err)
	_, err = db.Exec("UPDATE geometry_columns SET "+quotedColumn+" = ? WHERE f_table_name = ?", value, tableName)
	require.NoError(t, err)
}

func replaceTextCADSpatialTable(t *testing.T, db *sql.DB, tableName, columns string) {
	t.Helper()
	quotedTable, err := sqliteutil.QuoteIdentifier(tableName)
	require.NoError(t, err)
	_, err = db.Exec("DROP TABLE " + quotedTable)
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf("CREATE TABLE %s (%s)", quotedTable, columns))
	require.NoError(t, err)
}

func sampleDataSpatialCapabilityPath(t *testing.T) string {
	t.Helper()
	path := os.Getenv("UDBX_SAMPLE_DATA_PATH")
	required := os.Getenv("UDBX_REAL_SAMPLES") == "1"
	if path == "" {
		if required {
			t.Fatal("UDBX_SAMPLE_DATA_PATH is required when UDBX_REAL_SAMPLES=1")
		}
		t.Skip("set UDBX_SAMPLE_DATA_PATH to run SampleData spatial tests")
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("UDBX_SAMPLE_DATA_PATH must be absolute: %s", path)
	}
	if _, err := os.Stat(path); err != nil {
		if required {
			require.NoError(t, err, "SampleData fixture is required when UDBX_REAL_SAMPLES=1")
		}
		t.Skipf("SampleData fixture is unavailable: %s", path)
	}
	return path
}

func createSpatialRTree(t *testing.T, db *sql.DB, tableName, geometryColumn, columns string) {
	t.Helper()
	quoted, err := sqliteutil.QuoteIdentifier(spatialRTreeName(tableName, geometryColumn))
	require.NoError(t, err)
	_, err = db.Exec(fmt.Sprintf("CREATE VIRTUAL TABLE %s USING rtree(%s)", quoted, columns))
	require.NoError(t, err)
}
