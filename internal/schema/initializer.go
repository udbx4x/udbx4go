// Package schema provides UDBX database schema initialization.
package schema

import (
	"database/sql"

	"github.com/udbx4x/udbx4go/pkg/errors"
)

// Initializer handles creation of UDBX system tables.
type Initializer struct {
	db *sql.DB
}

// NewInitializer creates a new schema initializer.
func NewInitializer(db *sql.DB) *Initializer {
	return &Initializer{db: db}
}

// Initialize creates all required UDBX system tables.
func (i *Initializer) Initialize() error {
	if err := i.createSmRegister(); err != nil {
		return errors.IOError("failed to create SmRegister table", err)
	}

	if err := i.createSmFieldInfo(); err != nil {
		return errors.IOError("failed to create SmFieldInfo table", err)
	}

	if err := i.createGeometryColumns(); err != nil {
		return errors.IOError("failed to create geometry_columns table", err)
	}

	if err := i.createSmDataSourceInfo(); err != nil {
		return errors.IOError("failed to create SmDataSourceInfo table", err)
	}

	return nil
}

// createSmRegister creates the SmRegister table.
func (i *Initializer) createSmRegister() error {
	query := `
		CREATE TABLE IF NOT EXISTS SmRegister (
			SmID INTEGER PRIMARY KEY AUTOINCREMENT,
			SmDatasetType INTEGER NOT NULL,
			SmDatasetName TEXT NOT NULL UNIQUE,
			SmTableName TEXT NOT NULL,
			SmMaxX REAL,
			SmMaxY REAL,
			SmMinX REAL,
			SmMinY REAL,
			SmCenterX REAL,
			SmCenterY REAL,
			SmSrid INTEGER,
			SmObjectCount INTEGER DEFAULT 0,
			SmMaxGeometrySize INTEGER DEFAULT 0
		)
	`

	_, err := i.db.Exec(query)
	return err
}

// createSmFieldInfo creates the SmFieldInfo table.
func (i *Initializer) createSmFieldInfo() error {
	query := `
		CREATE TABLE IF NOT EXISTS SmFieldInfo (
			SmDatasetID INTEGER NOT NULL,
			SmFieldName TEXT NOT NULL,
			SmFieldCaption TEXT,
			SmFieldType INTEGER NOT NULL,
			SmFieldbRequired INTEGER DEFAULT 0,
			SmFieldDefaultValue TEXT,
			PRIMARY KEY (SmDatasetID, SmFieldName)
		)
	`

	_, err := i.db.Exec(query)
	return err
}

// createGeometryColumns creates the geometry_columns table.
func (i *Initializer) createGeometryColumns() error {
	query := `
		CREATE TABLE IF NOT EXISTS geometry_columns (
			f_table_name TEXT NOT NULL,
			f_geometry_column TEXT NOT NULL DEFAULT 'SmGeometry',
			geometry_type INTEGER,
			coord_dimension INTEGER,
			srid INTEGER,
			spatial_index_enabled INTEGER DEFAULT 0,
			PRIMARY KEY (f_table_name, f_geometry_column)
		)
	`

	_, err := i.db.Exec(query)
	return err
}

// createSmDataSourceInfo creates the SmDataSourceInfo table.
func (i *Initializer) createSmDataSourceInfo() error {
	query := `
		CREATE TABLE IF NOT EXISTS SmDataSourceInfo (
			SmFileSmid INTEGER PRIMARY KEY,
			SmEngineType INTEGER,
			SmFileIdentifier TEXT,
			SmFilePwd TEXT,
			SmPrjCoordSys TEXT
		)
	`

	_, err := i.db.Exec(query)
	return err
}

// CreateDatasetTable creates a dataset data table.
func (i *Initializer) CreateDatasetTable(tableName string, hasGeometry bool, fieldInfos []FieldColumn) error {
	query := "CREATE TABLE IF NOT EXISTS " + tableName + " (\n"
	query += "\tSmID INTEGER PRIMARY KEY"

	if hasGeometry {
		query += ",\n\tSmGeometry BLOB"
	}

	for _, field := range fieldInfos {
		query += ",\n\t" + field.Name + " " + field.SQLiteType
		if !field.Nullable {
			query += " NOT NULL"
		}
	}

	query += "\n)"

	_, err := i.db.Exec(query)
	return err
}

// DropDatasetTable drops a dataset data table.
func (i *Initializer) DropDatasetTable(tableName string) error {
	query := "DROP TABLE IF EXISTS " + tableName
	_, err := i.db.Exec(query)
	return err
}

// FieldColumn represents a field column definition.
type FieldColumn struct {
	Name       string
	SQLiteType string
	Nullable   bool
}
