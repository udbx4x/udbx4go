package schema

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/sqliteutil"
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

func TestInitializerWithTransactionRollsBackSchema(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, NewInitializer(tx).Initialize())
	require.NoError(t, tx.Rollback())

	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='SmRegister'").Scan(&count)
	require.NoError(t, err)
	assert.Zero(t, count)
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

func TestInitializer_CreateCadDatasetTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := NewInitializer(db).CreateCadDatasetTable("cad_layers", []FieldColumn{
		{Name: "name", SQLiteType: "TEXT", Nullable: false},
		{Name: "level", SQLiteType: "INTEGER", Nullable: true},
	})
	require.NoError(t, err)

	columns := readTableColumns(t, db, "cad_layers")
	require.Len(t, columns, 7)
	assert.Equal(t, tableColumn{Name: "SmID", Type: "INTEGER", PrimaryKey: 1}, columns[0])
	assert.Equal(t, tableColumn{Name: "SmUserID", Type: "INTEGER", Default: "0"}, columns[1])
	assert.Equal(t, tableColumn{Name: "SmGeoType", Type: "INTEGER", NotNull: 1}, columns[2])
	assert.Equal(t, tableColumn{Name: "SmGeometry", Type: "BLOB"}, columns[3])
	assert.Equal(t, tableColumn{Name: "SmIndexKey", Type: "POLYGON"}, columns[4])
	assert.Equal(t, tableColumn{Name: "name", Type: "TEXT", NotNull: 1}, columns[5])
	assert.Equal(t, tableColumn{Name: "level", Type: "INTEGER"}, columns[6])
}

func TestInitializer_CreateCadDatasetTableQuotesFieldNames(t *testing.T) {
	tests := []struct {
		name      string
		fieldName string
	}{
		{name: "SQL fragment", fieldName: "name]); DROP TABLE SmRegister;--"},
		{name: "embedded double quote", fieldName: `display"name`},
		{name: "Chinese", fieldName: "名称"},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			defer db.Close()
			require.NoError(t, NewInitializer(db).Initialize())

			tableName := fmt.Sprintf("cad_fields_%d", index)
			err := NewInitializer(db).CreateCadDatasetTable(tableName, []FieldColumn{
				{Name: tt.fieldName, SQLiteType: "TEXT", Nullable: true},
			})
			require.NoError(t, err)

			columns := readTableColumns(t, db, tableName)
			require.Len(t, columns, 6)
			assert.Equal(t, tt.fieldName, columns[5].Name)
			assertTableExists(t, db, "SmRegister", true)
		})
	}
}

func TestInitializer_CreateCadDatasetTableQuotesTableName(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	require.NoError(t, NewInitializer(db).Initialize())

	tableName := `cad"; DROP TABLE SmRegister;--`
	err := NewInitializer(db).CreateCadDatasetTable(tableName, nil)
	require.NoError(t, err)

	columns := readTableColumns(t, db, tableName)
	require.Len(t, columns, 5)
	assertTableExists(t, db, tableName, true)
	assertTableExists(t, db, "SmRegister", true)
}

func TestInitializer_CreateCadDatasetTableAcceptsFieldTypeSQLiteTypes(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	err := NewInitializer(db).CreateCadDatasetTable("cad_types", []FieldColumn{
		{Name: "integer_value", SQLiteType: "INTEGER", Nullable: true},
		{Name: "real_value", SQLiteType: "REAL", Nullable: true},
		{Name: "blob_value", SQLiteType: "BLOB", Nullable: true},
		{Name: "text_value", SQLiteType: "TEXT", Nullable: true},
	})
	require.NoError(t, err)

	columns := readTableColumns(t, db, "cad_types")
	require.Len(t, columns, 9)
	assert.Equal(t, "INTEGER", columns[5].Type)
	assert.Equal(t, "REAL", columns[6].Type)
	assert.Equal(t, "BLOB", columns[7].Type)
	assert.Equal(t, "TEXT", columns[8].Type)
}

func TestInitializer_CreateCadDatasetTableRejectsNULIdentifiers(t *testing.T) {
	tests := []struct {
		name      string
		tableName string
		fieldName string
	}{
		{name: "table name", tableName: "cad\x00archive", fieldName: "name"},
		{name: "field name", tableName: "cad_nul_field", fieldName: "name\x00archive"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			defer db.Close()

			err := NewInitializer(db).CreateCadDatasetTable(tt.tableName, []FieldColumn{
				{Name: tt.fieldName, SQLiteType: "TEXT", Nullable: true},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "NUL")
			if tt.tableName == "cad_nul_field" {
				assertTableExists(t, db, tt.tableName, false)
			}
		})
	}
}

func TestInitializer_CreateCadDatasetTableRejectsUnknownSQLiteTypes(t *testing.T) {
	tests := []struct {
		name       string
		sqliteType string
	}{
		{name: "unknown", sqliteType: "VARCHAR"},
		{name: "trailing whitespace", sqliteType: "TEXT "},
		{name: "SQL fragment", sqliteType: "TEXT); DROP TABLE SmRegister;--"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			defer db.Close()
			require.NoError(t, NewInitializer(db).Initialize())

			err := NewInitializer(db).CreateCadDatasetTable("cad_invalid_type", []FieldColumn{
				{Name: "name", SQLiteType: tt.sqliteType, Nullable: true},
			})
			require.Error(t, err)
			assertTableExists(t, db, "cad_invalid_type", false)
			assertTableExists(t, db, "SmRegister", true)
		})
	}
}

type tableColumn struct {
	Name       string
	Type       string
	NotNull    int
	Default    string
	PrimaryKey int
}

func readTableColumns(t *testing.T, db *sql.DB, tableName string) []tableColumn {
	t.Helper()

	quotedTableName, err := sqliteutil.QuoteIdentifier(tableName)
	require.NoError(t, err)
	rows, err := db.Query("PRAGMA table_info(" + quotedTableName + ")")
	require.NoError(t, err)
	defer rows.Close()

	var columns []tableColumn
	for rows.Next() {
		var column tableColumn
		var cid int
		var defaultValue sql.NullString
		require.NoError(t, rows.Scan(
			&cid,
			&column.Name,
			&column.Type,
			&column.NotNull,
			&defaultValue,
			&column.PrimaryKey,
		))
		if defaultValue.Valid {
			column.Default = defaultValue.String
		}
		columns = append(columns, column)
	}
	require.NoError(t, rows.Err())
	return columns
}

func assertTableExists(t *testing.T, db *sql.DB, tableName string, expected bool) {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRow(
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?",
		tableName,
	).Scan(&count))
	assert.Equal(t, expected, count == 1)
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
