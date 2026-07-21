package dataset

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/udbx4x/udbx4go/internal/codec"
	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// TextDataset represents a Text / GeoText dataset.
type TextDataset struct {
	*BaseDataset
	textCodec *codec.GeoTextCodec
}

// NewTextDataset creates a new Text dataset.
func NewTextDataset(db *sql.DB, info *types.DatasetInfo) *TextDataset {
	return &TextDataset{
		BaseDataset: NewBaseDataset(db, info),
		textCodec:   codec.NewGeoTextCodec(),
	}
}

// GetFields returns user-defined fields, excluding Text dataset system fields.
func (d *TextDataset) GetFields() ([]*types.FieldInfo, error) {
	fields, err := d.BaseDataset.GetFields()
	if err != nil {
		return nil, err
	}

	filtered := make([]*types.FieldInfo, 0, len(fields))
	for _, field := range fields {
		switch field.Name {
		case "SmID", "SmUserID", "SmGeometry", "SmIndexKey":
			continue
		default:
			filtered = append(filtered, field)
		}
	}

	return filtered, nil
}

// GetByID returns a Text feature by ID.
func (d *TextDataset) GetByID(id int) (*types.Feature, error) {
	query := fmt.Sprintf("SELECT * FROM %s WHERE SmID = ?", d.TableName())
	row := d.DB().QueryRow(query, id)
	return d.scanFeature(row, id)
}

// List returns Text features.
func (d *TextDataset) List(opts *types.QueryOptions) ([]*types.Feature, error) {
	return d.ListContext(context.Background(), opts)
}

// ListContext returns Text features and honors cancellation while querying and decoding.
func (d *TextDataset) ListContext(ctx context.Context, opts *types.QueryOptions) ([]*types.Feature, error) {
	if err := ctx.Err(); err != nil {
		return nil, mapSpatialListError(ctx, err)
	}
	query, args := d.buildQuery(opts)
	rows, err := d.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, mapSpatialListError(ctx, errors.IOError("failed to query Text features", err))
	}
	defer rows.Close()

	features, err := d.scanFeaturesContext(ctx, rows)
	if err != nil {
		return nil, mapSpatialListError(ctx, err)
	}
	return features, nil
}

