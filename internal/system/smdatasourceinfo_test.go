package system

import (
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSmDataSourceInfoDao_Insert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmDataSourceInfoDao(db)

	record := &SmDataSourceInfoRecord{
		SmFlag:       1,
		SmVersion:    sql.NullInt32{Int32: 1, Valid: true},
		SmDataFormat: sql.NullInt32{Int32: 1, Valid: true},
	}

	err := dao.Insert(record)
	require.NoError(t, err)
}

func TestSmDataSourceInfoDao_Get(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmDataSourceInfoDao(db)

	// Insert record
	record := &SmDataSourceInfoRecord{
		SmFlag:           1,
		SmVersion:        sql.NullInt32{Int32: 1, Valid: true},
		SmDsDescription:  sql.NullString{String: "test desc", Valid: true},
		SmDataFormat:     sql.NullInt32{Int32: 1, Valid: true},
	}
	err := dao.Insert(record)
	require.NoError(t, err)

	// Get
	retrieved, err := dao.Get()
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, 1, retrieved.SmFlag)
	assert.Equal(t, int32(1), retrieved.SmVersion.Int32)
	assert.Equal(t, "test desc", retrieved.SmDsDescription.String)
	assert.Equal(t, int32(1), retrieved.SmDataFormat.Int32)
}

func TestSmDataSourceInfoDao_Get_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmDataSourceInfoDao(db)

	// Get when no records
	retrieved, err := dao.Get()
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}
