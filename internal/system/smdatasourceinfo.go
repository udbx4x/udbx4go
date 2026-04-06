// Package system provides DAOs for UDBX system tables.
package system

import (
	"database/sql"

	"github.com/udbx4x/udbx4go/pkg/errors"
)

// SmDataSourceInfoDao provides access to the SmDataSourceInfo system table.
// SmDataSourceInfo stores file-level metadata.
// Compatible with Java/SuperMap UDBX format.
type SmDataSourceInfoDao struct {
	db *sql.DB
}

// NewSmDataSourceInfoDao creates a new SmDataSourceInfoDao.
func NewSmDataSourceInfoDao(db *sql.DB) *SmDataSourceInfoDao {
	return &SmDataSourceInfoDao{db: db}
}

// SmDataSourceInfoRecord represents a record in the SmDataSourceInfo table.
// Column names match Java/SuperMap UDBX format.
type SmDataSourceInfoRecord struct {
	SmFlag           int
	SmVersion        sql.NullInt32
	SmDsDescription  sql.NullString
	SmProjectInfo    []byte
	SmLastUpdateTime sql.NullString
	SmDataFormat     sql.NullInt32
}

// Get returns the data source info record (there should be only one).
func (dao *SmDataSourceInfoDao) Get() (*SmDataSourceInfoRecord, error) {
	query := `
		SELECT SmFlag, SmVersion, SmDsDescription, SmProjectInfo, SmLastUpdateTime, SmDataFormat
		FROM SmDataSourceInfo
		LIMIT 1
	`

	record := &SmDataSourceInfoRecord{}
	err := dao.db.QueryRow(query).Scan(
		&record.SmFlag,
		&record.SmVersion,
		&record.SmDsDescription,
		&record.SmProjectInfo,
		&record.SmLastUpdateTime,
		&record.SmDataFormat,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, errors.IOError("failed to query SmDataSourceInfo", err)
	}

	return record, nil
}

// Insert inserts a new data source info record.
func (dao *SmDataSourceInfoDao) Insert(record *SmDataSourceInfoRecord) error {
	query := `
		INSERT INTO SmDataSourceInfo (SmFlag, SmVersion, SmDsDescription, SmProjectInfo, SmLastUpdateTime, SmDataFormat)
		VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := dao.db.Exec(query,
		record.SmFlag,
		record.SmVersion,
		record.SmDsDescription,
		record.SmProjectInfo,
		record.SmLastUpdateTime,
		record.SmDataFormat,
	)
	if err != nil {
		return errors.IOError("failed to insert into SmDataSourceInfo", err)
	}

	return nil
}
