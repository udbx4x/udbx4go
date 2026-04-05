package system

import (
	"database/sql"

	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// SmFieldInfoDao provides access to the SmFieldInfo system table.
// SmFieldInfo stores field metadata for datasets.
type SmFieldInfoDao struct {
	db *sql.DB
}

// NewSmFieldInfoDao creates a new SmFieldInfoDao.
func NewSmFieldInfoDao(db *sql.DB) *SmFieldInfoDao {
	return &SmFieldInfoDao{db: db}
}

// SmFieldInfoRecord represents a record in the SmFieldInfo table.
type SmFieldInfoRecord struct {
	SmDatasetID       int
	SmFieldName       string
	SmFieldCaption    sql.NullString
	SmFieldType       int
	SmFieldbRequired  int
	SmFieldDefaultValue sql.NullString
}

// ToFieldInfo converts a SmFieldInfoRecord to FieldInfo.
func (r *SmFieldInfoRecord) ToFieldInfo() *types.FieldInfo {
	info := &types.FieldInfo{
		Name:     r.SmFieldName,
		FieldType: types.FieldType(r.SmFieldType),
		Required: r.SmFieldbRequired != 0,
	}

	if r.SmFieldCaption.Valid {
		info.Alias = &r.SmFieldCaption.String
	}

	return info
}

// ListByDatasetID returns all field records for a dataset.
func (dao *SmFieldInfoDao) ListByDatasetID(datasetID int) ([]*SmFieldInfoRecord, error) {
	query := `
		SELECT SmDatasetID, SmFieldName, SmFieldCaption, SmFieldType, SmFieldbRequired, SmFieldDefaultValue
		FROM SmFieldInfo
		WHERE SmDatasetID = ?
		ORDER BY SmFieldName
	`

	rows, err := dao.db.Query(query, datasetID)
	if err != nil {
		return nil, errors.IOError("failed to query SmFieldInfo", err)
	}
	defer rows.Close()

	return dao.scanRecords(rows)
}

// Insert inserts a new field record.
func (dao *SmFieldInfoDao) Insert(record *SmFieldInfoRecord) error {
	query := `
		INSERT INTO SmFieldInfo (SmDatasetID, SmFieldName, SmFieldCaption, SmFieldType, SmFieldbRequired, SmFieldDefaultValue)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := dao.db.Exec(query,
		record.SmDatasetID,
		record.SmFieldName,
		record.SmFieldCaption,
		record.SmFieldType,
		record.SmFieldbRequired,
		record.SmFieldDefaultValue,
	)
	if err != nil {
		return errors.IOError("failed to insert into SmFieldInfo", err)
	}

	return nil
}

// DeleteByDatasetID deletes all field records for a dataset.
func (dao *SmFieldInfoDao) DeleteByDatasetID(datasetID int) error {
	query := `DELETE FROM SmFieldInfo WHERE SmDatasetID = ?`

	_, err := dao.db.Exec(query, datasetID)
	if err != nil {
		return errors.IOError("failed to delete from SmFieldInfo", err)
	}

	return nil
}

func (dao *SmFieldInfoDao) scanRecords(rows *sql.Rows) ([]*SmFieldInfoRecord, error) {
	var records []*SmFieldInfoRecord

	for rows.Next() {
		record := &SmFieldInfoRecord{}
		err := rows.Scan(
			&record.SmDatasetID,
			&record.SmFieldName,
			&record.SmFieldCaption,
			&record.SmFieldType,
			&record.SmFieldbRequired,
			&record.SmFieldDefaultValue,
		)
		if err != nil {
			return nil, errors.IOError("failed to scan SmFieldInfo row", err)
		}
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.IOError("error iterating SmFieldInfo rows", err)
	}

	return records, nil
}
