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

func createPointDataset(t *testing.T, db *sql.DB) (*PointDataset, *system.SmRegisterRecord) {
	// Create table
	initializer := schema.NewInitializer(db)
	err := initializer.CreateDatasetTable("cities", true, []schema.FieldColumn{
		{Name: "name", SQLiteType: "TEXT", Nullable: true},
		{Name: "population", SQLiteType: "INTEGER", Nullable: true},
	})
	require.NoError(t, err)

	// Register dataset
	registerDao := system.NewSmRegisterDao(db)
	record := &system.SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindPoint),
		SmDatasetName: "cities",
		SmTableName:   "cities",
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
		SmFieldName:      "population",
		SmFieldType:      int(types.FieldTypeInt32),
		SmFieldbRequired: 0,
	})

	// Insert geometry column info
	geoColsDao := system.NewGeometryColumnsDao(db)
	srid := 4326
	geoColsDao.Insert(&system.GeometryColumnsRecord{
		FTableName:      "cities",
		GeometryType:    1,
		CoordDimension:  2,
		SRID:            srid,
	})

	info := record.ToDatasetInfo()
	sridPtr := &srid
	info.SRID = sridPtr

	return NewPointDataset(db, info), record
}

func TestPointDataset_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createPointDataset(t, db)

	// Insert a feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.PointGeometry{
			Type:        "Point",
			Coordinates: []float64{116.4, 39.9},
		},
		Attributes: map[string]interface{}{
			"name":       "Beijing",
			"population": 21540000,
		},
	}

	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Get by ID
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, 1, retrieved.ID)
	assert.NotNil(t, retrieved.Geometry)

	point, ok := retrieved.Geometry.(*types.PointGeometry)
	require.True(t, ok)
	assert.InDelta(t, 116.4, point.X(), 0.0001)
	assert.InDelta(t, 39.9, point.Y(), 0.0001)
	assert.Equal(t, "Beijing", retrieved.Attributes["name"])
}

func TestPointDataset_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createPointDataset(t, db)

	_, err := dataset.GetByID(999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPointDataset_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createPointDataset(t, db)

	// Insert multiple features
	features := []*types.Feature{
		{
			ID: 1,
			Geometry: &types.PointGeometry{
				Type:        "Point",
				Coordinates: []float64{116.4, 39.9},
			},
			Attributes: map[string]interface{}{"name": "Beijing"},
		},
		{
			ID: 2,
			Geometry: &types.PointGeometry{
				Type:        "Point",
				Coordinates: []float64{121.5, 31.2},
			},
			Attributes: map[string]interface{}{"name": "Shanghai"},
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

func TestPointDataset_Insert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createPointDataset(t, db)

	feature := &types.Feature{
		ID: 1,
		Geometry: &types.PointGeometry{
			Type:        "Point",
			Coordinates: []float64{116.4, 39.9},
		},
		Attributes: map[string]interface{}{
			"name":       "Beijing",
			"population": 21540000,
		},
	}

	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Verify
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, "Beijing", retrieved.Attributes["name"])
}

func TestPointDataset_Insert_WrongGeometryType(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createPointDataset(t, db)

	feature := &types.Feature{
		ID: 1,
		Geometry: &types.MultiLineStringGeometry{
			Type: "MultiLineString",
			Coordinates: [][][]float64{
				{{116.4, 39.9}, {116.5, 39.8}},
			},
		},
		Attributes: map[string]interface{}{"name": "Road"},
	}

	err := dataset.Insert(feature)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "geometry must be Point")
}

func TestPointDataset_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createPointDataset(t, db)

	// Insert feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.PointGeometry{
			Type:        "Point",
			Coordinates: []float64{116.4, 39.9},
		},
		Attributes: map[string]interface{}{
			"name":       "Beijing",
			"population": 21540000,
		},
	}
	err := dataset.Insert(feature)
	require.NoError(t, err)

	// Update
	changes := &FeatureChanges{
		Attributes: map[string]interface{}{
			"population": 22000000,
		},
	}
	err = dataset.Update(1, changes)
	require.NoError(t, err)

	// Verify
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, int64(22000000), retrieved.Attributes["population"])
}

func TestPointDataset_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createPointDataset(t, db)

	changes := &FeatureChanges{
		Attributes: map[string]interface{}{
			"name": "New",
		},
	}

	err := dataset.Update(999, changes)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestPointDataset_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createPointDataset(t, db)

	// Insert feature
	feature := &types.Feature{
		ID: 1,
		Geometry: &types.PointGeometry{
			Type:        "Point",
			Coordinates: []float64{116.4, 39.9},
		},
		Attributes: map[string]interface{}{"name": "Beijing"},
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

func TestPointDataset_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createPointDataset(t, db)

	err := dataset.Delete(999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
