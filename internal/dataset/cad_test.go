package dataset

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/system"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func createCadDataset(t *testing.T, db *sql.DB) (*CadDataset, *system.SmRegisterRecord) {
	// Task 5 must replace this DDL with DataSource.CreateCadDataset integration
	// coverage and remove DEFAULT 0 once CadDataset writes SmGeoType/SmIndexKey.
	_, err := db.Exec(`
		CREATE TABLE cad_layers (
			SmID INTEGER PRIMARY KEY,
			SmUserID INTEGER DEFAULT 0,
			SmGeoType INTEGER NOT NULL DEFAULT 0,
			SmGeometry BLOB,
			SmIndexKey POLYGON,
			name TEXT NOT NULL,
			level INTEGER
		)
	`)
	require.NoError(t, err)

	registerDao := system.NewSmRegisterDao(db)
	record := &system.SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindCAD),
		SmDatasetName: "cad_layers",
		SmTableName:   "cad_layers",
		SmObjectCount: 0,
		SmIDColName:   sql.NullString{String: "SmID", Valid: true},
		SmGeoColName:  sql.NullString{String: "SmGeometry", Valid: true},
		SmSRID:        sql.NullInt32{Int32: 0, Valid: true},
		SmIndexType:   sql.NullInt32{Int32: 0, Valid: true},
	}
	err = registerDao.Insert(record)
	require.NoError(t, err)
	require.NoError(t, system.NewGeometryColumnsDao(db).Insert(&system.GeometryColumnsRecord{
		FTableName:          "cad_layers",
		FGeometryColumn:     "SmIndexKey",
		GeometryType:        3,
		CoordDimension:      2,
		SRID:                0,
		SpatialIndexEnabled: 0,
	}))

	fieldInfoDao := system.NewSmFieldInfoDao(db)
	require.NoError(t, fieldInfoDao.Insert(&system.SmFieldInfoRecord{
		SmDatasetID:      record.SmDatasetID,
		SmFieldName:      "name",
		SmFieldType:      int(types.FieldTypeText),
		SmFieldbRequired: 1,
	}))
	require.NoError(t, fieldInfoDao.Insert(&system.SmFieldInfoRecord{
		SmDatasetID:      record.SmDatasetID,
		SmFieldName:      "level",
		SmFieldType:      int(types.FieldTypeInt32),
		SmFieldbRequired: 0,
	}))

	return NewCadDataset(db, record.ToDatasetInfo()), record
}

func TestCadDataset_CRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createCadDataset(t, db)

	feature := &types.Feature{
		ID: 1,
		Geometry: &types.CadPointGeometry{
			XCoord: 116.123,
			YCoord: 39.456,
			Style: &types.CadMarkerStyle{
				MarkerStyle:       1,
				MarkerSize:        20,
				MarkerAngle:       0,
				MarkerColor:       255,
				MarkerWidth:       20,
				MarkerHeight:      20,
				FillOpaqueRate:    100,
				FillGradientType:  0,
				FillAngle:         0,
				FillCenterOffsetX: 0,
				FillCenterOffsetY: 0,
				FillBackcolor:     16777215,
			},
		},
		Attributes: map[string]interface{}{
			"name":  "CAD point",
			"level": 1,
		},
	}

	require.NoError(t, dataset.Insert(feature))
	count, err := dataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	point, ok := retrieved.Geometry.(*types.CadPointGeometry)
	require.True(t, ok)
	assert.InDelta(t, 116.123, point.XCoord, 0.000000001)
	assert.Equal(t, "CAD point", retrieved.Attributes["name"])

	require.NoError(t, dataset.Update(1, &FeatureChanges{
		Geometry: &types.CadLineGeometry{
			NumSub:         1,
			SubPointCounts: []int{2},
			Coordinates:    [][2]float64{{116.123, 39.456}, {117, 40}},
			Style:          &types.CadLineStyle{LineStyle: 1, LineWidth: 1, LineColor: 65280},
		},
		Attributes: map[string]interface{}{
			"name": "CAD line",
		},
	}))

	updated, err := dataset.GetByID(1)
	require.NoError(t, err)
	_, ok = updated.Geometry.(*types.CadLineGeometry)
	require.True(t, ok)
	assert.Equal(t, "CAD line", updated.Attributes["name"])

	require.NoError(t, dataset.Delete(1))
	count, err = dataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCadDataset_InsertManyAndList(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createCadDataset(t, db)

	features := []*types.Feature{
		{
			ID:       1,
			Geometry: &types.CadPointGeometry{XCoord: 116.123, YCoord: 39.456},
			Attributes: map[string]interface{}{
				"name":  "CAD point",
				"level": 1,
			},
		},
		{
			ID: 2,
			Geometry: &types.CadRegionGeometry{
				NumSub:         1,
				SubPointCounts: []int{5},
				Coordinates:    [][2]float64{{116, 39.2}, {117, 39.2}, {117, 40}, {116, 40}, {116, 39.2}},
				Style:          &types.CadFillStyle{LineStyle: 1, LineWidth: 1, LineColor: 0, FillStyle: 0},
			},
			Attributes: map[string]interface{}{
				"name":  "CAD region",
				"level": 2,
			},
		},
	}

	require.NoError(t, dataset.InsertMany(features))

	all, err := dataset.List(nil)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	filtered, err := dataset.List(&types.QueryOptions{IDs: []int{2}})
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	assert.Equal(t, 2, filtered[0].ID)
	_, ok := filtered[0].Geometry.(*types.CadRegionGeometry)
	assert.True(t, ok)
}