// Insert inserts a Text feature.
func (d *TextDataset) Insert(feature *types.Feature) error {
	textGeometry, ok := feature.Geometry.(*types.TextGeometry)
	if !ok {
		return errors.ConstraintError("geometry must be Text")
	}

	fields, err := d.GetFields()
	if err != nil {
		return err
	}

	geometryBlob, err := d.textCodec.Encode(textGeometry)
	if err != nil {
		return err
	}
	indexKey, err := codec.EncodeTextIndexKey(textGeometry, d.srid())
	if err != nil {
		return err
	}

	columns := []string{"SmID", "SmUserID", "SmGeometry", "SmIndexKey"}
	placeholders := []string{"?", "?", "?", "?"}
	values := []interface{}{feature.ID, 0, geometryBlob, indexKey}
	for _, field := range fields {
		columns = append(columns, field.Name)
		placeholders = append(placeholders, "?")
		values = append(values, feature.Attributes[field.Name])
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		d.TableName(),
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	if _, err := d.DB().Exec(query, values...); err != nil {
		return errors.IOError("failed to insert Text feature", err)
	}

	return d.syncObjectCount()
}

// InsertMany inserts multiple Text features.
func (d *TextDataset) InsertMany(features []*types.Feature) error {
	for _, feature := range features {
		if err := d.Insert(feature); err != nil {
			_ = d.syncObjectCount()
			return err
		}
	}
	return d.syncObjectCount()
}

// Update updates a Text feature.
func (d *TextDataset) Update(id int, changes *FeatureChanges) error {
	fields, err := d.GetFields()
	if err != nil {
		return err
	}

	validFields := make(map[string]bool)
	for _, field := range fields {
		validFields[field.Name] = true
	}

	var setClauses []string
	var values []interface{}

	if changes.Geometry != nil {
		textGeometry, ok := changes.Geometry.(*types.TextGeometry)
		if !ok {
			return errors.ConstraintError("geometry must be Text")
		}
		geometryBlob, err := d.textCodec.Encode(textGeometry)
		if err != nil {
			return err
		}
		indexKey, err := codec.EncodeTextIndexKey(textGeometry, d.srid())
		if err != nil {
			return err
		}
		setClauses = append(setClauses, "SmGeometry = ?", "SmIndexKey = ?")
		values = append(values, geometryBlob, indexKey)
	}

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
		return errors.IOError("failed to update Text feature", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.FeatureNotFound(d.Info().Name, id)
	}

	return d.syncObjectCount()
}

// Delete deletes a Text feature by ID.
func (d *TextDataset) Delete(id int) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE SmID = ?", d.TableName())
	result, err := d.DB().Exec(query, id)
	if err != nil {
		return errors.IOError("failed to delete Text feature", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.FeatureNotFound(d.Info().Name, id)
	}

	return d.syncObjectCount()
}

func (d *TextDataset) scanFeature(row *sql.Row, id int) (*types.Feature, error) {
	rows, err := d.DB().Query(fmt.Sprintf("SELECT * FROM %s LIMIT 0", d.TableName()))
	if err != nil {
		return nil, errors.IOError("failed to get Text column names", err)
	}
	columns, err := rows.Columns()
	rows.Close()
	if err != nil {
		return nil, errors.IOError("failed to get Text columns", err)
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for index := range values {
		valuePtrs[index] = &values[index]
	}

	if err := row.Scan(valuePtrs...); err == sql.ErrNoRows {
		return nil, errors.FeatureNotFound(d.Info().Name, id)
	} else if err != nil {
		return nil, errors.IOError("failed to scan Text feature", err)
	}

	return d.buildFeature(columns, values)
}

func (d *TextDataset) scanFeatures(rows *sql.Rows) ([]*types.Feature, error) {
	return d.scanFeaturesContext(context.Background(), rows)
}

func (d *TextDataset) scanFeaturesContext(ctx context.Context, rows *sql.Rows) ([]*types.Feature, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, errors.IOError("failed to get Text columns", err)
	}

	var features []*types.Feature
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !rows.Next() {
			break
		}
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for index := range values {
			valuePtrs[index] = &values[index]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, errors.IOError("failed to scan Text feature", err)
		}
		feature, err := d.buildFeature(columns, values)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		features = append(features, feature)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.IOError("error iterating Text features", err)
	}
	return features, nil
}

func (d *TextDataset) buildFeature(columns []string, values []interface{}) (*types.Feature, error) {
	feature := &types.Feature{Attributes: make(map[string]interface{})}
	var geometryBlob []byte
	geometryColumnFound := false

	for index, column := range columns {
		value := values[index]
		switch column {
		case "SmID":
			if id, ok := value.(int64); ok {
				feature.ID = int(id)
			}
		case "SmGeometry":
			geometryColumnFound = true
			blob, ok := value.([]byte)
			if !ok || len(blob) == 0 {
				return nil, newSpatialGeometryError("Text geometry column is not a non-empty BLOB")
			}
			geometryBlob = blob
		case "SmUserID", "SmIndexKey":
			continue
		default:
			feature.Attributes[column] = value
		}
	}

	if !geometryColumnFound {
		return nil, newSpatialGeometryError("Text geometry column is missing")
	}
	geometry, err := d.textCodec.Decode(geometryBlob)
	if err != nil {
		return nil, &spatialGeometryError{cause: errors.FormatError("failed to decode GeoText geometry", err)}
	}
	feature.Geometry = geometry

	return feature, nil
}

func (d *TextDataset) buildQuery(opts *types.QueryOptions) (string, []interface{}) {
	if opts == nil {
		opts = &types.QueryOptions{}
	}

	query := fmt.Sprintf("SELECT * FROM %s", d.TableName())
	var args []interface{}

	if len(opts.IDs) > 0 {
		placeholders := make([]string, len(opts.IDs))
		for index, id := range opts.IDs {
			placeholders[index] = "?"
			args = append(args, id)
		}
		query += fmt.Sprintf(" WHERE SmID IN (%s)", strings.Join(placeholders, ", "))
	}

	query += " ORDER BY SmID"
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	return query, args
}

func (d *TextDataset) srid() int {
	if d.Info().SRID == nil {
		return 0
	}
	return *d.Info().SRID
}
