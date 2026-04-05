package schema

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/mattn/go-sqlite3"
)

func setupTestDB(t *testing.T) *sql.DB {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)
	return db
}

func TestNewInitializer(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	initializer := NewInitializer(db)
	assert.NotNil(t, initializer)
	assert.Equal(t, db, initializer.db)
}

func TestInitializer_Initialize(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	initializer := NewInitializer(db)
	err := initializer.Initialize()
	require.NoError(t, err)

	// Verify all system tables were created
	tables := []string{"SmRegister", "SmFieldInfo", "geometry_columns", "SmDataSourceInfo"}
	for _, table := range tables {
		var name string
		err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		require.NoError(t, err, "table %s should exist", table)
		assert.Equal(t, table, name)
	}
}

func TestInitializer_Initialize_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	initializer := NewInitializer(db)

	// First initialization
	err := initializer.Initialize()
	require.NoError(t, err)

	// Second initialization should not fail (idempotent)
	err = initializer.Initialize()
	require.NoError(t, err)
}

func TestInitializer_CreateDatasetTable_WithGeometry(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	initializer := NewInitializer(db)
	err := initializer.Initialize()
	require.NoError(t, err)

	fields := []FieldColumn{
		{Name: "name", SQLiteType: "TEXT", Nullable: true},
		{Name: "population", SQLiteType: "INTEGER", Nullable: true},
	}

	err = initializer.CreateDatasetTable("cities", true, fields)
	require.NoError(t, err)

	// Verify table was created with geometry column
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='cities'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Verify columns
	rows, err := db.Query("PRAGMA table_info(cities)")
	require.NoError(t, err)
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dfltValue interface{}
		err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk)
		require.NoError(t, err)
		columns[name] = true
	}

	assert.True(t, columns["SmID"])
	assert.True(t, columns["SmGeometry"])
	assert.True(t, columns["name"])
	assert.True(t, columns["population"])
}

func TestInitializer_CreateDatasetTable_WithoutGeometry(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	initializer := NewInitializer(db)
	err := initializer.Initialize()
	require.NoError(t, err)

	fields := []FieldColumn{
		{Name: "code", SQLiteType: "TEXT", Nullable: false},
		{Name: "value", SQLiteType: "REAL", Nullable: true},
	}

	err = initializer.CreateDatasetTable("attributes", false, fields)
	require.NoError(t, err)

	// Verify columns
	rows, err := db.Query("PRAGMA table_info(attributes)")
	require.NoError(t, err)
	defer rows.Close()

	columns := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, typ string
		var notNull, pk int
		var dfltValue interface{}
		err := rows.Scan(&cid, &name, &typ, &notNull, &dfltValue, &pk)
		require.NoError(t, err)
		columns[name] = true
	}

	assert.True(t, columns["SmID"])
	assert.False(t, columns["SmGeometry"])
	assert.True(t, columns["code"])
	assert.True(t, columns["value"])
}

func TestInitializer_CreateDatasetTable_NoFields(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	initializer := NewInitializer(db)
	err := initializer.Initialize()
	require.NoError(t, err)

	err = initializer.CreateDatasetTable("empty", true, nil)
	require.NoError(t, err)

	// Verify table exists with only SmID and SmGeometry
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='empty'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestInitializer_DropDatasetTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	initializer := NewInitializer(db)
	err := initializer.Initialize()
	require.NoError(t, err)

	// Create a table
	err = initializer.CreateDatasetTable("to_drop", false, nil)
	require.NoError(t, err)

	// Verify table exists
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='to_drop'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	// Drop the table
	err = initializer.DropDatasetTable("to_drop")
	require.NoError(t, err)

	// Verify table is gone
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='to_drop'").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestInitializer_DropDatasetTable_NonExistent(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	initializer := NewInitializer(db)
	err := initializer.Initialize()
	require.NoError(t, err)

	// Dropping a non-existent table should not error (IF EXISTS)
	err = initializer.DropDatasetTable("nonexistent")
	require.NoError(t, err)
}
