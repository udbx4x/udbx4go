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
		SmFileSmid:       1,
		SmEngineType:     sql.NullInt32{Int32: 1, Valid: true},
		SmFileIdentifier: sql.NullString{String: "test_id", Valid: true},
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
		SmFileSmid:       1,
		SmEngineType:     sql.NullInt32{Int32: 1, Valid: true},
		SmFileIdentifier: sql.NullString{String: "test_id", Valid: true},
		SmPrjCoordSys:    sql.NullString{String: "WGS84", Valid: true},
	}
	err := dao.Insert(record)
	require.NoError(t, err)

	// Get
	retrieved, err := dao.Get()
	require.NoError(t, err)
	require.NotNil(t, retrieved)

	assert.Equal(t, 1, retrieved.SmFileSmid)
	assert.Equal(t, int32(1), retrieved.SmEngineType.Int32)
	assert.Equal(t, "test_id", retrieved.SmFileIdentifier.String)
	assert.Equal(t, "WGS84", retrieved.SmPrjCoordSys.String)
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
