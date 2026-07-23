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

// CadDataset represents a CAD GeoHeader dataset.
type CadDataset struct {
	*BaseDataset
	cadCodec  *codec.CadGeometryCodec
	textCodec *codec.GeoTextCodec
}

// NewCadDataset creates a new CAD dataset.
func NewCadDataset(db *sql.DB, info *types.DatasetInfo) *CadDataset {
	return &CadDataset{
		BaseDataset: NewBaseDataset(db, info),
		cadCodec:    codec.NewCadGeometryCodec(),
		textCodec:   codec.NewGeoTextCodec(),
	}
}

// GetByID returns a CAD feature by ID.
func (d *CadDataset) GetByID(id int) (*types.Feature, error) {
	quotedTable, err := quoteCadIdentifier(d.TableName(), "CAD dataset table name")
	if err != nil {
		return nil, err
	}
	quotedID, err := quoteCadIdentifier("SmID", "CAD SmID column")
	if err != nil {
		return nil, err
	}
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = ?", quotedTable, quotedID)
	row := d.DB().QueryRow(query, id)
	return d.scanFeature(row, id)
}

// Count returns the number of CAD features.
func (d *CadDataset) Count() (int, error) {
	quotedTable, err := quoteCadIdentifier(d.TableName(), "CAD dataset table name")
	if err != nil {
		return 0, err
	}

	var count int
	if err := d.DB().QueryRow("SELECT COUNT(*) FROM " + quotedTable).Scan(&count); err != nil {
		return 0, errors.IOError("failed to count CAD dataset rows", err)
	}
	return count, nil
}

// List returns CAD features.
func (d *CadDataset) List(opts *types.QueryOptions) ([]*types.Feature, error) {
	return d.ListContext(context.Background(), opts)
}

// ListContext returns CAD features and honors cancellation while querying and decoding.
func (d *CadDataset) ListContext(ctx context.Context, opts *types.QueryOptions) ([]*types.Feature, error) {
	if err := ctx.Err(); err != nil {
		return nil, mapSpatialListError(ctx, err)
	}
	query, args, err := d.buildQuery(opts)
	if err != nil {
		return nil, mapSpatialListError(ctx, err)
	}
	rows, err := d.DB().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, mapSpatialListError(ctx, errors.IOError("failed to query CAD features", err))
	}
	defer rows.Close()

	features, err := d.scanFeaturesContext(ctx, rows)
	if err != nil {
		return nil, mapSpatialListError(ctx, err)
	}
	return features, nil
}

