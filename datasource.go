package udbx4go

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"

	"github.com/udbx4x/udbx4go/internal/dataset"
	"github.com/udbx4x/udbx4go/internal/schema"
	"github.com/udbx4x/udbx4go/internal/system"
	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// DataSource represents a UDBX data source.
type DataSource struct {
	db           *sql.DB
	registerDao  *system.SmRegisterDao
	fieldInfoDao *system.SmFieldInfoDao
	geoColsDao   *system.GeometryColumnsDao
}

// Open opens an existing UDBX file.
func Open(path string) (*DataSource, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, errors.IOErrorf("failed to open UDBX file: %s: %v", path, err)
	}

	// Verify it's a valid UDBX file by checking for SmRegister table
	var count int
	err = db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='SmRegister'").Scan(&count)
	if err != nil {
		db.Close()
		return nil, errors.IOError("failed to check for SmRegister table", err)
	}
	if count == 0 {
		db.Close()
		return nil, errors.FormatError("not a valid UDBX file: SmRegister table not found")
	}

	return newDataSource(db), nil
}

// Create creates a new UDBX file.
func Create(path string) (*DataSource, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, errors.IOErrorf("failed to create UDBX file: %s: %v", path, err)
	}

	// Initialize schema
	initializer := schema.NewInitializer(db)
	if err := initializer.Initialize(); err != nil {
		db.Close()
		return nil, err
	}

	return newDataSource(db), nil
}

// newDataSource creates a new DataSource instance.
func newDataSource(db *sql.DB) *DataSource {
	return &DataSource{
		db:           db,
		registerDao:  system.NewSmRegisterDao(db),
		fieldInfoDao: system.NewSmFieldInfoDao(db),
		geoColsDao:   system.NewGeometryColumnsDao(db),
	}
}

// Close closes the data source.
func (ds *DataSource) Close() error {
	if ds.db != nil {
		return ds.db.Close()
	}
	return nil
}

// ListDatasets returns a list of all datasets in the data source.
func (ds *DataSource) ListDatasets() ([]*types.DatasetInfo, error) {
	records, err := ds.registerDao.ListAll()
	if err != nil {
		return nil, err
	}

	infos := make([]*types.DatasetInfo, len(records))
	for i, record := range records {
		infos[i] = record.ToDatasetInfo()
	}

	return infos, nil
}

// GetDataset returns a dataset by name (generic interface).
func (ds *DataSource) GetDataset(name string) (dataset.Dataset, error) {
	record, err := ds.registerDao.GetByName(name)
	if err != nil {
		return nil, err
	}

	info := record.ToDatasetInfo()

	switch info.Kind {
	case types.DatasetKindTabular:
		return dataset.NewTabularDataset(ds.db, info), nil
	case types.DatasetKindPoint:
		return dataset.NewPointDataset(ds.db, info), nil
	case types.DatasetKindLine:
		return dataset.NewLineDataset(ds.db, info), nil
	case types.DatasetKindRegion:
		return dataset.NewRegionDataset(ds.db, info), nil
	case types.DatasetKindPointZ:
		return dataset.NewPointZDataset(ds.db, info), nil
	case types.DatasetKindLineZ:
		return dataset.NewLineZDataset(ds.db, info), nil
	case types.DatasetKindRegionZ:
		return dataset.NewRegionZDataset(ds.db, info), nil
	default:
		return nil, errors.UnsupportedError(fmt.Sprintf("dataset kind '%s' is not supported", info.Kind.String()))
	}
}

// GetTabularDataset returns a tabular dataset by name.
func (ds *DataSource) GetTabularDataset(name string) (*dataset.TabularDataset, error) {
	d, err := ds.GetDataset(name)
	if err != nil {
		return nil, err
	}

	tabular, ok := d.(*dataset.TabularDataset)
	if !ok {
		return nil, errors.FormatError(fmt.Sprintf("dataset '%s' is not a tabular dataset", name))
	}

	return tabular, nil
}

// GetPointDataset returns a point dataset by name.
func (ds *DataSource) GetPointDataset(name string) (*dataset.PointDataset, error) {
	d, err := ds.GetDataset(name)
	if err != nil {
		return nil, err
	}

	point, ok := d.(*dataset.PointDataset)
	if !ok {
		return nil, errors.FormatError(fmt.Sprintf("dataset '%s' is not a point dataset", name))
	}

	return point, nil
}

