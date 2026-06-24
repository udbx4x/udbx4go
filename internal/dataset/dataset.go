// Package dataset provides UDBX dataset implementations.
package dataset

import (
	"database/sql"
	"fmt"

	"github.com/udbx4x/udbx4go/internal/system"
	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// Dataset is the base interface for all dataset types.
type Dataset interface {
	// Info returns the dataset metadata.
	Info() *types.DatasetInfo
	// Count returns the number of records/features.
	Count() (int, error)
	// GetFields returns the field metadata.
	GetFields() ([]*types.FieldInfo, error)
	// Close releases any resources.
	Close() error
}

// ReadableDataset is a dataset that can be read from.
type ReadableDataset[T any] interface {
	Dataset
	// GetByID returns a record/feature by ID.
	GetByID(id int) (T, error)
	// List returns a list of records/features.
	List(opts *types.QueryOptions) ([]T, error)
}

// WritableDataset is a dataset that can be written to.
type WritableDataset[T any] interface {
	ReadableDataset[T]
	// Insert inserts a new record/feature.
	Insert(item T) error
	// InsertMany inserts multiple records/features.
	InsertMany(items []T) error
	// Update updates a record/feature.
	Update(id int, changes interface{}) error
	// Delete deletes a record/feature by ID.
	Delete(id int) error
}

// BaseDataset provides common functionality for all datasets.
type BaseDataset struct {
	db          *sql.DB
	info        *types.DatasetInfo
	fieldDao    *system.SmFieldInfoDao
	registerDao *system.SmRegisterDao
}

// NewBaseDataset creates a new base dataset.
func NewBaseDataset(db *sql.DB, info *types.DatasetInfo) *BaseDataset {
	return &BaseDataset{
		db:          db,
		info:        info,
		fieldDao:    system.NewSmFieldInfoDao(db),
		registerDao: system.NewSmRegisterDao(db),
	}
}

// Info returns the dataset metadata.
func (d *BaseDataset) Info() *types.DatasetInfo {
	return d.info
}

// Count returns the actual number of rows in the physical dataset table.
func (d *BaseDataset) Count() (int, error) {
	return d.countRows()
}

// GetFields returns the field metadata.
func (d *BaseDataset) GetFields() ([]*types.FieldInfo, error) {
	records, err := d.fieldDao.ListByDatasetID(d.info.ID)
	if err != nil {
		return nil, err
	}

	fields := make([]*types.FieldInfo, len(records))
	for i, record := range records {
		fields[i] = record.ToFieldInfo()
	}

	return fields, nil
}

// Close releases resources (no-op for base dataset).
func (d *BaseDataset) Close() error {
	return nil
}

// DB returns the database connection.
func (d *BaseDataset) DB() *sql.DB {
	return d.db
}

// TableName returns the dataset table name.
func (d *BaseDataset) TableName() string {
	return d.info.TableName
}

// syncObjectCount refreshes SmRegister.SmObjectCount from the actual table row count.
func (d *BaseDataset) syncObjectCount() error {
	count, err := d.countRows()
	if err != nil {
		return err
	}

	if err := d.registerDao.UpdateObjectCount(d.info.ID, count); err != nil {
		return err
	}

	d.info.ObjectCount = count
	return nil
}

func (d *BaseDataset) countRows() (int, error) {
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", d.TableName())

	var count int
	if err := d.db.QueryRow(query).Scan(&count); err != nil {
		return 0, errors.IOError("failed to count dataset rows", err)
	}

	return count, nil
}
