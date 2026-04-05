package dataset

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// RegionDataset represents a 2D region (MultiPolygon) dataset.
type RegionDataset struct {
	*VectorDataset
}

// NewRegionDataset creates a new region dataset.
func NewRegionDataset(db *sql.DB, info *types.DatasetInfo) *RegionDataset {
	return &RegionDataset{
		VectorDataset: NewVectorDataset(db, info),
	}
}

// GetByID returns a feature by ID.
func (d *RegionDataset) GetByID(id int) (*types.Feature, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE SmID = ?", d.TableName())

	row := d.DB().QueryRow(query, id)
	return d.scanFeature(row, "MultiPolygon")
}

// List returns a list of features.
func (d *RegionDataset) List(opts *types.QueryOptions) ([]*types.Feature, error) {
	query, args := d.buildQuery(opts)

	rows, err := d.DB().Query(query, args...)
	if err != nil {
		return nil, errors.IOError("failed to query features", err)
	}
	defer rows.Close()

	return d.scanFeatures(rows, "MultiPolygon")
}

// Insert inserts a new region feature.
func (d *RegionDataset) Insert(feature *types.Feature) error {
	// Validate geometry type
	if _, ok := feature.Geometry.(*types.MultiPolygonGeometry); !ok {
		return errors.ConstraintError("geometry must be MultiPolygon")
	}

	fields, err := d.GetFields()
	if err != nil {
		return err
	}

	// Build query
	columns := []string{"SmID", "SmGeometry"}
	placeholders := []string{"?", "?"}
	values := []interface{}{feature.ID}

	// Encode geometry
	geomBlob, err := d.encodeGeometry(feature.Geometry)
	if err != nil {
		return err
	}
	values = append(values, geomBlob)

	// Add attribute columns
	for _, field := range fields {
		columns = append(columns, field.Name)
		placeholders = append(placeholders, "?")
		values = append(values, feature.Attributes[field.Name])
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		d.TableName(),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	_, err = d.DB().Exec(query, values...)
	if err != nil {
		return errors.IOError("failed to insert feature", err)
	}

	return nil
}

// InsertMany inserts multiple features.
func (d *RegionDataset) InsertMany(features []*types.Feature) error {
	tx, err := d.DB().Begin()
	if err != nil {
		return errors.IOError("failed to begin transaction", err)
	}
	defer tx.Rollback()

	for _, feature := range features {
		if err := d.Insert(feature); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// Update updates a feature.
func (d *RegionDataset) Update(id int, changes *FeatureChanges) error {
	fields, err := d.GetFields()
	if err != nil {
		return err
	}

	// Build valid field set
	validFields := make(map[string]bool)
	for _, f := range fields {
		validFields[f.Name] = true
	}

	var setClauses []string
	var values []interface{}

	// Update geometry if provided
	if changes.Geometry != nil {
		if _, ok := changes.Geometry.(*types.MultiPolygonGeometry); !ok {
			return errors.ConstraintError("geometry must be MultiPolygon")
		}

		geomBlob, err := d.encodeGeometry(changes.Geometry)
		if err != nil {
			return err
		}

		setClauses = append(setClauses, "SmGeometry = ?")
		values = append(values, geomBlob)
	}

	// Update attributes
	for name, value := range changes.Attributes {
		if !validFields[name] {
			return errors.FieldNotFound(name)
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", name))
		values = append(values, value)
	}

	if len(setClauses) == 0 {
		return nil
	}

	values = append(values, id)

	query := fmt.Sprintf("UPDATE %s SET %s WHERE SmID = ?",
		d.TableName(),
		strings.Join(setClauses, ", "))

	result, err := d.DB().Exec(query, values...)
	if err != nil {
		return errors.IOError("failed to update feature", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.FeatureNotFound(d.Info().Name, id)
	}

	return nil
}

// Delete deletes a feature by ID.
func (d *RegionDataset) Delete(id int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE SmID = ?", d.TableName())

	result, err := d.DB().Exec(query, id)
	if err != nil {
		return errors.IOError("failed to delete feature", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.FeatureNotFound(d.Info().Name, id)
	}

	return nil
}