// GetLineDataset returns a line dataset by name.
func (ds *DataSource) GetLineDataset(name string) (*dataset.LineDataset, error) {
	d, err := ds.GetDataset(name)
	if err != nil {
		return nil, err
	}

	line, ok := d.(*dataset.LineDataset)
	if !ok {
		return nil, errors.FormatError(fmt.Sprintf("dataset '%s' is not a line dataset", name))
	}

	return line, nil
}

// GetRegionDataset returns a region dataset by name.
func (ds *DataSource) GetRegionDataset(name string) (*dataset.RegionDataset, error) {
	d, err := ds.GetDataset(name)
	if err != nil {
		return nil, err
	}

	region, ok := d.(*dataset.RegionDataset)
	if !ok {
		return nil, errors.FormatError(fmt.Sprintf("dataset '%s' is not a region dataset", name))
	}

	return region, nil
}

// CreateTabularDataset creates a new tabular dataset.
func (ds *DataSource) CreateTabularDataset(name string, fields []*types.FieldInfo) (*dataset.TabularDataset, error) {
	return ds.createTabularDatasetInternal(name, types.DatasetKindTabular, 0, fields)
}

// CreatePointDataset creates a new 2D point dataset.
func (ds *DataSource) CreatePointDataset(name string, srid int, fields []*types.FieldInfo) (*dataset.PointDataset, error) {
	return ds.createPointDatasetInternal(name, types.DatasetKindPoint, srid, fields)
}

// CreateLineDataset creates a new 2D line dataset.
func (ds *DataSource) CreateLineDataset(name string, srid int, fields []*types.FieldInfo) (*dataset.LineDataset, error) {
	return ds.createLineDatasetInternal(name, types.DatasetKindLine, srid, fields)
}

// CreateRegionDataset creates a new 2D region dataset.
func (ds *DataSource) CreateRegionDataset(name string, srid int, fields []*types.FieldInfo) (*dataset.RegionDataset, error) {
	return ds.createRegionDatasetInternal(name, types.DatasetKindRegion, srid, fields)
}

// CreatePointZDataset creates a new 3D point dataset.
func (ds *DataSource) CreatePointZDataset(name string, srid int, fields []*types.FieldInfo) (*dataset.PointZDataset, error) {
	d, err := ds.createPointDatasetInternal(name, types.DatasetKindPointZ, srid, fields)
	if err != nil {
		return nil, err
	}
	return dataset.NewPointZDataset(ds.db, d.Info()), nil
}

// CreateLineZDataset creates a new 3D line dataset.
func (ds *DataSource) CreateLineZDataset(name string, srid int, fields []*types.FieldInfo) (*dataset.LineZDataset, error) {
	d, err := ds.createLineDatasetInternal(name, types.DatasetKindLineZ, srid, fields)
	if err != nil {
		return nil, err
	}
	return dataset.NewLineZDataset(ds.db, d.Info()), nil
}

// CreateRegionZDataset creates a new 3D region dataset.
func (ds *DataSource) CreateRegionZDataset(name string, srid int, fields []*types.FieldInfo) (*dataset.RegionZDataset, error) {
	d, err := ds.createRegionDatasetInternal(name, types.DatasetKindRegionZ, srid, fields)
	if err != nil {
		return nil, err
	}
	return dataset.NewRegionZDataset(ds.db, d.Info()), nil
}

// Internal creation methods

func (ds *DataSource) createTabularDatasetInternal(name string, kind types.DatasetKind, srid int, fields []*types.FieldInfo) (*dataset.TabularDataset, error) {
	// Check if dataset already exists
	exists, err := ds.registerDao.Exists(name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.ConstraintError(fmt.Sprintf("dataset '%s' already exists", name))
	}

	tableName := generateTableName(name)

	// Create data table
	initializer := schema.NewInitializer(ds.db)

	fieldColumns := make([]schema.FieldColumn, len(fields))
	for i, f := range fields {
		fieldColumns[i] = schema.FieldColumn{
			Name:       f.Name,
			SQLiteType: f.FieldType.SQLiteType(),
			Nullable:   f.Nullable,
		}
	}

	if err := initializer.CreateDatasetTable(tableName, false, fieldColumns); err != nil {
		return nil, errors.IOError("failed to create dataset table", err)
	}

	// Insert into SmRegister
	record := &system.SmRegisterRecord{
		SmDatasetType: int(kind),
		SmDatasetName: name,
		SmTableName:   tableName,
		SmObjectCount: 0,
	}

	if srid > 0 {
		record.SmSRID = sql.NullInt32{Int32: int32(srid), Valid: true}
	}

	if err := ds.registerDao.Insert(record); err != nil {
		return nil, err
	}

	// Insert field info
	for _, field := range fields {
		fieldRecord := &system.SmFieldInfoRecord{
			SmDatasetID:      record.SmDatasetID,
			SmFieldName:      field.Name,
			SmFieldType:      int(field.FieldType),
			SmFieldbRequired: boolToInt(field.Required),
		}
		if field.Alias != nil {
			fieldRecord.SmFieldCaption = sql.NullString{String: *field.Alias, Valid: true}
		}
		if err := ds.fieldInfoDao.Insert(fieldRecord); err != nil {
			return nil, err
		}
	}

	info := record.ToDatasetInfo()
	return dataset.NewTabularDataset(ds.db, info), nil
}

