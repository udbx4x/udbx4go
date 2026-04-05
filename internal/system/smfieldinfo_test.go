package system

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestSmFieldInfoDao_Insert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmFieldInfoDao(db)

	record := &SmFieldInfoRecord{
		SmDatasetID:      1,
		SmFieldName:      "name",
		SmFieldCaption:   sql.NullString{String: "City Name", Valid: true},
		SmFieldType:      int(types.FieldTypeText),
		SmFieldbRequired: 1,
	}

	err := dao.Insert(record)
	require.NoError(t, err)
}

func TestSmFieldInfoDao_ListByDatasetID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmFieldInfoDao(db)

	// Insert fields for dataset 1
	fields := []*SmFieldInfoRecord{
		{SmDatasetID: 1, SmFieldName: "name", SmFieldType: int(types.FieldTypeText)},
		{SmDatasetID: 1, SmFieldName: "population", SmFieldType: int(types.FieldTypeInt32)},
		{SmDatasetID: 1, SmFieldName: "area", SmFieldType: int(types.FieldTypeDouble)},
	}

	for _, f := range fields {
		err := dao.Insert(f)
		require.NoError(t, err)
	}

	// Insert field for dataset 2
	err := dao.Insert(&SmFieldInfoRecord{
		SmDatasetID: 2, SmFieldName: "code", SmFieldType: int(types.FieldTypeText),
	})
	require.NoError(t, err)

	// List fields for dataset 1
	list, err := dao.ListByDatasetID(1)
	require.NoError(t, err)
	assert.Len(t, list, 3)
}

func TestSmFieldInfoDao_ListByDatasetID_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmFieldInfoDao(db)

	list, err := dao.ListByDatasetID(999)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestSmFieldInfoDao_DeleteByDatasetID(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmFieldInfoDao(db)

	// Insert fields
	err := dao.Insert(&SmFieldInfoRecord{SmDatasetID: 1, SmFieldName: "name", SmFieldType: int(types.FieldTypeText)})
	require.NoError(t, err)

	// Delete all fields for dataset 1
	err = dao.DeleteByDatasetID(1)
	require.NoError(t, err)

	// Verify deletion
	list, err := dao.ListByDatasetID(1)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestSmFieldInfoRecord_ToFieldInfo(t *testing.T) {
	record := &SmFieldInfoRecord{
		SmDatasetID:       1,
		SmFieldName:       "population",
		SmFieldCaption:    sql.NullString{String: "Population", Valid: true},
		SmFieldType:       int(types.FieldTypeInt32),
		SmFieldbRequired:  1,
	}

	info := record.ToFieldInfo()

	assert.Equal(t, "population", info.Name)
	assert.Equal(t, types.FieldTypeInt32, info.FieldType)
	require.NotNil(t, info.Alias)
	assert.Equal(t, "Population", *info.Alias)
	assert.True(t, info.Required)
}

func TestSmFieldInfoRecord_ToFieldInfo_NoAlias(t *testing.T) {
	record := &SmFieldInfoRecord{
		SmDatasetID:      1,
		SmFieldName:      "name",
		SmFieldCaption:   sql.NullString{},
		SmFieldType:      int(types.FieldTypeText),
		SmFieldbRequired: 0,
	}

	info := record.ToFieldInfo()

	assert.Nil(t, info.Alias)
	assert.False(t, info.Required)
}
