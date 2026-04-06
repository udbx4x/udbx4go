package dataset

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/schema"
	"github.com/udbx4x/udbx4go/internal/system"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func createLineDataset(t *testing.T, db *sql.DB) (*LineDataset, *system.SmRegisterRecord) {
	// Create table
	initializer := schema.NewInitializer(db)
	err := initializer.CreateDatasetTable("roads", true, []schema.FieldColumn{
		{Name: "name", SQLiteType: "TEXT", Nullable: true},
		{Name: "length", SQLiteType: "REAL", Nullable: true},
	})
	require.NoError(t, err)

	// Register dataset
	registerDao := system.NewSmRegisterDao(db)
	record := &system.SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindLine),
		SmDatasetName: "roads",
		SmTableName:   "roads",
		SmObjectCount: 0,
	}
	err = registerDao.Insert(record)
	require.NoError(t, err)

	// Insert field info
	fieldInfoDao := system.NewSmFieldInfoDao(db)
	fieldInfoDao.Insert(&system.SmFieldInfoRecord{
		SmDatasetID:      record.SmDatasetID,
		SmFieldName:      "name",
		SmFieldType:      int(types.FieldTypeText),
		SmFieldbRequired: 0,
	})
	fieldInfoDao.Insert(&system.SmFieldInfoRecord{
		SmDatasetID:      record.SmDatasetID,
		SmFieldName:      "length",
		SmFieldType:      int(types.FieldTypeDouble),
		SmFieldbRequired: 0,
	})

	// Insert geometry column info
	geoColsDao := system.NewGeometryColumnsDao(db)
	srid := 4326
	geoColsDao.Insert(&system.GeometryColumnsRecord{
		FTableName:      "roads",
		GeometryType:    5,
		CoordDimension:  2,
		SRID:            srid,
	})

	info := record.ToDatasetInfo()
	sridPtr := &srid
	info.SRID = sridPtr

	return NewLineDataset(db, info), record
}

func TestLineDataset_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createLineDataset(t, db)

	// Insert a feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiLineStringGeometry{
			Type: "MultiLineString",
			Coordinates: [][][]float64{
				{{116.4, 39.9}, {116.5, 39.8}, {116.6, 39.85}},
			},
		},
		Attributes: map[string]interface{}{
			"name":   "Main Road",
			"length": 10.5,
		},
	}

	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Get by ID
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, 1, retrieved.ID)
	assert.NotNil(t, retrieved.Geometry)

	line, ok := retrieved.Geometry.(*types.MultiLineStringGeometry)
	require.True(t, ok)
	assert.Len(t, line.Coordinates, 1)
	assert.Len(t, line.Coordinates[0], 3)
	assert.Equal(t, "Main Road", retrieved.Attributes["name"])
}

func TestLineDataset_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createLineDataset(t, db)

	_, err := dataset.GetByID(999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLineDataset_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createLineDataset(t, db)

	// Insert multiple features
	features := []*types.Feature{
		{
			ID: 1,
			Geometry: &types.MultiLineStringGeometry{
				Type:        "MultiLineString",
				Coordinates: [][][]float64{{{116.4, 39.9}, {116.5, 39.8}}},
			},
			Attributes: map[string]interface{}{"name": "Road A"},
		},
		{
			ID: 2,
			Geometry: &types.MultiLineStringGeometry{
				Type:        "MultiLineString",
				Coordinates: [][][]float64{{{121.5, 31.2}, {121.6, 31.3}}},
			},
			Attributes: map[string]interface{}{"name": "Road B"},
		},
	}

	for _, f := range features {
		err := dataset.Insert(f)
		require.NoError(t, err)
	}

	// List all
	all, err := dataset.List(nil)
	require.NoError(t, err)
	assert.Len(t, all, 2)

	// List with limit
	limited, err := dataset.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	assert.Len(t, limited, 1)

	// List with IDs filter
	filtered, err := dataset.List(&types.QueryOptions{IDs: []int{1}})
	require.NoError(t, err)
	assert.Len(t, filtered, 1)
	assert.Equal(t, 1, filtered[0].ID)
}