func (ds *DataSource) createPointDatasetInternal(name string, kind types.DatasetKind, srid int, fields []*types.FieldInfo) (*dataset.PointDataset, error) {
	// Check if dataset already exists
	exists, err := ds.registerDao.Exists(name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.ConstraintError(fmt.Sprintf("dataset '%s' already exists", name))
	}

	tableName := generateTableName(name)

	// Create data table
	initializer := schema.NewInitializer(ds.db)

	fieldColumns := make([]schema.FieldColumn, len(fields))
	for i, f := range fields {
		fieldColumns[i] = schema.FieldColumn{
			Name:       f.Name,
			SQLiteType: f.FieldType.SQLiteType(),
			Nullable:   f.Nullable,
		}
	}

	if err := initializer.CreateDatasetTable(tableName, true, fieldColumns); err != nil {
		return nil, errors.IOError("failed to create dataset table", err)
	}

	// Insert into SmRegister
	record := &system.SmRegisterRecord{
		SmDatasetType: int(kind),
		SmDatasetName: name,
		SmTableName:   tableName,
		SmObjectCount: 0,
	}

	if srid > 0 {
		record.SmSRID = sql.NullInt32{Int32: int32(srid), Valid: true}
	}

	if err := ds.registerDao.Insert(record); err != nil {
		return nil, err
	}

	// Insert into geometry_columns
	geoRecord := &system.GeometryColumnsRecord{
		FTableName:          strings.ToLower(tableName),
		FGeometryColumn:     "SmGeometry",
		GeometryType:        kind.GeometryType(),
		CoordDimension:      kind.CoordDimension(),
		SRID:                srid,
		SpatialIndexEnabled: 0,
	}
	if err := ds.geoColsDao.Insert(geoRecord); err != nil {
		return nil, err
	}

	// Insert field info
	for _, field := range fields {
		fieldRecord := &system.SmFieldInfoRecord{
			SmDatasetID:      record.SmDatasetID,
			SmFieldName:      field.Name,
			SmFieldType:      int(field.FieldType),
			SmFieldbRequired: boolToInt(field.Required),
		}
		if field.Alias != nil {
			fieldRecord.SmFieldCaption = sql.NullString{String: *field.Alias, Valid: true}
		}
		if err := ds.fieldInfoDao.Insert(fieldRecord); err != nil {
			return nil, err
		}
	}

	info := record.ToDatasetInfo()
	return dataset.NewPointDataset(ds.db, info), nil
}

func (ds *DataSource) createLineDatasetInternal(name string, kind types.DatasetKind, srid int, fields []*types.FieldInfo) (*dataset.LineDataset, error) {
	// Reuse point dataset creation then return line dataset
	pointDS, err := ds.createPointDatasetInternal(name, kind, srid, fields)
	if err != nil {
		return nil, err
	}
	return dataset.NewLineDataset(ds.db, pointDS.Info()), nil
}

func (ds *DataSource) createRegionDatasetInternal(name string, kind types.DatasetKind, srid int, fields []*types.FieldInfo) (*dataset.RegionDataset, error) {
	// Reuse point dataset creation then return region dataset
	pointDS, err := ds.createPointDatasetInternal(name, kind, srid, fields)
	if err != nil {
		return nil, err
	}
	return dataset.NewRegionDataset(ds.db, pointDS.Info()), nil
}

// generateTableName generates a safe table name from dataset name.
func generateTableName(name string) string {
	// Remove file extension if present
	name = strings.TrimSuffix(name, filepath.Ext(name))

	// Replace invalid characters
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")

	return name
}

// boolToInt converts bool to int (1 for true, 0 for false).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