// Insert inserts a CAD feature.
func (d *CadDataset) Insert(feature *types.Feature) error {
	cadGeometry, ok := feature.Geometry.(types.CadGeometry)
	if !ok {
		return errors.ConstraintError("geometry must be CAD GeoHeader geometry")
	}

	geometryBlob, err := d.encodeGeometry(cadGeometry)
	if err != nil {
		return err
	}
	indexKey, err := codec.EncodeEnvelopeIndexKey(cadGeometry.GetBBox(), d.srid())
	if err != nil {
		return err
	}

	fields, err := d.GetFields()
	if err != nil {
		return err
	}

	quotedTable, err := quoteCadIdentifier(d.TableName(), "CAD dataset table name")
	if err != nil {
		return err
	}
	columnNames := []string{"SmID", "SmUserID", "SmGeoType", "SmGeometry", "SmIndexKey"}
	columns := make([]string, 0, len(columnNames)+len(fields))
	for _, name := range columnNames {
		quoted, err := quoteCadIdentifier(name, "CAD system column "+name)
		if err != nil {
			return err
		}
		columns = append(columns, quoted)
	}
	placeholders := []string{"?", "?", "?", "?", "?"}
	values := []interface{}{feature.ID, 0, cadGeometry.CadGeoType(), geometryBlob, indexKey}

	for _, field := range fields {
		quoted, err := quoteCadIdentifier(field.Name, fmt.Sprintf("CAD field name %q", field.Name))
		if err != nil {
			return err
		}
		columns = append(columns, quoted)
		placeholders = append(placeholders, "?")
		values = append(values, feature.Attributes[field.Name])
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quotedTable,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "))

	if _, err := d.DB().Exec(query, values...); err != nil {
		return errors.IOError("failed to insert CAD feature", err)
	}

	return d.syncObjectCount()
}

// InsertMany inserts multiple CAD features.
func (d *CadDataset) InsertMany(features []*types.Feature) error {
	for _, feature := range features {
		if err := d.Insert(feature); err != nil {
			_ = d.syncObjectCount()
			return err
		}
	}
	return d.syncObjectCount()
}

// Update updates a CAD feature.
func (d *CadDataset) Update(id int, changes *FeatureChanges) error {
	fields, err := d.GetFields()
	if err != nil {
		return err
	}

	validFields := make(map[string]string)
	for _, field := range fields {
		quoted, err := quoteCadIdentifier(field.Name, fmt.Sprintf("CAD field name %q", field.Name))
		if err != nil {
			return err
		}
		validFields[field.Name] = quoted
	}

	var setClauses []string
	var values []interface{}

	if changes.Geometry != nil {
		cadGeometry, ok := changes.Geometry.(types.CadGeometry)
		if !ok {
			return errors.ConstraintError("geometry must be CAD GeoHeader geometry")
		}
		geometryBlob, err := d.encodeGeometry(cadGeometry)
		if err != nil {
			return err
		}
		indexKey, err := codec.EncodeEnvelopeIndexKey(cadGeometry.GetBBox(), d.srid())
		if err != nil {
			return err
		}
		quotedGeoType, err := quoteCadIdentifier("SmGeoType", "CAD SmGeoType column")
		if err != nil {
			return err
		}
		quotedGeometry, err := quoteCadIdentifier("SmGeometry", "CAD SmGeometry column")
		if err != nil {
			return err
		}
		quotedIndexKey, err := quoteCadIdentifier("SmIndexKey", "CAD SmIndexKey column")
		if err != nil {
			return err
		}
		setClauses = append(setClauses, quotedGeoType+" = ?", quotedGeometry+" = ?", quotedIndexKey+" = ?")
		values = append(values, cadGeometry.CadGeoType(), geometryBlob, indexKey)
	}

	for name, value := range changes.Attributes {
		quoted, ok := validFields[name]
		if !ok {
			return errors.FieldNotFound(name)
		}
		setClauses = append(setClauses, quoted+" = ?")
		values = append(values, value)
	}

	if len(setClauses) == 0 {
		return nil
	}

	quotedTable, err := quoteCadIdentifier(d.TableName(), "CAD dataset table name")
	if err != nil {
		return err
	}
	quotedID, err := quoteCadIdentifier("SmID", "CAD SmID column")
	if err != nil {
		return err
	}
	values = append(values, id)
	query := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?",
		quotedTable,
		strings.Join(setClauses, ", "),
		quotedID)

	result, err := d.DB().Exec(query, values...)
	if err != nil {
		return errors.IOError("failed to update CAD feature", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.FeatureNotFound(d.Info().Name, id)
	}

	return d.syncObjectCount()
}

// Delete deletes a CAD feature by ID.
func (d *CadDataset) Delete(id int) error {
	quotedTable, err := quoteCadIdentifier(d.TableName(), "CAD dataset table name")
	if err != nil {
		return err
	}
	quotedID, err := quoteCadIdentifier("SmID", "CAD SmID column")
	if err != nil {
		return err
	}
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", quotedTable, quotedID)
	result, err := d.DB().Exec(query, id)
	if err != nil {
		return errors.IOError("failed to delete CAD feature", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return errors.FeatureNotFound(d.Info().Name, id)
	}

	return d.syncObjectCount()
}

func (d *CadDataset) scanFeature(row *sql.Row, id int) (*types.Feature, error) {
	quotedTable, err := quoteCadIdentifier(d.TableName(), "CAD dataset table name")
	if err != nil {
		return nil, err
	}
	rows, err := d.DB().Query(fmt.Sprintf("SELECT * FROM %s LIMIT 0", quotedTable))
	if err != nil {
		return nil, errors.IOError("failed to get CAD column names", err)
	}
	columns, err := rows.Columns()
	rows.Close()
	if err != nil {
		return nil, errors.IOError("failed to get CAD columns", err)
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for index := range values {
		valuePtrs[index] = &values[index]
	}

	if err := row.Scan(valuePtrs...); err == sql.ErrNoRows {
		return nil, errors.FeatureNotFound(d.Info().Name, id)
	} else if err != nil {
		return nil, errors.IOError("failed to scan CAD feature", err)
	}

	return d.buildFeature(columns, values)
}

func (d *CadDataset) scanFeatures(rows *sql.Rows) ([]*types.Feature, error) {
	return d.scanFeaturesContext(context.Background(), rows)
}

func (d *CadDataset) scanFeaturesContext(ctx context.Context, rows *sql.Rows) ([]*types.Feature, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, errors.IOError("failed to get CAD columns", err)
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
			return nil, errors.IOError("failed to scan CAD feature", err)
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
		return nil, errors.IOError("error iterating CAD features", err)
	}
	return features, nil
}

func (d *CadDataset) buildFeature(columns []string, values []interface{}) (*types.Feature, error) {
	feature := &types.Feature{Attributes: make(map[string]interface{})}
	var geometryBlob []byte
	geometryColumnFound := false
	storedGeoType := 0
	storedGeoTypeFound := false
	indexKeyColumnFound := false
	var indexEnvelope *types.BoundingBox

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
				return nil, newSpatialGeometryError("CAD geometry column is not a non-empty BLOB")
			}
			geometryBlob = blob
		case "SmGeoType":
			storedGeoTypeFound = true
			geoType, ok := value.(int64)
			if !ok {
				return nil, newSpatialGeometryError("CAD SmGeoType column is not an integer")
			}
			storedGeoType = int(geoType)
		case "SmIndexKey":
			indexKeyColumnFound = true
			if value == nil {
				continue
			}
			indexKey, ok := value.([]byte)
			if !ok || len(indexKey) == 0 {
				return nil, newSpatialGeometryError("CAD SmIndexKey column is not a non-empty BLOB or NULL")
			}
			envelope, err := codec.ReadGaiaEnvelope(indexKey)
			if err != nil {
				return nil, &spatialGeometryError{cause: errors.FormatError("failed to decode CAD SmIndexKey envelope", err)}
			}
			indexEnvelope = &envelope
		case "SmUserID":
			continue
		default:
			feature.Attributes[column] = value
		}
	}

	if !geometryColumnFound {
		return nil, newSpatialGeometryError("CAD geometry column is missing")
	}
	if !storedGeoTypeFound {
		return nil, newSpatialGeometryError("CAD SmGeoType column is missing")
	}
	if !indexKeyColumnFound {
		return nil, newSpatialGeometryError("CAD SmIndexKey column is missing")
	}
	geometry, err := d.cadCodec.Decode(geometryBlob)
	if err != nil {
		return nil, &spatialGeometryError{cause: errors.FormatError("failed to decode CAD geometry", err)}
	}
	if storedGeoTypeFound && storedGeoType != geometry.CadGeoType() {
		return nil, newSpatialGeometryError(fmt.Sprintf(
			"stored CAD SmGeoType %d does not match decoded geometry type %d",
			storedGeoType,
			geometry.CadGeoType(),
		))
	}
	if text, ok := geometry.(*types.CadTextGeometry); ok && indexEnvelope != nil {
		text.BBox = []float64{
			indexEnvelope.MinX,
			indexEnvelope.MinY,
			indexEnvelope.MaxX,
			indexEnvelope.MaxY,
		}
	}
	feature.Geometry = geometry

	return feature, nil
}

