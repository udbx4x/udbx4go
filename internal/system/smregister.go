// Package system provides DAOs for UDBX system tables.
package system

import (
	"database/sql"

	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// SmRegisterDao provides access to the SmRegister system table.
// SmRegister stores dataset metadata.
// Compatible with Java/SuperMap UDBX format.
type SmRegisterDao struct {
	db *sql.DB
}

// NewSmRegisterDao creates a new SmRegisterDao.
func NewSmRegisterDao(db *sql.DB) *SmRegisterDao {
	return &SmRegisterDao{db: db}
}

// SmRegisterRecord represents a record in the SmRegister table.
// Column names match Java/SuperMap UDBX format.
type SmRegisterRecord struct {
	SmDatasetID               int
	SmDatasetName             string
	SmTableName               string
	SmOption                  sql.NullInt32
	SmEncType                 sql.NullInt32
	SmParentDTID              int
	SmDatasetType             int
	SmObjectCount             int
	SmLeft                    sql.NullFloat64
	SmRight                   sql.NullFloat64
	SmTop                     sql.NullFloat64
	SmBottom                  sql.NullFloat64
	SmIDColName               sql.NullString
	SmGeoColName              sql.NullString
	SmMinZ                    sql.NullFloat64
	SmMaxZ                    sql.NullFloat64
	SmSRID                    sql.NullInt32
	SmIndexType               sql.NullInt32
	SmToleRanceFuzzy          sql.NullFloat64
	SmToleranceDAngle         sql.NullFloat64
	SmToleranceNodeSnap       sql.NullFloat64
	SmToleranceSmallPolygon   sql.NullFloat64
	SmToleranceGrain          sql.NullFloat64
	SmMaxGeometrySize         int
	SmOptimizeCount           int
	SmOptimizeRatio           sql.NullFloat64
	SmDescription             sql.NullString
	SmExtInfo                 sql.NullString
	SmCreateTime              sql.NullString
	SmLastUpdateTime          sql.NullString
	SmProjectInfo             []byte
}

// ToDatasetInfo converts a SmRegisterRecord to DatasetInfo.
func (r *SmRegisterRecord) ToDatasetInfo() *types.DatasetInfo {
	info := &types.DatasetInfo{
		ID:          r.SmDatasetID,
		Name:        r.SmDatasetName,
		TableName:   r.SmTableName,
		Kind:        types.DatasetKind(r.SmDatasetType),
		ObjectCount: r.SmObjectCount,
	}

	if r.SmSRID.Valid {
		srid := int(r.SmSRID.Int32)
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
		SELECT SmDatasetID, SmDatasetName, SmTableName, SmOption, SmEncType,
		       SmParentDTID, SmDatasetType, SmObjectCount, SmLeft, SmRight,
		       SmTop, SmBottom, SmIDColName, SmGeoColName, SmMinZ, SmMaxZ,
		       SmSRID, SmIndexType, SmToleRanceFuzzy, SmToleranceDAngle,
		       SmToleranceNodeSnap, SmToleranceSmallPolygon, SmToleranceGrain,
		       SmMaxGeometrySize, SmOptimizeCount, SmOptimizeRatio,
		       SmDescription, SmExtInfo, SmCreateTime, SmLastUpdateTime, SmProjectInfo
		FROM SmRegister
		ORDER BY SmDatasetID
	`

	rows, err := dao.db.Query(query)
	if err != nil {
		return nil, errors.IOError("failed to query SmRegister", err)
	}
	defer rows.Close()

	return dao.scanRecords(rows)
}

// GetByID returns a record by SmDatasetID.
func (dao *SmRegisterDao) GetByID(id int) (*SmRegisterRecord, error) {
	query := `
		SELECT SmDatasetID, SmDatasetName, SmTableName, SmOption, SmEncType,
		       SmParentDTID, SmDatasetType, SmObjectCount, SmLeft, SmRight,
		       SmTop, SmBottom, SmIDColName, SmGeoColName, SmMinZ, SmMaxZ,
		       SmSRID, SmIndexType, SmToleRanceFuzzy, SmToleranceDAngle,
		       SmToleranceNodeSnap, SmToleranceSmallPolygon, SmToleranceGrain,
		       SmMaxGeometrySize, SmOptimizeCount, SmOptimizeRatio,
		       SmDescription, SmExtInfo, SmCreateTime, SmLastUpdateTime, SmProjectInfo
		FROM SmRegister
		WHERE SmDatasetID = ?
	`

	row := dao.db.QueryRow(query, id)
	return dao.scanRecord(row)
}

// GetByName returns a record by dataset name.
func (dao *SmRegisterDao) GetByName(name string) (*SmRegisterRecord, error) {
	query := `
		SELECT SmDatasetID, SmDatasetName, SmTableName, SmOption, SmEncType,
		       SmParentDTID, SmDatasetType, SmObjectCount, SmLeft, SmRight,
		       SmTop, SmBottom, SmIDColName, SmGeoColName, SmMinZ, SmMaxZ,
		       SmSRID, SmIndexType, SmToleRanceFuzzy, SmToleranceDAngle,
		       SmToleranceNodeSnap, SmToleranceSmallPolygon, SmToleranceGrain,
		       SmMaxGeometrySize, SmOptimizeCount, SmOptimizeRatio,
		       SmDescription, SmExtInfo, SmCreateTime, SmLastUpdateTime, SmProjectInfo
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
		                       SmParentDTID, SmObjectCount, SmLeft, SmRight,
		                       SmTop, SmBottom, SmSRID, SmMaxGeometrySize,
		                       SmOptimizeCount)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := dao.db.Exec(query,
		record.SmDatasetType, record.SmDatasetName, record.SmTableName,
		record.SmParentDTID, record.SmObjectCount,
		record.SmLeft, record.SmRight, record.SmTop, record.SmBottom,
		record.SmSRID, record.SmMaxGeometrySize, record.SmOptimizeCount,
	)
	if err != nil {
		return errors.IOError("failed to insert into SmRegister", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return errors.IOError("failed to get last insert ID", err)
	}

	record.SmDatasetID = int(id)
	return nil
}

// UpdateObjectCount updates the SmObjectCount for a dataset.
func (dao *SmRegisterDao) UpdateObjectCount(id int, count int) error {
	query := `UPDATE SmRegister SET SmObjectCount = ? WHERE SmDatasetID = ?`

	_, err := dao.db.Exec(query, count, id)
	if err != nil {
		return errors.IOError("failed to update object count", err)
	}

	return nil
}

// UpdateBounds updates the bounds for a dataset.
func (dao *SmRegisterDao) UpdateBounds(id int, minX, minY, maxX, maxY float64) error {
	query := `
		UPDATE SmRegister
		SET SmLeft = ?, SmBottom = ?, SmRight = ?, SmTop = ?
		WHERE SmDatasetID = ?
	`

	_, err := dao.db.Exec(query, minX, minY, maxX, maxY, id)
	if err != nil {
		return errors.IOError("failed to update bounds", err)
	}

	return nil
}

// Delete deletes a record from SmRegister.
func (dao *SmRegisterDao) Delete(id int) error {
	query := `DELETE FROM SmRegister WHERE SmDatasetID = ?`

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
			&record.SmDatasetID, &record.SmDatasetName, &record.SmTableName,
			&record.SmOption, &record.SmEncType, &record.SmParentDTID,
			&record.SmDatasetType, &record.SmObjectCount,
			&record.SmLeft, &record.SmRight, &record.SmTop, &record.SmBottom,
			&record.SmIDColName, &record.SmGeoColName,
			&record.SmMinZ, &record.SmMaxZ, &record.SmSRID, &record.SmIndexType,
			&record.SmToleRanceFuzzy, &record.SmToleranceDAngle,
			&record.SmToleranceNodeSnap, &record.SmToleranceSmallPolygon,
			&record.SmToleranceGrain, &record.SmMaxGeometrySize,
			&record.SmOptimizeCount, &record.SmOptimizeRatio,
			&record.SmDescription, &record.SmExtInfo,
			&record.SmCreateTime, &record.SmLastUpdateTime, &record.SmProjectInfo,
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
		&record.SmDatasetID, &record.SmDatasetName, &record.SmTableName,
		&record.SmOption, &record.SmEncType, &record.SmParentDTID,
		&record.SmDatasetType, &record.SmObjectCount,
		&record.SmLeft, &record.SmRight, &record.SmTop, &record.SmBottom,
		&record.SmIDColName, &record.SmGeoColName,
		&record.SmMinZ, &record.SmMaxZ, &record.SmSRID, &record.SmIndexType,
		&record.SmToleRanceFuzzy, &record.SmToleranceDAngle,
		&record.SmToleranceNodeSnap, &record.SmToleranceSmallPolygon,
		&record.SmToleranceGrain, &record.SmMaxGeometrySize,
		&record.SmOptimizeCount, &record.SmOptimizeRatio,
		&record.SmDescription, &record.SmExtInfo,
		&record.SmCreateTime, &record.SmLastUpdateTime, &record.SmProjectInfo,
	)
	if err == sql.ErrNoRows {
		return nil, errors.NotFoundError("record not found in SmRegister")
	}
	if err != nil {
		return nil, errors.IOError("failed to scan SmRegister row", err)
	}
	return record, nil
}