func TestLineDataset_Insert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createLineDataset(t, db)

	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiLineStringGeometry{
			Type: "MultiLineString",
			Coordinates: [][][]float64{
				{{116.4, 39.9}, {116.5, 39.8}},
			},
		},
		Attributes: map[string]interface{}{
			"name":   "Highway",
			"length": 25.3,
		},
	}

	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Verify
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, "Highway", retrieved.Attributes["name"])
}

func TestLineDataset_Insert_WrongGeometryType(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createLineDataset(t, db)

	feature := &types.Feature{
		ID: 1,
		Geometry: &types.PointGeometry{
			Type:        "Point",
			Coordinates: []float64{116.4, 39.9},
		},
		Attributes: map[string]interface{}{"name": "Point"},
	}

	err := dataset.Insert(feature)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "geometry must be MultiLineString")
}

func TestLineDataset_InsertMany(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createLineDataset(t, db)

	features := []*types.Feature{
		{
			ID: 1,
			Geometry: &types.MultiLineStringGeometry{
				Type:        "MultiLineString",
				Coordinates: [][][]float64{{{0, 0}, {1, 1}}},
			},
			Attributes: map[string]interface{}{"name": "Line 1"},
		},
		{
			ID: 2,
			Geometry: &types.MultiLineStringGeometry{
				Type:        "MultiLineString",
				Coordinates: [][][]float64{{{2, 2}, {3, 3}}},
			},
			Attributes: map[string]interface{}{"name": "Line 2"},
		},
	}

	err := dataset.InsertMany(features)
	require.NoError(t, err)

	// Verify
	all, err := dataset.List(nil)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestLineDataset_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createLineDataset(t, db)

	// Insert feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiLineStringGeometry{
			Type:        "MultiLineString",
			Coordinates: [][][]float64{{{0, 0}, {1, 1}}},
		},
		Attributes: map[string]interface{}{
			"name":   "Old Name",
			"length": 10.0,
		},
	}
	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Update
	changes := &FeatureChanges{
		Attributes: map[string]interface{}{
			"length": 15.5,
		},
	}
	err = dataset.Update(1, changes)
	require.NoError(t, err)

	// Verify
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.InDelta(t, float64(15.5), retrieved.Attributes["length"], 0.001)
}

func TestLineDataset_Update_WithGeometry(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createLineDataset(t, db)

	// Insert feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiLineStringGeometry{
			Type:        "MultiLineString",
			Coordinates: [][][]float64{{{0, 0}, {1, 1}}},
		},
		Attributes: map[string]interface{}{"name": "Road"},
	}
	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Update with new geometry
	changes := &FeatureChanges{
		Geometry: &types.MultiLineStringGeometry{
			Type:        "MultiLineString",
			Coordinates: [][][]float64{{{0, 0}, {2, 2}, {4, 4}}},
		},
	}
	err = dataset.Update(1, changes)
	require.NoError(t, err)

	// Verify
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	line := retrieved.Geometry.(*types.MultiLineStringGeometry)
	assert.Len(t, line.Coordinates[0], 3)
}

func TestLineDataset_Update_WrongGeometryType(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createLineDataset(t, db)

	// Insert feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiLineStringGeometry{
			Type:        "MultiLineString",
			Coordinates: [][][]float64{{{0, 0}, {1, 1}}},
		},
		Attributes: map[string]interface{}{"name": "Road"},
	}
	dataset.Insert(feature)

	// Update with wrong geometry type
	changes := &FeatureChanges{
		Geometry: &types.PointGeometry{
			Type:        "Point",
			Coordinates: []float64{0, 0},
		},
	}
	err := dataset.Update(1, changes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "geometry must be MultiLineString")
}

func TestLineDataset_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createLineDataset(t, db)

	changes := &FeatureChanges{
		Attributes: map[string]interface{}{"name": "New"},
	}

	err := dataset.Update(999, changes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestLineDataset_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createLineDataset(t, db)

	// Insert feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiLineStringGeometry{
			Type:        "MultiLineString",
			Coordinates: [][][]float64{{{0, 0}, {1, 1}}},
		},
		Attributes: map[string]interface{}{"name": "Road"},
	}
	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Delete
	err = dataset.Delete(1)
	require.NoError(t, err)

	// Verify deleted
	_, err = dataset.GetByID(1)
	require.Error(t, err)
}

func TestLineDataset_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createLineDataset(t, db)

	err := dataset.Delete(999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