func (d *CadDataset) encodeGeometry(geometry types.CadGeometry) ([]byte, error) {
	switch typed := geometry.(type) {
	case *types.CadPointGeometry:
		if typed == nil {
			return nil, errors.ConstraintError("CAD geometry is required")
		}
	case *types.CadLineGeometry:
		if typed == nil {
			return nil, errors.ConstraintError("CAD geometry is required")
		}
	case *types.CadRegionGeometry:
		if typed == nil {
			return nil, errors.ConstraintError("CAD geometry is required")
		}
	case *types.CadTextGeometry:
		if typed == nil {
			return nil, errors.ConstraintError("CAD geometry is required")
		}
		if typed.CadStyleData != nil {
			return nil, errors.UnsupportedError("CAD Text outer style is unsupported")
		}
		if len(typed.Anchor) < 2 {
			return nil, errors.ConstraintError("CAD Text geometry anchor must contain x and y")
		}
		return d.textCodec.Encode(&types.TextGeometry{
			Type:     "Text",
			Text:     typed.Text,
			Anchor:   typed.Anchor,
			Rotation: typed.Rotation,
			BBox:     typed.BBox,
			GeoType:  typed.CadGeoType(),
			Style:    typed.TextStyle,
			SubTexts: typed.SubTexts,
		})
	default:
		return nil, errors.UnsupportedError("unsupported CAD geometry")
	}
	return d.cadCodec.Encode(geometry)
}

func (d *CadDataset) srid() int {
	if d.Info().SRID == nil {
		return 0
	}
	return *d.Info().SRID
}

func (d *CadDataset) buildQuery(opts *types.QueryOptions) (string, []interface{}, error) {
	if opts == nil {
		opts = &types.QueryOptions{}
	}

	quotedTable, err := quoteCadIdentifier(d.TableName(), "CAD dataset table name")
	if err != nil {
		return "", nil, err
	}
	quotedID, err := quoteCadIdentifier("SmID", "CAD SmID column")
	if err != nil {
		return "", nil, err
	}
	query := fmt.Sprintf("SELECT * FROM %s", quotedTable)
	var args []interface{}

	if len(opts.IDs) > 0 {
		placeholders := make([]string, len(opts.IDs))
		for index, id := range opts.IDs {
			placeholders[index] = "?"
			args = append(args, id)
		}
		query += fmt.Sprintf(" WHERE %s IN (%s)", quotedID, strings.Join(placeholders, ", "))
	}

	query += " ORDER BY " + quotedID
	if opts.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	if opts.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	return query, args, nil
}

func (d *CadDataset) syncObjectCount() error {
	count, err := d.Count()
	if err != nil {
		return err
	}
	if err := d.registerDao.UpdateObjectCount(d.Info().ID, count); err != nil {
		return err
	}
	d.Info().ObjectCount = count
	return nil
}

func quoteCadIdentifier(name string, description string) (string, error) {
	quoted, err := sqliteutil.QuoteIdentifier(name)
	if err != nil {
		return "", errors.FormatError("invalid "+description, err)
	}
	return quoted, nil
}
