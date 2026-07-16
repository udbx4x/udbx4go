package dataset

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/udbx4x/udbx4go/internal/codec"
	"github.com/udbx4x/udbx4go/internal/sqliteutil"
	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// VectorDataset is the base for spatial datasets.
type VectorDataset struct {
	*BaseDataset
	geoCodec *codec.GaiaGeometryCodec
	srid     int
}

type spatialGeometryError struct {
	cause error
}

func (e *spatialGeometryError) Error() string {
	return e.cause.Error()
}

func (e *spatialGeometryError) Unwrap() error {
	return e.cause
}

// NewVectorDataset creates a new vector dataset.
func NewVectorDataset(db *sql.DB, info *types.DatasetInfo) *VectorDataset {
	srid := 0
	if info.SRID != nil {
		srid = *info.SRID
	}

	return &VectorDataset{
		BaseDataset: NewBaseDataset(db, info),
		geoCodec:    codec.NewGaiaGeometryCodec(),
		srid:        srid,
	}
}

// SRID returns the coordinate reference system ID.
func (d *VectorDataset) SRID() int {
	return d.srid
}

// scanFeature scans a row into a Feature.
func (d *VectorDataset) scanFeature(row *sql.Row, geometryType string, id int) (*types.Feature, error) {
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
		return nil, errors.FeatureNotFound(d.Info().Name, id)
	}
	if err != nil {
		return nil, errors.IOError("failed to scan feature", err)
	}

	return d.buildFeature(columns, values, geometryType)
}

// scanFeatures scans multiple rows into Features.
func (d *VectorDataset) scanFeatures(rows *sql.Rows, geometryType string) ([]*types.Feature, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, errors.IOError("failed to get columns", err)
	}

	var features []*types.Feature

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		err := rows.Scan(valuePtrs...)
		if err != nil {
			return nil, errors.IOError("failed to scan feature", err)
		}

		feature, err := d.buildFeature(columns, values, geometryType)
		if err != nil {
			return nil, err
		}

		features = append(features, feature)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.IOError("error iterating features", err)
	}

	return features, nil
}

// buildFeature builds a Feature from column values.
func (d *VectorDataset) buildFeature(columns []string, values []interface{}, geometryType string) (*types.Feature, error) {
	return d.buildFeatureWithMetadata(columns, values, "SmID", "SmGeometry")
}

func (d *VectorDataset) buildFeatureWithMetadata(
	columns []string,
	values []interface{},
	idColumn string,
	geometryColumn string,
) (*types.Feature, error) {
	feature := &types.Feature{
		Attributes: make(map[string]interface{}),
	}

	var geometryBlob []byte
	geometryColumnFound := false

	for i, col := range columns {
		val := values[i]

		switch {
		case strings.EqualFold(col, idColumn):
			switch id := val.(type) {
			case int64:
				feature.ID = int(id)
			case int:
				feature.ID = id
			default:
				return nil, errors.FormatError("feature ID column is not an integer")
			}
		case strings.EqualFold(col, geometryColumn):
			geometryColumnFound = true
			blob, ok := val.([]byte)
			if !ok || len(blob) == 0 {
				return nil, newSpatialGeometryError("feature geometry column is not a non-empty BLOB")
			}
			geometryBlob = blob
		default:
			feature.Attributes[col] = val
		}
	}

	if !geometryColumnFound {
		return nil, newSpatialGeometryError("feature geometry column is missing")
	}

	geometry, err := d.geoCodec.Decode(geometryBlob)
	if err != nil {
		return nil, &spatialGeometryError{cause: errors.FormatError("failed to decode geometry", err)}
	}
	feature.Geometry = geometry

	return feature, nil
}

func newSpatialGeometryError(message string) error {
	return &spatialGeometryError{cause: errors.FormatError(message)}
}

const spatialFeatureIDBatchSize = 500

func (d *VectorDataset) loadFeaturesByIDs(
	ctx context.Context,
	ids []int,
	idColumn string,
	geometryColumn string,
) (map[int]*types.Feature, error) {
	features := make(map[int]*types.Feature, len(ids))
	for start := 0; start < len(ids); start += spatialFeatureIDBatchSize {
		end := start + spatialFeatureIDBatchSize
		if end > len(ids) {
			end = len(ids)
		}
		batch, err := d.loadFeatureBatch(ctx, ids[start:end], idColumn, geometryColumn)
		if err != nil {
			return nil, err
		}
		for id, feature := range batch {
			features[id] = feature
		}
	}
	return features, nil
}

func (d *VectorDataset) loadFeatureBatch(
	ctx context.Context,
	ids []int,
	idColumn string,
	geometryColumn string,
) (map[int]*types.Feature, error) {
	features := make(map[int]*types.Feature, len(ids))
	if len(ids) == 0 {
		return features, nil
	}

	quotedTable, err := sqliteutil.QuoteIdentifier(d.TableName())
	if err != nil {
		return nil, errors.IOError("failed to quote dataset table name", err)
	}
	quotedID, err := sqliteutil.QuoteIdentifier(idColumn)
	if err != nil {
		return nil, errors.IOError("failed to quote feature ID column", err)
	}

	placeholders := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}
	query := "SELECT * FROM " + quotedTable +
		" WHERE " + quotedID + " IN (" + strings.Join(placeholders, ", ") + ")" +
		" ORDER BY " + quotedID

	rows, err := d.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.IOError("failed to load spatial query features", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, errors.IOError("failed to get spatial query columns", err)
	}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePointers := make([]interface{}, len(columns))
		for i := range values {
			valuePointers[i] = &values[i]
		}
		if err := rows.Scan(valuePointers...); err != nil {
			return nil, errors.IOError("failed to scan spatial query feature", err)
		}
		feature, err := d.buildFeatureWithMetadata(columns, values, idColumn, geometryColumn)
		if err != nil {
			return nil, err
		}
		features[feature.ID] = feature
	}
	if err := rows.Err(); err != nil {
		return nil, errors.IOError("error iterating spatial query features", err)
	}
	return features, nil
}

// buildQuery builds a SELECT query with optional filters.
func (d *VectorDataset) buildQuery(opts *types.QueryOptions) (string, []interface{}) {
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
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	return query, args
}

// encodeGeometry encodes a geometry to BLOB.
func (d *VectorDataset) encodeGeometry(geometry types.Geometry) ([]byte, error) {
	// Use geometry SRID if available, otherwise use dataset SRID
	srid := d.srid
	if g, ok := geometry.(interface{ GetSRID() int }); ok {
		if g.GetSRID() != 0 {
			srid = g.GetSRID()
		}
	}

	return d.geoCodec.Encode(geometry, srid)
}
