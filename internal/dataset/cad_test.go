package dataset

import (
	"database/sql"
	"encoding/binary"
	stderrors "errors"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/codec"
	"github.com/udbx4x/udbx4go/internal/schema"
	"github.com/udbx4x/udbx4go/internal/system"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func createCadDataset(t *testing.T, db *sql.DB) (*CadDataset, *system.SmRegisterRecord) {
	return createCadDatasetWithSRID(t, db, 0)
}

func createCadDatasetWithSRID(t *testing.T, db *sql.DB, srid int) (*CadDataset, *system.SmRegisterRecord) {
	require.NoError(t, schema.NewInitializer(db).CreateCadDatasetTable("cad_layers", []schema.FieldColumn{
		{Name: "name", SQLiteType: "TEXT", Nullable: false},
		{Name: "level", SQLiteType: "INTEGER", Nullable: true},
	}))
	// Preserve the existing corruption test's ability to inject a NULL geometry
	// without weakening the production schema or normal CAD writes.
	_, err := db.Exec(`
		CREATE TRIGGER cad_layers_insert_missing_geometry
		BEFORE INSERT ON cad_layers
		WHEN NEW.SmGeometry IS NULL AND NEW.SmGeoType IS NULL
		BEGIN
			INSERT INTO cad_layers (
				SmID, SmUserID, SmGeoType, SmGeometry, SmIndexKey, name, level
			) VALUES (
				NEW.SmID, NEW.SmUserID, 0, NULL, NEW.SmIndexKey, NEW.name, NEW.level
			);
			SELECT RAISE(IGNORE);
		END
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
		SmSRID:        sql.NullInt32{Int32: int32(srid), Valid: true},
		SmIndexType:   sql.NullInt32{Int32: 0, Valid: true},
	}
	err = registerDao.Insert(record)
	require.NoError(t, err)
	require.NoError(t, system.NewGeometryColumnsDao(db).Insert(&system.GeometryColumnsRecord{
		FTableName:          "cad_layers",
		FGeometryColumn:     "SmIndexKey",
		GeometryType:        3,
		CoordDimension:      2,
		SRID:                srid,
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

func TestCadDatasetMaintainsGeoTypeAndIndexKey(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createCadDatasetWithSRID(t, db, 3857)
	tests := []struct {
		name     string
		id       int
		geometry types.CadGeometry
		geoType  int
		bbox     types.BoundingBox
	}{
		{
			name:     "point with zero-area negative envelope",
			id:       1,
			geometry: &types.CadPointGeometry{XCoord: -3, YCoord: -4},
			geoType:  1,
			bbox:     types.BoundingBox{MinX: -3, MinY: -4, MaxX: -3, MaxY: -4},
		},
		{
			name: "line",
			id:   2,
			geometry: &types.CadLineGeometry{
				NumSub:         1,
				SubPointCounts: []int{2},
				Coordinates:    [][2]float64{{-8, -2}, {9, 5}},
			},
			geoType: 3,
			bbox:    types.BoundingBox{MinX: -8, MinY: -2, MaxX: 9, MaxY: 5},
		},
		{
			name: "region",
			id:   3,
			geometry: &types.CadRegionGeometry{
				NumSub:         1,
				SubPointCounts: []int{5},
				Coordinates:    [][2]float64{{-6, -7}, {4, -7}, {4, 2}, {-6, 2}, {-6, -7}},
			},
			geoType: 5,
			bbox:    types.BoundingBox{MinX: -6, MinY: -7, MaxX: 4, MaxY: 2},
		},
		{
			name: "text",
			id:   4,
			geometry: &types.CadTextGeometry{
				Text:   "CAD text",
				Anchor: []float64{-2, 3},
				BBox:   []float64{-5, -1, 6, 8},
			},
			geoType: 7,
			bbox:    types.BoundingBox{MinX: -5, MinY: -1, MaxX: 6, MaxY: 8},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, dataset.Insert(&types.Feature{
				ID:         tt.id,
				Geometry:   tt.geometry,
				Attributes: map[string]interface{}{"name": tt.name},
			}))
			assertCadStorage(t, db, dataset.TableName(), tt.id, tt.geoType, 3857, tt.bbox)
			feature, err := dataset.GetByID(tt.id)
			require.NoError(t, err)
			assert.Equal(t, tt.geoType, feature.Geometry.(types.CadGeometry).CadGeoType())
		})
	}

	line := &types.CadLineGeometry{
		NumSub:         1,
		SubPointCounts: []int{2},
		Coordinates:    [][2]float64{{-10, 2}, {8, 9}},
	}
	require.NoError(t, dataset.Update(1, &FeatureChanges{Geometry: line}))
	assertCadStorage(t, db, dataset.TableName(), 1, 3, 3857, types.BoundingBox{MinX: -10, MinY: 2, MaxX: 8, MaxY: 9})

	var indexBefore []byte
	require.NoError(t, db.QueryRow("SELECT SmIndexKey FROM cad_layers WHERE SmID = 1").Scan(&indexBefore))
	require.NoError(t, dataset.Update(1, &FeatureChanges{Attributes: map[string]interface{}{"name": "renamed"}}))
	var indexAfter []byte
	require.NoError(t, db.QueryRow("SELECT SmIndexKey FROM cad_layers WHERE SmID = 1").Scan(&indexAfter))
	assert.Equal(t, indexBefore, indexAfter)

	feature, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, "renamed", feature.Attributes["name"])
	assert.NotContains(t, feature.Attributes, "SmUserID")
	assert.NotContains(t, feature.Attributes, "SmGeoType")
	assert.NotContains(t, feature.Attributes, "SmIndexKey")
}

func TestCadDatasetRejectsStoredGeoTypeMismatch(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createCadDataset(t, db)
	require.NoError(t, dataset.Insert(&types.Feature{
		ID:         1,
		Geometry:   &types.CadPointGeometry{XCoord: 3, YCoord: 4},
		Attributes: map[string]interface{}{"name": "point"},
	}))
	_, err := db.Exec("UPDATE cad_layers SET SmGeoType = 5 WHERE SmID = 1")
	require.NoError(t, err)

	_, err = dataset.GetByID(1)
	require.Error(t, err)
	var geometryErr *spatialGeometryError
	assert.True(t, stderrors.As(err, &geometryErr))
	assert.ErrorContains(t, err, "stored CAD SmGeoType 5 does not match decoded geometry type 1")
}

func TestCadDatasetGeometryEncodingFailureDoesNotWritePartialData(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createCadDataset(t, db)
	invalidGeometries := []struct {
		name     string
		geometry types.CadGeometry
	}{
		{name: "non-finite bbox", geometry: &types.CadPointGeometry{XCoord: math.NaN(), YCoord: 1}},
		{name: "empty bbox", geometry: &types.CadLineGeometry{}},
		{
			name:     "text missing anchor",
			geometry: &types.CadTextGeometry{Text: "invalid", BBox: []float64{1, 2, 3, 4}},
		},
	}
	for index, tt := range invalidGeometries {
		t.Run(tt.name, func(t *testing.T) {
			err := dataset.Insert(&types.Feature{
				ID:         index + 1,
				Geometry:   tt.geometry,
				Attributes: map[string]interface{}{"name": tt.name},
			})
			require.Error(t, err)
			var count int
			require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM cad_layers WHERE SmID = ?", index+1).Scan(&count))
			assert.Zero(t, count)
		})
	}

	require.NoError(t, dataset.Insert(&types.Feature{
		ID:         10,
		Geometry:   &types.CadPointGeometry{XCoord: 1, YCoord: 2},
		Attributes: map[string]interface{}{"name": "before", "level": 1},
	}))
	var beforeType int
	var beforeGeometry, beforeIndex []byte
	var beforeName string
	require.NoError(t, db.QueryRow(
		"SELECT SmGeoType, SmGeometry, SmIndexKey, name FROM cad_layers WHERE SmID = 10",
	).Scan(&beforeType, &beforeGeometry, &beforeIndex, &beforeName))

	err := dataset.Update(10, &FeatureChanges{
		Geometry:   &types.CadTextGeometry{Text: "invalid", Anchor: []float64{1, 2}, BBox: []float64{4, 3, 2, 1}},
		Attributes: map[string]interface{}{"name": "after"},
	})
	require.Error(t, err)

	var afterType int
	var afterGeometry, afterIndex []byte
	var afterName string
	require.NoError(t, db.QueryRow(
		"SELECT SmGeoType, SmGeometry, SmIndexKey, name FROM cad_layers WHERE SmID = 10",
	).Scan(&afterType, &afterGeometry, &afterIndex, &afterName))
	assert.Equal(t, beforeType, afterType)
	assert.Equal(t, beforeGeometry, afterGeometry)
	assert.Equal(t, beforeIndex, afterIndex)
	assert.Equal(t, beforeName, afterName)
}

func assertCadStorage(
	t *testing.T,
	db *sql.DB,
	table string,
	id int,
	wantType int,
	wantSRID int,
	wantBBox types.BoundingBox,
) {
	t.Helper()

	var userID, geoType int
	var index []byte
	require.NoError(t, db.QueryRow(
		"SELECT SmUserID, SmGeoType, SmIndexKey FROM "+table+" WHERE SmID = ?", id,
	).Scan(&userID, &geoType, &index))
	assert.Zero(t, userID)
	assert.Equal(t, wantType, geoType)
	require.GreaterOrEqual(t, len(index), codec.GaiaHeaderLength)
	assert.Equal(t, wantSRID, int(int32(binary.LittleEndian.Uint32(index[2:6]))))
	envelope, err := codec.ReadGaiaEnvelope(index)
	require.NoError(t, err)
	assert.Equal(t, wantBBox, envelope)
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
