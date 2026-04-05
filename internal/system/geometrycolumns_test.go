package system

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeometryColumnsDao_Insert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewGeometryColumnsDao(db)

	record := &GeometryColumnsRecord{
		FTableName:          "cities",
		FGeometryColumn:     "SmGeometry",
		GeometryType:        1,
		CoordDimension:      2,
		SRID:                4326,
		SpatialIndexEnabled: 0,
	}

	err := dao.Insert(record)
	require.NoError(t, err)
}

func TestGeometryColumnsDao_GetByTableName(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewGeometryColumnsDao(db)

	// Insert record
	record := &GeometryColumnsRecord{
		FTableName:          "cities",
		FGeometryColumn:     "SmGeometry",
		GeometryType:        1,
		CoordDimension:      2,
		SRID:                4326,
		SpatialIndexEnabled: 0,
	}
	err := dao.Insert(record)
	require.NoError(t, err)

	// Get by table name
	retrieved, err := dao.GetByTableName("cities")
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, "cities", retrieved.FTableName)
	assert.Equal(t, "SmGeometry", retrieved.FGeometryColumn)
	assert.Equal(t, 1, retrieved.GeometryType)
	assert.Equal(t, 4326, retrieved.SRID)
}

func TestGeometryColumnsDao_GetByTableName_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewGeometryColumnsDao(db)

	// Get non-existent
	retrieved, err := dao.GetByTableName("nonexistent")
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestGeometryColumnsDao_DeleteByTableName(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewGeometryColumnsDao(db)

	// Insert record
	err := dao.Insert(&GeometryColumnsRecord{
		FTableName:      "cities",
		GeometryType:    1,
		CoordDimension:  2,
		SRID:            4326,
	})
	require.NoError(t, err)

	// Delete
	err = dao.DeleteByTableName("cities")
	require.NoError(t, err)

	// Verify deletion
	retrieved, err := dao.GetByTableName("cities")
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}
