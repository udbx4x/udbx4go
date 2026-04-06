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
// Compatible with Java/SuperMap UDBX format.
func (i *Initializer) createSmRegister() error {
	query := `
		CREATE TABLE IF NOT EXISTS SmRegister (
			SmDatasetID INTEGER PRIMARY KEY AUTOINCREMENT,
			SmDatasetName TEXT NOT NULL UNIQUE,
			SmTableName TEXT NOT NULL,
			SmOption INTEGER,
			SmEncType INTEGER,
			SmParentDTID INTEGER DEFAULT 0 NOT NULL,
			SmDatasetType INTEGER,
			SmObjectCount INTEGER DEFAULT 0 NOT NULL,
			SmLeft REAL,
			SmRight REAL,
			SmTop REAL,
			SmBottom REAL,
			SmIDColName TEXT,
			SmGeoColName TEXT,
			SmMinZ REAL,
			SmMaxZ REAL,
			SmSRID INTEGER DEFAULT 0,
			SmIndexType INTEGER DEFAULT 1,
			SmToleRanceFuzzy REAL,
			SmToleranceDAngle REAL,
			SmToleranceNodeSnap REAL,
			SmToleranceSmallPolygon REAL,
			SmToleranceGrain REAL,
			SmMaxGeometrySize INTEGER DEFAULT 0 NOT NULL,
			SmOptimizeCount INTEGER DEFAULT 0 NOT NULL,
			SmOptimizeRatio REAL,
			SmDescription TEXT,
			SmExtInfo TEXT,
			SmCreateTime TEXT,
			SmLastUpdateTime TEXT,
			SmProjectInfo BLOB
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
// Compatible with Java/SuperMap UDBX format.
func (i *Initializer) createSmDataSourceInfo() error {
	query := `
		CREATE TABLE IF NOT EXISTS SmDataSourceInfo (
			SmFlag INTEGER PRIMARY KEY DEFAULT 0,
			SmVersion INTEGER,
			SmDsDescription TEXT,
			SmProjectInfo BLOB,
			SmLastUpdateTime DATE DEFAULT CURRENT_TIMESTAMP,
			SmDataFormat INTEGER DEFAULT 0
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
