package dataset

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// TabularDataset represents a non-spatial tabular dataset.
type TabularDataset struct {
	*BaseDataset
}

// NewTabularDataset creates a new tabular dataset.
func NewTabularDataset(db *sql.DB, info *types.DatasetInfo) *TabularDataset {
	return &TabularDataset{
		BaseDataset: NewBaseDataset(db, info),
	}
}

// GetByID returns a record by ID.
func (d *TabularDataset) GetByID(id int) (*types.TabularRecord, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE SmID = ?", d.TableName())

	row := d.DB().QueryRow(query, id)
	return d.scanRecord(row)
}

// List returns a list of records.
func (d *TabularDataset) List(opts *types.QueryOptions) ([]*types.TabularRecord, error) {
	if opts == nil {
		opts = &types.QueryOptions{}
	}

	query := fmt.Sprintf("SELECT * FROM %s", d.TableName())
	var args []interface{}

	// Add WHERE clause for IDs
	if len(opts.IDs) > 0 {
		placeholders := make([]string, len(opts.IDs))
		for i, id := range opts.IDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		query += fmt.Sprintf(" WHERE SmID IN (%s)", strings.Join(placeholders, ", "))
	}

	query += " ORDER BY SmID"

	// Add LIMIT and OFFSET
	// SQLite requires LIMIT when using OFFSET
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
		if opts.Offset > 0 {
			query += fmt.Sprintf(" OFFSET %d", opts.Offset)
		}
	}

	rows, err := d.DB().Query(query, args...)
	if err != nil {
		return nil, errors.IOError("failed to query records", err)
	}
	defer rows.Close()

	return d.scanRecords(rows)
}

// Insert inserts a new record.
func (d *TabularDataset) Insert(record *types.TabularRecord) error {
	fields, err := d.GetFields()
	if err != nil {
		return err
	}

	columns := []string{"SmID"}
	placeholders := []string{"?"}
	values := []interface{}{record.ID}

	for _, field := range fields {
		columns = append(columns, field.Name)
		placeholders = append(placeholders, "?")
		values = append(values, record.Attributes[field.Name])
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		d.TableName(),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	_, err = d.DB().Exec(query, values...)
	if err != nil {
		return errors.IOError("failed to insert record", err)
	}

	return nil
}

// InsertMany inserts multiple records.
func (d *TabularDataset) InsertMany(records []*types.TabularRecord) error {
	tx, err := d.DB().Begin()
	if err != nil {
		return errors.IOError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	for _, record := range records {
		if err := d.Insert(record); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Update updates a record.
func (d *TabularDataset) Update(id int, attributes map[string]interface{}) error {
	if len(attributes) == 0 {
		return nil
	}

	fields, err := d.GetFields()
	if err != nil {
		return err
	}

	// Build valid field set
	validFields := make(map[string]bool)
	for _, f := range fields {
		validFields[f.Name] = true
	}

	// Build SET clause
	var setClauses []string
	var values []interface{}

	for name, value := range attributes {
		if !validFields[name] {
			return errors.FieldNotFound(name)
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", name))
		values = append(values, value)
	}

	values = append(values, id)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE SmID = ?",
		d.TableName(),
		strings.Join(setClauses, ", "))

	result, err := d.DB().Exec(query, values...)
	if err != nil {
		return errors.IOError("failed to update record", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.FeatureNotFound(d.Info().Name, id)
	}

	return nil
}

// Delete deletes a record by ID.
func (d *TabularDataset) Delete(id int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE SmID = ?", d.TableName())

	result, err := d.DB().Exec(query, id)
	if err != nil {
		return errors.IOError("failed to delete record", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.FeatureNotFound(d.Info().Name, id)
	}

	return nil
}

func (d *TabularDataset) scanRecord(row *sql.Row) (*types.TabularRecord, error) {
	// Get column names
	rows, err := d.DB().Query(fmt.Sprintf("SELECT * FROM %s LIMIT 0", d.TableName()))
	if err != nil {
		return nil, errors.IOError("failed to get column names", err)
	}

	columns, err := rows.Columns()
	if err != nil {
		return nil, errors.IOError("failed to get columns", err)
	}
	rows.Close()

	// Create scan targets
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range values {
		valuePtrs[i] = &values[i]
	}

	// Scan the row
	err = row.Scan(valuePtrs...)
	if err == sql.ErrNoRows {
		return nil, errors.FeatureNotFound(d.Info().Name, 0)
	}
	if err != nil {
		return nil, errors.IOError("failed to scan record", err)
	}

	// Build record
	record := &types.TabularRecord{
		Attributes: make(map[string]interface{}),
	}

	for i, col := range columns {
		val := values[i]

		switch col {
		case "SmID":
			if id, ok := val.(int64); ok {
				record.ID = int(id)
			}
		default:
			record.Attributes[col] = val
		}
	}

	return record, nil
}

func (d *TabularDataset) scanRecords(rows *sql.Rows) ([]*types.TabularRecord, error) {
	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, errors.IOError("failed to get columns", err)
	}

	var records []*types.TabularRecord

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		err := rows.Scan(valuePtrs...)
		if err != nil {
			return nil, errors.IOError("failed to scan record", err)
		}

		record := &types.TabularRecord{
			Attributes: make(map[string]interface{}),
		}

		for i, col := range columns {
			val := values[i]

			switch col {
			case "SmID":
				if id, ok := val.(int64); ok {
					record.ID = int(id)
				}
			default:
				record.Attributes[col] = val
			}
		}

		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.IOError("error iterating records", err)
	}

	return records, nil
}
