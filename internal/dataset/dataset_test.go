package dataset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/system"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestBaseDataset_Info(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	info := &types.DatasetInfo{
		ID:          1,
		Name:        "test",
		TableName:   "test_table",
		Kind:        types.DatasetKindPoint,
		ObjectCount: 10,
	}

	dataset := NewBaseDataset(db, info)

	assert.Equal(t, info, dataset.Info())
	assert.Equal(t, "test_table", dataset.TableName())
	assert.Equal(t, db, dataset.DB())
}

func TestBaseDataset_CountReadsPhysicalTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE test_table (SmID INTEGER PRIMARY KEY, value TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO test_table (SmID, value) VALUES (1, 'a'), (2, 'b'), (3, 'c')`)
	require.NoError(t, err)

	info := &types.DatasetInfo{
		ID:          1,
		Name:        "test",
		TableName:   "test_table",
		Kind:        types.DatasetKindTabular,
		ObjectCount: 99,
	}

	dataset := NewBaseDataset(db, info)
	count, err := dataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.Equal(t, 99, dataset.Info().ObjectCount)
}

func TestBaseDataset_GetFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create a dataset record
	registerDao := system.NewSmRegisterDao(db)
	record := &system.SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindPoint),
		SmDatasetName: "test",
		SmTableName:   "test_table",
		SmObjectCount: 0,
	}
	err := registerDao.Insert(record)
	require.NoError(t, err)

	// Insert field info
	fieldInfoDao := system.NewSmFieldInfoDao(db)
	fieldRecord := &system.SmFieldInfoRecord{
		SmDatasetID:      record.SmDatasetID,
		SmFieldName:      "name",
		SmFieldType:      int(types.FieldTypeText),
		SmFieldbRequired: 1,
	}
	err = fieldInfoDao.Insert(fieldRecord)
	require.NoError(t, err)

	info := record.ToDatasetInfo()
	dataset := NewBaseDataset(db, info)

	fields, err := dataset.GetFields()
	require.NoError(t, err)
	require.Len(t, fields, 1)

	assert.Equal(t, "name", fields[0].Name)
	assert.Equal(t, types.FieldTypeText, fields[0].FieldType)
	assert.True(t, fields[0].Required)
}

func TestBaseDataset_Close(t *testing.T) {
	db := setupTestDB(t)

	info := &types.DatasetInfo{
		ID:        1,
		Name:      "test",
		TableName: "test_table",
		Kind:      types.DatasetKindTabular,
	}

	dataset := NewBaseDataset(db, info)

	// Close should not return error
	err := dataset.Close()
	assert.NoError(t, err)

	// Close DB
	db.Close()
}
