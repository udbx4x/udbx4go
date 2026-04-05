package system

import (
	"database/sql"

	"github.com/udbx4x/udbx4go/pkg/errors"
)

// GeometryColumnsDao provides access to the geometry_columns system table.
// geometry_columns stores spatial metadata for geometry columns (OGC/SpatiaLite standard).
type GeometryColumnsDao struct {
	db *sql.DB
}

// NewGeometryColumnsDao creates a new GeometryColumnsDao.
func NewGeometryColumnsDao(db *sql.DB) *GeometryColumnsDao {
	return &GeometryColumnsDao{db: db}
}

// GeometryColumnsRecord represents a record in the geometry_columns table.
type GeometryColumnsRecord struct {
	FTableName        string
	FGeometryColumn   string
	GeometryType      int
	CoordDimension    int
	SRID              int
	SpatialIndexEnabled int
}

// GetByTableName returns the geometry column record for a table.
func (dao *GeometryColumnsDao) GetByTableName(tableName string) (*GeometryColumnsRecord, error) {
	query := `
		SELECT f_table_name, f_geometry_column, geometry_type, coord_dimension, srid, spatial_index_enabled
		FROM geometry_columns
		WHERE f_table_name = ?
	`

	record := &GeometryColumnsRecord{}
	err := dao.db.QueryRow(query, tableName).Scan(
		&record.FTableName,
		&record.FGeometryColumn,
		&record.GeometryType,
		&record.CoordDimension,
		&record.SRID,
		&record.SpatialIndexEnabled,
	)
	if err == sql.ErrNoRows {
		return nil, nil // No geometry column for this table
	}
	if err != nil {
		return nil, errors.IOError("failed to query geometry_columns", err)
	}

	return record, nil
}

// Insert inserts a new geometry column record.
func (dao *GeometryColumnsDao) Insert(record *GeometryColumnsRecord) error {
	query := `
		INSERT INTO geometry_columns (f_table_name, f_geometry_column, geometry_type, coord_dimension, srid, spatial_index_enabled)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := dao.db.Exec(query,
		record.FTableName,
		record.FGeometryColumn,
		record.GeometryType,
		record.CoordDimension,
		record.SRID,
		record.SpatialIndexEnabled,
	)
	if err != nil {
		return errors.IOError("failed to insert into geometry_columns", err)
	}

	return nil
}

// DeleteByTableName deletes a geometry column record.
func (dao *GeometryColumnsDao) DeleteByTableName(tableName string) error {
	query := `DELETE FROM geometry_columns WHERE f_table_name = ?`

	_, err := dao.db.Exec(query, tableName)
	if err != nil {
		return errors.IOError("failed to delete from geometry_columns", err)
	}

	return nil
}
