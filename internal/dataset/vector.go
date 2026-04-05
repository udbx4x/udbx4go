package dataset

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/udbx4x/udbx4go/internal/codec"
	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// VectorDataset is the base for spatial datasets.
type VectorDataset struct {
	*BaseDataset
	geoCodec *codec.GaiaGeometryCodec
	srid     int
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
func (d *VectorDataset) scanFeature(row *sql.Row, geometryType string) (*types.Feature, error) {
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
	feature := &types.Feature{
		Attributes: make(map[string]interface{}),
	}

	var geometryBlob []byte

	for i, col := range columns {
		val := values[i]

		switch col {
		case "SmID":
			if id, ok := val.(int64); ok {
				feature.ID = int(id)
			}
		case "SmGeometry":
			if blob, ok := val.([]byte); ok {
				geometryBlob = blob
			}
		default:
			feature.Attributes[col] = val
		}
	}

	// Decode geometry if present
	if geometryBlob != nil {
		geometry, err := d.geoCodec.Decode(geometryBlob)
		if err != nil {
			return nil, errors.FormatError("failed to decode geometry", err)
		}
		feature.Geometry = geometry
	}

	return feature, nil
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
