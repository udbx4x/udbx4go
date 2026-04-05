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

func createTabularDataset(t *testing.T, db *sql.DB) (*TabularDataset, *system.SmRegisterRecord) {
	// Create table (no geometry)
	initializer := schema.NewInitializer(db)
	err := initializer.CreateDatasetTable("attributes", false, []schema.FieldColumn{
		{Name: "code", SQLiteType: "TEXT", Nullable: false},
		{Name: "value", SQLiteType: "REAL", Nullable: true},
		{Name: "description", SQLiteType: "TEXT", Nullable: true},
	})
	require.NoError(t, err)

	// Register dataset
	registerDao := system.NewSmRegisterDao(db)
	record := &system.SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindTabular),
		SmDatasetName: "attributes",
		SmTableName:   "attributes",
		SmObjectCount: 0,
	}
	err = registerDao.Insert(record)
	require.NoError(t, err)

	// Insert field info
	fieldInfoDao := system.NewSmFieldInfoDao(db)
	fieldInfoDao.Insert(&system.SmFieldInfoRecord{
		SmDatasetID:      record.SmID,
		SmFieldName:      "code",
		SmFieldType:      int(types.FieldTypeText),
		SmFieldbRequired: 1,
	})
	fieldInfoDao.Insert(&system.SmFieldInfoRecord{
		SmDatasetID:      record.SmID,
		SmFieldName:      "value",
		SmFieldType:      int(types.FieldTypeDouble),
		SmFieldbRequired: 0,
	})
	fieldInfoDao.Insert(&system.SmFieldInfoRecord{
		SmDatasetID:      record.SmID,
		SmFieldName:      "description",
		SmFieldType:      int(types.FieldTypeText),
		SmFieldbRequired: 0,
	})

	info := record.ToDatasetInfo()
	return NewTabularDataset(db, info), record
}

func TestTabularDataset_GetByID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createTabularDataset(t, db)

	// Insert a record
	record := &types.TabularRecord{
		ID: 1,
		Attributes: map[string]interface{}{
			"code":        "ATTR001",
			"value":       99.9,
			"description": "Test attribute",
		},
	}

	err := dataset.Insert(record)
	require.NoError(t, err)

	// Get by ID
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, 1, retrieved.ID)
	assert.Equal(t, "ATTR001", retrieved.Attributes["code"])
	assert.Equal(t, 99.9, retrieved.Attributes["value"])
}

func TestTabularDataset_GetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createTabularDataset(t, db)

	_, err := dataset.GetByID(999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTabularDataset_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createTabularDataset(t, db)

	// Insert multiple records
	records := []*types.TabularRecord{
		{
			ID: 1,
			Attributes: map[string]interface{}{
				"code":  "CODE1",
				"value": 10.5,
			},
		},
		{
			ID: 2,
			Attributes: map[string]interface{}{
				"code":  "CODE2",
				"value": 20.5,
			},
		},
		{
			ID: 3,
			Attributes: map[string]interface{}{
				"code":  "CODE3",
				"value": 30.5,
			},
		},
	}

	for _, r := range records {
		err := dataset.Insert(r)
		require.NoError(t, err)
	}

	// List all
	all, err := dataset.List(nil)
	require.NoError(t, err)
	assert.Len(t, all, 3)

	// List with limit
	limited, err := dataset.List(&types.QueryOptions{Limit: 2})
	require.NoError(t, err)
	assert.Len(t, limited, 2)

	// List with offset (requires limit in SQLite)
	offset, err := dataset.List(&types.QueryOptions{Limit: 10, Offset: 1})
	require.NoError(t, err)
	assert.Len(t, offset, 2)

	// List with IDs filter
	filtered, err := dataset.List(&types.QueryOptions{IDs: []int{1, 3}})
	require.NoError(t, err)
	assert.Len(t, filtered, 2)

	ids := []int{filtered[0].ID, filtered[1].ID}
	assert.Contains(t, ids, 1)
	assert.Contains(t, ids, 3)
}

func TestTabularDataset_Insert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createTabularDataset(t, db)

	record := &types.TabularRecord{
		ID: 100,
		Attributes: map[string]interface{}{
			"code":        "NEW001",
			"value":       123.45,
			"description": "New record",
		},
	}

	err := dataset.Insert(record)
	require.NoError(t, err)

	// Verify
	retrieved, err := dataset.GetByID(100)
	require.NoError(t, err)
	assert.Equal(t, "NEW001", retrieved.Attributes["code"])
	assert.Equal(t, 123.45, retrieved.Attributes["value"])
}

func TestTabularDataset_InsertMany(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createTabularDataset(t, db)

	records := []*types.TabularRecord{
		{ID: 1, Attributes: map[string]interface{}{"code": "A"}},
		{ID: 2, Attributes: map[string]interface{}{"code": "B"}},
		{ID: 3, Attributes: map[string]interface{}{"code": "C"}},
	}

	err := dataset.InsertMany(records)
	require.NoError(t, err)

	// Verify
	all, err := dataset.List(nil)
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestTabularDataset_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createTabularDataset(t, db)

	// Insert record
	record := &types.TabularRecord{
		ID: 1,
		Attributes: map[string]interface{}{
			"code":        "ORIGINAL",
			"value":       10.0,
			"description": "Original desc",
		},
	}
	err := dataset.Insert(record)
	require.NoError(t, err)

	// Update
	updates := map[string]interface{}{
		"value":       99.9,
		"description": "Updated desc",
	}
	err = dataset.Update(1, updates)
	require.NoError(t, err)

	// Verify
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, "ORIGINAL", retrieved.Attributes["code"]) // unchanged
	assert.Equal(t, 99.9, retrieved.Attributes["value"])
	assert.Equal(t, "Updated desc", retrieved.Attributes["description"])
}

func TestTabularDataset_Update_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createTabularDataset(t, db)

	// Insert record
	record := &types.TabularRecord{
		ID:         1,
		Attributes: map[string]interface{}{"code": "TEST"},
	}
	dataset.Insert(record)

	// Update with empty changes
	err := dataset.Update(1, map[string]interface{}{})
	require.NoError(t, err)

	// Verify unchanged
	retrieved, err := dataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, "TEST", retrieved.Attributes["code"])
}

func TestTabularDataset_Update_InvalidField(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createTabularDataset(t, db)

	// Insert record
	record := &types.TabularRecord{
		ID:         1,
		Attributes: map[string]interface{}{"code": "TEST"},
	}
	dataset.Insert(record)

	// Update with invalid field
	updates := map[string]interface{}{
		"nonexistent": "value",
	}
	err := dataset.Update(1, updates)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not exist")
}

func TestTabularDataset_Update_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createTabularDataset(t, db)

	updates := map[string]interface{}{
		"code": "NEW",
	}

	err := dataset.Update(999, updates)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestTabularDataset_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createTabularDataset(t, db)

	// Insert record
	record := &types.TabularRecord{
		ID:         1,
		Attributes: map[string]interface{}{"code": "TO_DELETE"},
	}
	err := dataset.Insert(record)
	require.NoError(t, err)

	// Delete
	err = dataset.Delete(1)
	require.NoError(t, err)

	// Verify deleted
	_, err = dataset.GetByID(1)
	require.Error(t, err)
}

func TestTabularDataset_Delete_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createTabularDataset(t, db)

	err := dataset.Delete(999)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
