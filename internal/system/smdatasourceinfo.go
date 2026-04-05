package system

import (
	"database/sql"

	"github.com/udbx4x/udbx4go/pkg/errors"
)

// SmDataSourceInfoDao provides access to the SmDataSourceInfo system table.
// SmDataSourceInfo stores file-level metadata.
type SmDataSourceInfoDao struct {
	db *sql.DB
}

// NewSmDataSourceInfoDao creates a new SmDataSourceInfoDao.
func NewSmDataSourceInfoDao(db *sql.DB) *SmDataSourceInfoDao {
	return &SmDataSourceInfoDao{db: db}
}

// SmDataSourceInfoRecord represents a record in the SmDataSourceInfo table.
type SmDataSourceInfoRecord struct {
	SmFileSmid        int
	SmEngineType      sql.NullInt32
	SmFileIdentifier  sql.NullString
	SmFilePwd         sql.NullString
	SmPrjCoordSys     sql.NullString
}

// Get returns the data source info record (there should be only one).
func (dao *SmDataSourceInfoDao) Get() (*SmDataSourceInfoRecord, error) {
	query := `
		SELECT SmFileSmid, SmEngineType, SmFileIdentifier, SmFilePwd, SmPrjCoordSys
		FROM SmDataSourceInfo
		LIMIT 1
	`

	record := &SmDataSourceInfoRecord{}
	err := dao.db.QueryRow(query).Scan(
		&record.SmFileSmid,
		&record.SmEngineType,
		&record.SmFileIdentifier,
		&record.SmFilePwd,
		&record.SmPrjCoordSys,
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
		INSERT INTO SmDataSourceInfo (SmFileSmid, SmEngineType, SmFileIdentifier, SmFilePwd, SmPrjCoordSys)
		VALUES (?, ?, ?, ?, ?)
	`

	_, err := dao.db.Exec(query,
		record.SmFileSmid,
		record.SmEngineType,
		record.SmFileIdentifier,
		record.SmFilePwd,
		record.SmPrjCoordSys,
	)
	if err != nil {
		return errors.IOError("failed to insert into SmDataSourceInfo", err)
	}

	return nil
}
