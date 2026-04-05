// Package system provides DAOs for UDBX system tables.
package system

import (
	"database/sql"

	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// SmRegisterDao provides access to the SmRegister system table.
// SmRegister stores dataset metadata.
type SmRegisterDao struct {
	db *sql.DB
}

// NewSmRegisterDao creates a new SmRegisterDao.
func NewSmRegisterDao(db *sql.DB) *SmRegisterDao {
	return &SmRegisterDao{db: db}
}

// SmRegisterRecord represents a record in the SmRegister table.
type SmRegisterRecord struct {
	SmID          int
	SmDatasetType int
	SmDatasetName string
	SmTableName   string
	SmMaxX        sql.NullFloat64
	SmMaxY        sql.NullFloat64
	SmMinX        sql.NullFloat64
	SmMinY        sql.NullFloat64
	SmCenterX     sql.NullFloat64
	SmCenterY     sql.NullFloat64
	SmSrid        sql.NullInt32
	SmObjectCount int
}

// ToDatasetInfo converts a SmRegisterRecord to DatasetInfo.
func (r *SmRegisterRecord) ToDatasetInfo() *types.DatasetInfo {
	info := &types.DatasetInfo{
		ID:          r.SmID,
		Name:        r.SmDatasetName,
		TableName:   r.SmTableName,
		Kind:        types.DatasetKind(r.SmDatasetType),
		ObjectCount: r.SmObjectCount,
	}

	if r.SmSrid.Valid {
		srid := int(r.SmSrid.Int32)
		info.SRID = &srid
	}

	if types.DatasetKind(r.SmDatasetType).IsSpatial() {
		geoType := types.DatasetKind(r.SmDatasetType).GeometryType()
		info.GeometryType = &geoType
	}

	return info
}

// ListAll returns all records from SmRegister.
func (dao *SmRegisterDao) ListAll() ([]*SmRegisterRecord, error) {
	query := `
		SELECT SmID, SmDatasetType, SmDatasetName, SmTableName,
		       SmMaxX, SmMaxY, SmMinX, SmMinY, SmCenterX, SmCenterY,
		       SmSrid, SmObjectCount
		FROM SmRegister
		ORDER BY SmID
	`

	rows, err := dao.db.Query(query)
	if err != nil {
		return nil, errors.IOError("failed to query SmRegister", err)
	}
	defer rows.Close()

	return dao.scanRecords(rows)
}

// GetByID returns a record by SmID.
func (dao *SmRegisterDao) GetByID(id int) (*SmRegisterRecord, error) {
	query := `
		SELECT SmID, SmDatasetType, SmDatasetName, SmTableName,
		       SmMaxX, SmMaxY, SmMinX, SmMinY, SmCenterX, SmCenterY,
		       SmSmid, SmObjectCount
		FROM SmRegister
		WHERE SmID = ?
	`

	row := dao.db.QueryRow(query, id)
	return dao.scanRecord(row)
}

// GetByName returns a record by dataset name.
func (dao *SmRegisterDao) GetByName(name string) (*SmRegisterRecord, error) {
	query := `
		SELECT SmID, SmDatasetType, SmDatasetName, SmTableName,
		       SmMaxX, SmMaxY, SmMinX, SmMinY, SmCenterX, SmCenterY,
		       SmSrid, SmObjectCount
		FROM SmRegister
		WHERE SmDatasetName = ?
	`

	row := dao.db.QueryRow(query, name)
	record, err := dao.scanRecord(row)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.DatasetNotFound(name)
		}
		return nil, err
	}
	return record, nil
}

// Insert inserts a new record into SmRegister.
func (dao *SmRegisterDao) Insert(record *SmRegisterRecord) error {
	query := `
		INSERT INTO SmRegister (SmDatasetType, SmDatasetName, SmTableName,
		                       SmMaxX, SmMaxY, SmMinX, SmMinY, SmCenterX, SmCenterY,
		                       SmSrid, SmObjectCount)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := dao.db.Exec(query,
		record.SmDatasetType, record.SmDatasetName, record.SmTableName,
		record.SmMaxX, record.SmMaxY, record.SmMinX, record.SmMinY,
		record.SmCenterX, record.SmCenterY,
		record.SmSrid, record.SmObjectCount,
	)
	if err != nil {
		return errors.IOError("failed to insert into SmRegister", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return errors.IOError("failed to get last insert ID", err)
	}

	record.SmID = int(id)
	return nil
}

// UpdateObjectCount updates the SmObjectCount for a dataset.
func (dao *SmRegisterDao) UpdateObjectCount(id int, count int) error {
	query := `UPDATE SmRegister SET SmObjectCount = ? WHERE SmID = ?`

	_, err := dao.db.Exec(query, count, id)
	if err != nil {
		return errors.IOError("failed to update object count", err)
	}

	return nil
}

// UpdateBounds updates the bounds for a dataset.
func (dao *SmRegisterDao) UpdateBounds(id int, minX, minY, maxX, maxY float64) error {
	centerX := (minX + maxX) / 2
	centerY := (minY + maxY) / 2

	query := `
		UPDATE SmRegister
		SET SmMinX = ?, SmMinY = ?, SmMaxX = ?, SmMaxY = ?, SmCenterX = ?, SmCenterY = ?
		WHERE SmID = ?
	`

	_, err := dao.db.Exec(query, minX, minY, maxX, maxY, centerX, centerY, id)
	if err != nil {
		return errors.IOError("failed to update bounds", err)
	}

	return nil
}

// Delete deletes a record from SmRegister.
func (dao *SmRegisterDao) Delete(id int) error {
	query := `DELETE FROM SmRegister WHERE SmID = ?`

	_, err := dao.db.Exec(query, id)
	if err != nil {
		return errors.IOError("failed to delete from SmRegister", err)
	}

	return nil
}

// Exists checks if a dataset with the given name exists.
func (dao *SmRegisterDao) Exists(name string) (bool, error) {
	query := `SELECT 1 FROM SmRegister WHERE SmDatasetName = ?`

	var exists int
	err := dao.db.QueryRow(query, name).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, errors.IOError("failed to check existence", err)
	}

	return true, nil
}

func (dao *SmRegisterDao) scanRecords(rows *sql.Rows) ([]*SmRegisterRecord, error) {
	var records []*SmRegisterRecord

	for rows.Next() {
		record := &SmRegisterRecord{}
		err := rows.Scan(
			&record.SmID, &record.SmDatasetType, &record.SmDatasetName, &record.SmTableName,
			&record.SmMaxX, &record.SmMaxY, &record.SmMinX, &record.SmMinY,
			&record.SmCenterX, &record.SmCenterY,
			&record.SmSrid, &record.SmObjectCount,
		)
		if err != nil {
			return nil, errors.IOError("failed to scan SmRegister row", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.IOError("error iterating SmRegister rows", err)
	}

	return records, nil
}

func (dao *SmRegisterDao) scanRecord(row *sql.Row) (*SmRegisterRecord, error) {
	record := &SmRegisterRecord{}
	err := row.Scan(
		&record.SmID, &record.SmDatasetType, &record.SmDatasetName, &record.SmTableName,
		&record.SmMaxX, &record.SmMaxY, &record.SmMinX, &record.SmMinY,
		&record.SmCenterX, &record.SmCenterY,
		&record.SmSrid, &record.SmObjectCount,
	)
	if err == sql.ErrNoRows {
		return nil, errors.NotFoundError("record not found in SmRegister")
	}
	if err != nil {
		return nil, errors.IOError("failed to scan SmRegister row", err)
	}
	return record, nil
}
