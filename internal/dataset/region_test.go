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

func createRegionDataset(t *testing.T, db *sql.DB) (*RegionDataset, *system.SmRegisterRecord) {
	// Create table
	initializer := schema.NewInitializer(db)
	err := initializer.CreateDatasetTable("districts", true, []schema.FieldColumn{
		{Name: "name", SQLiteType: "TEXT", Nullable: true},
		{Name: "area", SQLiteType: "REAL", Nullable: true},
	})
	require.NoError(t, err)

	// Register dataset
	registerDao := system.NewSmRegisterDao(db)
	record := &system.SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindRegion),
		SmDatasetName: "districts",
		SmTableName:   "districts",
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
		SmFieldName:      "area",
		SmFieldType:      int(types.FieldTypeDouble),
		SmFieldbRequired: 0,
	})

	// Insert geometry column info
	geoColsDao := system.NewGeometryColumnsDao(db)
	srid := 4326
	geoColsDao.Insert(&system.GeometryColumnsRecord{
		FTableName:      "districts",
		GeometryType:    6,
		CoordDimension:  2,
		SRID:            srid,
	})

	info := record.ToDatasetInfo()
	sridPtr := &srid
	info.SRID = sridPtr

	return NewRegionDataset(db, info), record
}

func TestRegionDataset_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

	// Insert a feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiPolygonGeometry{
			Type: "MultiPolygon",
			Coordinates: [][][][]float64{
				{
					{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}},
				},
			},
		},
		Attributes: map[string]interface{}{
			"name": "District A",
			"area": 100.0,
		},
	}

	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Get by ID
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, 1, retrieved.ID)
	assert.NotNil(t, retrieved.Geometry)

	region, ok := retrieved.Geometry.(*types.MultiPolygonGeometry)
	require.True(t, ok)
	assert.Len(t, region.Coordinates, 1)
	assert.Len(t, region.Coordinates[0], 1)
	assert.Len(t, region.Coordinates[0][0], 5) // 5 points (closed ring)
	assert.Equal(t, "District A", retrieved.Attributes["name"])
}

func TestRegionDataset_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

	_, err := dataset.GetByID(999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegionDataset_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

	// Insert multiple features
	features := []*types.Feature{
		{
			ID: 1,
			Geometry: &types.MultiPolygonGeometry{
				Type: "MultiPolygon",
				Coordinates: [][][][]float64{
					{{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}},
				},
			},
			Attributes: map[string]interface{}{"name": "Zone A"},
		},
		{
			ID: 2,
			Geometry: &types.MultiPolygonGeometry{
				Type: "MultiPolygon",
				Coordinates: [][][][]float64{
					{{{2, 2}, {3, 2}, {3, 3}, {2, 3}, {2, 2}}},
				},
			},
			Attributes: map[string]interface{}{"name": "Zone B"},
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

func TestRegionDataset_Insert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiPolygonGeometry{
			Type: "MultiPolygon",
			Coordinates: [][][][]float64{
				{
					{{0, 0}, {5, 0}, {5, 5}, {0, 5}, {0, 0}},
				},
			},
		},
		Attributes: map[string]interface{}{
			"name": "Park",
			"area": 25.0,
		},
	}

	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Verify
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, "Park", retrieved.Attributes["name"])
}

func TestRegionDataset_Insert_WithHole(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

	// Polygon with hole
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiPolygonGeometry{
			Type: "MultiPolygon",
			Coordinates: [][][][]float64{
				{
					{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}},       // outer ring
					{{2, 2}, {8, 2}, {8, 8}, {2, 8}, {2, 2}},           // inner ring (hole)
				},
			},
		},
		Attributes: map[string]interface{}{"name": "Donut"},
	}

	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Verify
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	region := retrieved.Geometry.(*types.MultiPolygonGeometry)
	assert.Len(t, region.Coordinates[0], 2) // outer + inner ring
}

func TestRegionDataset_Insert_WrongGeometryType(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

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
	assert.Contains(t, err.Error(), "geometry must be MultiPolygon")
}

func TestRegionDataset_InsertMany(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

	features := []*types.Feature{
		{
			ID: 1,
			Geometry: &types.MultiPolygonGeometry{
				Type: "MultiPolygon",
				Coordinates: [][][][]float64{
					{{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}},
				},
			},
			Attributes: map[string]interface{}{"name": "Area 1"},
		},
		{
			ID: 2,
			Geometry: &types.MultiPolygonGeometry{
				Type: "MultiPolygon",
				Coordinates: [][][][]float64{
					{{{2, 2}, {3, 2}, {3, 3}, {2, 3}, {2, 2}}},
				},
			},
			Attributes: map[string]interface{}{"name": "Area 2"},
		},
	}

	err := dataset.InsertMany(features)
	require.NoError(t, err)

	// Verify
	all, err := dataset.List(nil)
	require.NoError(t, err)
	assert.Len(t, all, 2)
}

func TestRegionDataset_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

	// Insert feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiPolygonGeometry{
			Type: "MultiPolygon",
			Coordinates: [][][][]float64{
				{{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}},
			},
		},
		Attributes: map[string]interface{}{
			"name":   "Old Name",
			"area":   10.0,
		},
	}
	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Update
	changes := &FeatureChanges{
		Attributes: map[string]interface{}{
			"area": 15.5,
		},
	}
	err = dataset.Update(1, changes)
	require.NoError(t, err)

	// Verify
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.InDelta(t, float64(15.5), retrieved.Attributes["area"], 0.001)
}

func TestRegionDataset_Update_WithGeometry(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

	// Insert feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiPolygonGeometry{
			Type: "MultiPolygon",
			Coordinates: [][][][]float64{
				{{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}},
			},
		},
		Attributes: map[string]interface{}{"name": "Zone"},
	}
	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Update with new geometry
	changes := &FeatureChanges{
		Geometry: &types.MultiPolygonGeometry{
			Type: "MultiPolygon",
			Coordinates: [][][][]float64{
				{{{0, 0}, {2, 0}, {2, 2}, {0, 2}, {0, 0}}},
			},
		},
	}
	err = dataset.Update(1, changes)
	require.NoError(t, err)

	// Verify
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	region := retrieved.Geometry.(*types.MultiPolygonGeometry)
	assert.Len(t, region.Coordinates[0][0], 5)
}

func TestRegionDataset_Update_WrongGeometryType(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

	// Insert feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiPolygonGeometry{
			Type: "MultiPolygon",
			Coordinates: [][][][]float64{
				{{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}},
			},
		},
		Attributes: map[string]interface{}{"name": "Zone"},
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
	assert.Contains(t, err.Error(), "geometry must be MultiPolygon")
}

func TestRegionDataset_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

	changes := &FeatureChanges{
		Attributes: map[string]interface{}{"name": "New"},
	}

	err := dataset.Update(999, changes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRegionDataset_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

	// Insert feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiPolygonGeometry{
			Type: "MultiPolygon",
			Coordinates: [][][][]float64{
				{{{0, 0}, {1, 0}, {1, 1}, {0, 1}, {0, 0}}},
			},
		},
		Attributes: map[string]interface{}{"name": "Zone"},
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

func TestRegionDataset_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createRegionDataset(t, db)

	err := dataset.Delete(999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
