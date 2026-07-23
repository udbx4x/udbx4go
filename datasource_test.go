package udbx4go

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	internaldataset "github.com/udbx4x/udbx4go/internal/dataset"
	"github.com/udbx4x/udbx4go/internal/sqliteutil"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestCreateAndOpen(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	// Test Create
	ds, err := Create(udbxPath)
	require.NoError(t, err)
	require.NotNil(t, ds)
	defer ds.Close()

	// Verify file was created
	_, err = os.Stat(udbxPath)
	require.NoError(t, err)

	// Close and reopen
	ds.Close()

	// Test Open
	ds2, err := Open(udbxPath)
	require.NoError(t, err)
	require.NotNil(t, ds2)
	defer ds2.Close()
}

func TestOpen_NotUdbxFile(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "not_udbx.db")

	// Create a plain SQLite file without UDBX tables
	ds, err := Create(tempFile)
	require.NoError(t, err)
	ds.Close()

	// Remove the SmRegister table to make it invalid
	db, err := sql.Open("sqlite3", tempFile)
	require.NoError(t, err)
	_, err = db.Exec("DROP TABLE SmRegister")
	require.NoError(t, err)
	db.Close()

	// Try to open
	ds2, err := Open(tempFile)
	assert.Error(t, err)
	assert.Nil(t, ds2)
	assert.Contains(t, err.Error(), "not a valid UDBX file")
}

func TestDataSource_ListDatasets(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	ds, err := Create(udbxPath)
	require.NoError(t, err)
	defer ds.Close()

	// Initially empty
	datasets, err := ds.ListDatasets()
	require.NoError(t, err)
	assert.Empty(t, datasets)

	// Create some datasets
	_, err = ds.CreatePointDataset("cities", 4326, nil)
	require.NoError(t, err)

	_, err = ds.CreateTabularDataset("countries", nil)
	require.NoError(t, err)

	// List again
	datasets, err = ds.ListDatasets()
	require.NoError(t, err)
	assert.Len(t, datasets, 2)

	// Verify dataset info
	names := make([]string, len(datasets))
	for i, d := range datasets {
		names[i] = d.Name
	}
	assert.Contains(t, names, "cities")
	assert.Contains(t, names, "countries")
}

func TestDataSource_CreateTabularDataset(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	ds, err := Create(udbxPath)
	require.NoError(t, err)
	defer ds.Close()

	fields := []*types.FieldInfo{
		{Name: "name", FieldType: types.FieldTypeText, Nullable: false},
		{Name: "population", FieldType: types.FieldTypeInt32, Nullable: true},
	}

	tabular, err := ds.CreateTabularDataset("countries", fields)
	require.NoError(t, err)
	require.NotNil(t, tabular)

	// Verify dataset info
	assert.Equal(t, "countries", tabular.Info().Name)
	assert.Equal(t, types.DatasetKindTabular, tabular.Info().Kind)

	// Get fields
	retrievedFields, err := tabular.GetFields()
	require.NoError(t, err)
	assert.Len(t, retrievedFields, 2)
}

func TestDataSource_CreatePointDataset(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	ds, err := Create(udbxPath)
	require.NoError(t, err)
	defer ds.Close()

	fields := []*types.FieldInfo{
		{Name: "name", FieldType: types.FieldTypeText, Nullable: true},
	}

	pointDS, err := ds.CreatePointDataset("cities", 4326, fields)
	require.NoError(t, err)
	require.NotNil(t, pointDS)

	// Verify dataset info
	assert.Equal(t, "cities", pointDS.Info().Name)
	assert.Equal(t, types.DatasetKindPoint, pointDS.Info().Kind)
	assert.Equal(t, 4326, pointDS.SRID())
}

func TestDataSourceCreateCadDatasetWritesWhitepaperSchema(t *testing.T) {
	ds, err := Create(filepath.Join(t.TempDir(), "cad.udbx"))
	require.NoError(t, err)
	defer ds.Close()

	alias := "Display name"
	cad, err := ds.CreateCadDataset("CAD-Layer", []*types.FieldInfo{
		{Name: "name", Alias: &alias, FieldType: types.FieldTypeText, Required: true, Nullable: false},
		{Name: "level", FieldType: types.FieldTypeInt32, Nullable: true},
	})
	require.NoError(t, err)

	columns, err := sqliteTableColumns(ds.db, cad.Info().TableName)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"SmID", "SmUserID", "SmGeoType", "SmGeometry", "SmIndexKey", "name", "level",
	}, columns)

	record, err := ds.registerDao.GetByName("CAD-Layer")
	require.NoError(t, err)
	assert.Equal(t, int(types.DatasetKindCAD), record.SmDatasetType)
	assert.Equal(t, "CAD_Layer", record.SmTableName)
	assert.Equal(t, 0, record.SmObjectCount)
	assert.Equal(t, sql.NullString{String: "SmID", Valid: true}, record.SmIDColName)
	assert.Equal(t, sql.NullString{String: "SmGeometry", Valid: true}, record.SmGeoColName)
	assert.Equal(t, sql.NullInt32{Int32: 0, Valid: true}, record.SmSRID)
	assert.Equal(t, sql.NullInt32{Int32: 0, Valid: true}, record.SmIndexType)

	geometry, err := ds.geoColsDao.GetByTableName("cad_layer")
	require.NoError(t, err)
	require.NotNil(t, geometry)
	assert.Equal(t, "cad_layer", geometry.FTableName)
	assert.Equal(t, "SmIndexKey", geometry.FGeometryColumn)
	assert.Equal(t, 3, geometry.GeometryType)
	assert.Equal(t, 2, geometry.CoordDimension)
	assert.Equal(t, 0, geometry.SRID)
	assert.Equal(t, 0, geometry.SpatialIndexEnabled)

	fieldRecords, err := ds.fieldInfoDao.ListByDatasetID(record.SmDatasetID)
	require.NoError(t, err)
	require.Len(t, fieldRecords, 2)
	assert.Equal(t, "level", fieldRecords[0].SmFieldName)
	assert.Equal(t, int(types.FieldTypeInt32), fieldRecords[0].SmFieldType)
	assert.Equal(t, 0, fieldRecords[0].SmFieldbRequired)
	assert.Equal(t, "name", fieldRecords[1].SmFieldName)
	assert.Equal(t, int(types.FieldTypeText), fieldRecords[1].SmFieldType)
	assert.Equal(t, 1, fieldRecords[1].SmFieldbRequired)
	assert.Equal(t, sql.NullString{String: alias, Valid: true}, fieldRecords[1].SmFieldCaption)
}

func TestDataSourceCreateCadDatasetQuotesMaliciousFieldName(t *testing.T) {
	ds, err := Create(filepath.Join(t.TempDir(), "cad-quoted-field.udbx"))
	require.NoError(t, err)
	defer ds.Close()

	fieldName := "name]); DROP TABLE SmRegister;--"
	cad, err := ds.CreateCadDataset("cad quoted field", []*types.FieldInfo{
		{Name: fieldName, FieldType: types.FieldTypeText, Nullable: true},
	})
	require.NoError(t, err)

	columns, err := sqliteTableColumns(ds.db, cad.Info().TableName)
	require.NoError(t, err)
	assert.Contains(t, columns, fieldName)
	assertDatabaseCount(t, ds.db, "SELECT COUNT(*) FROM SmRegister", 1)
}

func TestDataSourceCreateCadDatasetCRUDQuotesIdentifiers(t *testing.T) {
	ds, err := Create(filepath.Join(t.TempDir(), "cad-identifier-crud.udbx"))
	require.NoError(t, err)
	defer ds.Close()

	cad, err := ds.CreateCadDataset(`select"中文; DROP TABLE SmRegister;--`, []*types.FieldInfo{
		{Name: "from", FieldType: types.FieldTypeText, Nullable: true},
		{Name: `quoted"field`, FieldType: types.FieldTypeInt32, Nullable: true},
		{Name: "中文字段", FieldType: types.FieldTypeText, Nullable: true},
	})
	require.NoError(t, err)

	require.NoError(t, cad.Insert(&types.Feature{
		ID:       1,
		Geometry: &types.CadPointGeometry{XCoord: -2, YCoord: 3},
		Attributes: map[string]interface{}{
			"from":         "inserted",
			`quoted"field`: 7,
			"中文字段":         "初始值",
		},
	}))

	feature, err := cad.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, "inserted", feature.Attributes["from"])
	assert.EqualValues(t, 7, feature.Attributes[`quoted"field`])
	assert.Equal(t, "初始值", feature.Attributes["中文字段"])

	require.NoError(t, cad.Update(1, &internaldataset.FeatureChanges{
		Geometry: &types.CadLineGeometry{
			NumSub:         1,
			SubPointCounts: []int{2},
			Coordinates:    [][2]float64{{-4, -5}, {6, 7}},
		},
		Attributes: map[string]interface{}{
			"from":         "updated",
			`quoted"field`: 8,
			"中文字段":         "更新值",
		},
	}))

	features, err := cad.List(nil)
	require.NoError(t, err)
	require.Len(t, features, 1)
	assert.Equal(t, "updated", features[0].Attributes["from"])
	assert.EqualValues(t, 8, features[0].Attributes[`quoted"field`])
	assert.Equal(t, "更新值", features[0].Attributes["中文字段"])

	require.NoError(t, cad.Delete(1))
	count, err := cad.Count()
	require.NoError(t, err)
	assert.Zero(t, count)
	assertDatabaseCount(t, ds.db, "SELECT COUNT(*) FROM SmRegister", 1)
}

func TestDataSourceCreateCadDatasetRollsBackOnNULFieldName(t *testing.T) {
	ds, err := Create(filepath.Join(t.TempDir(), "cad-nul-field.udbx"))
	require.NoError(t, err)
	defer ds.Close()

	_, err = ds.CreateCadDataset("cad nul field", []*types.FieldInfo{
		{Name: "name\x00archive", FieldType: types.FieldTypeText, Nullable: true},
	})
	require.Error(t, err)
	assert.True(t, IsIOError(err))
	assert.Contains(t, err.Error(), "NUL")

	assertDatabaseCount(t, ds.db, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", 0, "cad_nul_field")
	assertDatabaseCount(t, ds.db, "SELECT COUNT(*) FROM SmRegister", 0)
	assertDatabaseCount(t, ds.db, "SELECT COUNT(*) FROM geometry_columns", 0)
	assertDatabaseCount(t, ds.db, "SELECT COUNT(*) FROM SmFieldInfo", 0)
}

func TestDataSourceCreateCadDatasetRollsBackOnFieldMetadataFailure(t *testing.T) {
	ds, err := Create(filepath.Join(t.TempDir(), "cad-rollback.udbx"))
	require.NoError(t, err)
	defer ds.Close()

	_, err = ds.db.Exec(`
		CREATE TRIGGER fail_cad_field_metadata
		BEFORE INSERT ON SmFieldInfo
		BEGIN
			SELECT RAISE(ABORT, 'controlled SmFieldInfo failure');
		END
	`)
	require.NoError(t, err)

	_, err = ds.CreateCadDataset("cad atomic", []*types.FieldInfo{
		{Name: "name", FieldType: types.FieldTypeText, Nullable: true},
	})
	require.Error(t, err)
	assert.True(t, IsIOError(err))

	assertDatabaseCount(t, ds.db, "SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", 0, "cad_atomic")
	assertDatabaseCount(t, ds.db, "SELECT COUNT(*) FROM SmRegister WHERE SmDatasetName = ?", 0, "cad atomic")
	assertDatabaseCount(t, ds.db, "SELECT COUNT(*) FROM geometry_columns WHERE f_table_name = ?", 0, "cad_atomic")
	assertDatabaseCount(t, ds.db, "SELECT COUNT(*) FROM SmFieldInfo", 0)
}

func sqliteTableColumns(db *sql.DB, tableName string) ([]string, error) {
	rows, err := db.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var cid, notNull, primaryKey int
		var name, typeName string
		var defaultValue interface{}
		if err := rows.Scan(&cid, &name, &typeName, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func assertDatabaseCount(t *testing.T, db *sql.DB, query string, expected int, args ...interface{}) {
	t.Helper()

	var count int
	require.NoError(t, db.QueryRow(query, args...).Scan(&count))
	assert.Equal(t, expected, count)
}

func TestDataSource_CreateDuplicateDataset(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	ds, err := Create(udbxPath)
	require.NoError(t, err)
	defer ds.Close()

	_, err = ds.CreatePointDataset("cities", 4326, nil)
	require.NoError(t, err)

	// Try to create again with same name
	_, err = ds.CreatePointDataset("cities", 4326, nil)
	assert.Error(t, err)
	assert.True(t, IsConstraintViolation(err))
}

func TestDataSource_GetDataset(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	ds, err := Create(udbxPath)
	require.NoError(t, err)
	defer ds.Close()

	// Create datasets
	_, err = ds.CreateTabularDataset("countries", nil)
	require.NoError(t, err)

	_, err = ds.CreatePointDataset("cities", 4326, nil)
	require.NoError(t, err)

	_, err = ds.CreateLineDataset("roads", 4326, nil)
	require.NoError(t, err)

	_, err = ds.CreateRegionDataset("regions", 4326, nil)
	require.NoError(t, err)

	// Get each dataset
	tabular, err := ds.GetTabularDataset("countries")
	require.NoError(t, err)
	assert.NotNil(t, tabular)

	point, err := ds.GetPointDataset("cities")
	require.NoError(t, err)
	assert.NotNil(t, point)

	line, err := ds.GetLineDataset("roads")
	require.NoError(t, err)
	assert.NotNil(t, line)

	region, err := ds.GetRegionDataset("regions")
	require.NoError(t, err)
	assert.NotNil(t, region)
}

func TestDataSource_GetDataset_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	ds, err := Create(udbxPath)
	require.NoError(t, err)
	defer ds.Close()

	// Try to get non-existent dataset
	_, err = ds.GetDataset("nonexistent")
	assert.Error(t, err)
	assert.True(t, IsNotFound(err))
}

func TestDataSourceSpatialQueryPublicEntryPoints(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spatial.udbx")
	ds, err := Create(path)
	require.NoError(t, err)
	defer ds.Close()

	pointDataset, err := ds.CreatePointDataset("cities", 4326, []*types.FieldInfo{
		{Name: "name", FieldType: types.FieldTypeText, Nullable: true},
	})
	require.NoError(t, err)
	require.NoError(t, pointDataset.Insert(&types.Feature{
		ID: 7,
		Geometry: &types.PointGeometry{
			Type:        "Point",
			Coordinates: []float64{116.4, 39.9},
		},
		Attributes: map[string]interface{}{"name": "Beijing"},
	}))

	tableName := pointDataset.Info().TableName
	_, err = ds.db.Exec(
		"UPDATE SmRegister SET SmIDColName = ?, SmGeoColName = ? WHERE SmDatasetName = ?",
		"SmID", "SmGeometry", "cities",
	)
	require.NoError(t, err)
	_, err = ds.db.Exec(
		"UPDATE geometry_columns SET spatial_index_enabled = 1 WHERE f_table_name = ?",
		tableName,
	)
	require.NoError(t, err)
	quotedRTree, err := sqliteutil.QuoteIdentifier("idx_" + tableName + "_SmGeometry")
	require.NoError(t, err)
	_, err = ds.db.Exec(fmt.Sprintf(
		"CREATE VIRTUAL TABLE %s USING rtree(pkid, xmin, xmax, ymin, ymax)",
		quotedRTree,
	))
	require.NoError(t, err)
	_, err = ds.db.Exec(fmt.Sprintf(
		"INSERT INTO %s (pkid, xmin, xmax, ymin, ymax) VALUES (?, ?, ?, ?, ?)",
		quotedRTree,
	), 7, 116.4, 116.4, 39.9, 39.9)
	require.NoError(t, err)

	capability, err := ds.GetSpatialQueryCapability(context.Background(), "cities")
	require.NoError(t, err)
	assert.True(t, capability.RTreeAvailable)

	result, err := ds.QuerySpatial(context.Background(), "cities", types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 116, MinY: 39, MaxX: 117, MaxY: 40},
		Limit:  10,
	})
	require.NoError(t, err)
	require.Len(t, result.Features, 1)
	assert.Equal(t, 7, result.Features[0].ID)
	assert.Equal(t, "Beijing", result.Features[0].Attributes["name"])
}

func TestDataSourceSpatialQueryEntryPointsMapCanceledMetadataLookupToTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spatial.udbx")
	ds, err := Create(path)
	require.NoError(t, err)
	defer ds.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = ds.GetSpatialQueryCapability(ctx, "cities")
	assertPublicSpatialTimeout(t, err)
	_, err = ds.QuerySpatial(ctx, "cities", types.SpatialQueryOptions{
		Bounds: types.BoundingBox{},
		Limit:  1,
	})
	assertPublicSpatialTimeout(t, err)
}

func TestDataSourceCloseClearsEnvelopeCacheManager(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spatial-cache.udbx")
	ds, err := Create(path)
	require.NoError(t, err)

	points, err := ds.CreatePointDataset("cities", 4326, nil)
	require.NoError(t, err)
	require.NoError(t, points.Insert(&types.Feature{
		ID: 1,
		Geometry: &types.PointGeometry{
			Type:        "Point",
			Coordinates: []float64{116.4, 39.9},
		},
	}))

	result, err := ds.QuerySpatial(context.Background(), "cities", types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 116, MinY: 39, MaxX: 117, MaxY: 40},
		Limit:  10,
	})
	require.NoError(t, err)
	assert.Equal(t, types.SpatialQueryStrategyEnvelopeCache, result.Strategy)
	assert.Positive(t, ds.envelopeCacheManager.TotalBytes())
	assert.Equal(t, 1, ds.envelopeCacheManager.EntryCount())

	require.NoError(t, ds.Close())
	assert.Zero(t, ds.envelopeCacheManager.TotalBytes())
	assert.Zero(t, ds.envelopeCacheManager.ReservedBytes())
	assert.Zero(t, ds.envelopeCacheManager.EntryCount())
}

func TestDataSourceQuerySpatialMapsClosedEnvelopeCacheToLifecycleError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "spatial-cache-closed.udbx")
	ds, err := Create(path)
	require.NoError(t, err)
	defer ds.db.Close()

	_, err = ds.CreatePointDataset("cities", 4326, nil)
	require.NoError(t, err)
	ds.envelopeCacheManager.Close()

	_, err = ds.QuerySpatial(context.Background(), "cities", types.SpatialQueryOptions{
		Bounds: types.BoundingBox{},
		Limit:  10,
	})
	require.Error(t, err)
	assert.True(t, IsIOError(err))
	assert.True(t, IsUdbxError(err))
	_, hasReason := SpatialQueryReasonOf(err)
	assert.False(t, hasReason)
}

func TestDataSourceAttachesSpatialMutationHookOnCreate(t *testing.T) {
	tests := []struct {
		name   string
		create func(*testing.T, *DataSource) (internaldataset.Dataset, func() error)
	}{
		{
			name: "Text",
			create: func(t *testing.T, ds *DataSource) (internaldataset.Dataset, func() error) {
				text, err := ds.CreateTextDataset("labels", 4326, nil)
				require.NoError(t, err)
				require.NoError(t, text.Insert(dataSourceTextFeature(1, 1, 1)))
				return text, func() error { return text.Insert(dataSourceTextFeature(2, 2, 2)) }
			},
		},
		{
			name: "CAD",
			create: func(t *testing.T, ds *DataSource) (internaldataset.Dataset, func() error) {
				cad, err := ds.CreateCadDataset("cad", nil)
				require.NoError(t, err)
				require.NoError(t, cad.Insert(dataSourceCadFeature(1, 1, 1)))
				return cad, func() error { return cad.Insert(dataSourceCadFeature(2, 2, 2)) }
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, err := Create(filepath.Join(t.TempDir(), "mutation.udbx"))
			require.NoError(t, err)
			defer ds.Close()

			created, mutate := tt.create(t, ds)
			ds.attachSpatialMutationHook(created)
			ds.attachSpatialMutationHook(created)
			buildDataSourceEnvelopeCacheForSpatialDataset(t, ds, created.Info())
			require.Equal(t, 1, ds.envelopeCacheManager.EntryCount())
			require.Positive(t, ds.envelopeCacheManager.TotalBytes())

			require.NoError(t, mutate())
			assert.Zero(t, ds.envelopeCacheManager.EntryCount())
			assert.Zero(t, ds.envelopeCacheManager.TotalBytes())

			buildDataSourceEnvelopeCacheForSpatialDataset(t, ds, created.Info())
			assert.Equal(t, 1, ds.envelopeCacheManager.EntryCount())
			assert.Positive(t, ds.envelopeCacheManager.TotalBytes())
		})
	}
}

func TestDataSourceAttachesSpatialMutationHookOnGet(t *testing.T) {
	tests := []struct {
		name   string
		create func(*testing.T, *DataSource) *types.DatasetInfo
		get    func(*testing.T, *DataSource) func() error
	}{
		{
			name: "Text",
			create: func(t *testing.T, ds *DataSource) *types.DatasetInfo {
				text, err := ds.CreateTextDataset("labels", 4326, nil)
				require.NoError(t, err)
				require.NoError(t, text.Insert(dataSourceTextFeature(1, 1, 1)))
				return text.Info()
			},
			get: func(t *testing.T, ds *DataSource) func() error {
				text, err := ds.GetTextDataset("labels")
				require.NoError(t, err)
				return func() error { return text.Insert(dataSourceTextFeature(2, 2, 2)) }
			},
		},
		{
			name: "CAD",
			create: func(t *testing.T, ds *DataSource) *types.DatasetInfo {
				cad, err := ds.CreateCadDataset("cad", nil)
				require.NoError(t, err)
				require.NoError(t, cad.Insert(dataSourceCadFeature(1, 1, 1)))
				return cad.Info()
			},
			get: func(t *testing.T, ds *DataSource) func() error {
				cad, err := ds.GetCadDataset("cad")
				require.NoError(t, err)
				return func() error { return cad.Insert(dataSourceCadFeature(2, 2, 2)) }
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, err := Create(filepath.Join(t.TempDir(), "mutation.udbx"))
			require.NoError(t, err)
			defer ds.Close()

			info := tt.create(t, ds)
			buildDataSourceEnvelopeCacheForSpatialDataset(t, ds, info)
			require.Equal(t, 1, ds.envelopeCacheManager.EntryCount())
			require.Positive(t, ds.envelopeCacheManager.TotalBytes())
			_ = tt.get(t, ds)
			mutate := tt.get(t, ds)

			require.NoError(t, mutate())
			assert.Zero(t, ds.envelopeCacheManager.EntryCount())
			assert.Zero(t, ds.envelopeCacheManager.TotalBytes())

			buildDataSourceEnvelopeCacheForSpatialDataset(t, ds, info)
			assert.Equal(t, 1, ds.envelopeCacheManager.EntryCount())
			assert.Positive(t, ds.envelopeCacheManager.TotalBytes())
		})
	}
}

func TestDataSourceSpatialMutationHookSetterIsNotPromoted(t *testing.T) {
	ds, err := Create(filepath.Join(t.TempDir(), "mutation-method-set.udbx"))
	require.NoError(t, err)
	defer ds.Close()

	text, err := ds.CreateTextDataset("labels", 4326, nil)
	require.NoError(t, err)
	cad, err := ds.CreateCadDataset("cad", nil)
	require.NoError(t, err)

	for _, value := range []interface{}{text, cad} {
		_, exposed := reflect.TypeOf(value).MethodByName("SetSpatialMutationHook")
		assert.False(t, exposed)
	}
}

func TestDataSourceMutationAfterCloseReturnsErrorWithoutPanic(t *testing.T) {
	ds, err := Create(filepath.Join(t.TempDir(), "closed-mutation.udbx"))
	require.NoError(t, err)
	text, err := ds.CreateTextDataset("labels", 4326, nil)
	require.NoError(t, err)
	cad, err := ds.CreateCadDataset("cad", nil)
	require.NoError(t, err)
	require.NoError(t, ds.Close())

	assert.NotPanics(t, func() {
		assert.Error(t, text.Insert(dataSourceTextFeature(1, 1, 1)))
	})
	assert.NotPanics(t, func() {
		assert.Error(t, cad.Insert(dataSourceCadFeature(1, 1, 1)))
	})
}

func TestDataSourceCadSpatialQueryAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cad-spatial-reopen.udbx")
	ds, err := Create(path)
	require.NoError(t, err)

	cad, err := ds.CreateCadDataset("cad", []*types.FieldInfo{
		{Name: "name", FieldType: types.FieldTypeText, Nullable: true},
	})
	require.NoError(t, err)
	require.NoError(t, cad.InsertMany([]*types.Feature{
		{ID: 1, Geometry: &types.CadPointGeometry{XCoord: 1, YCoord: 1}, Attributes: map[string]interface{}{"name": "point"}},
		{ID: 2, Geometry: &types.CadLineGeometry{
			NumSub: 1, SubPointCounts: []int{2}, Coordinates: [][2]float64{{10, 10}, {12, 12}},
		}, Attributes: map[string]interface{}{"name": "line"}},
		{ID: 3, Geometry: &types.CadRegionGeometry{
			NumSub: 1, SubPointCounts: []int{5},
			Coordinates: [][2]float64{{20, 20}, {24, 20}, {24, 24}, {20, 24}, {20, 20}},
		}, Attributes: map[string]interface{}{"name": "region"}},
		{ID: 4, Geometry: &types.CadTextGeometry{
			Text: "label", Anchor: []float64{41, 41}, BBox: []float64{40, 40, 42, 42},
		}, Attributes: map[string]interface{}{"name": "text"}},
	}))

	require.NoError(t, ds.Close())

	reopened, err := Open(path)
	require.NoError(t, err)
	defer reopened.Close()
	result, err := reopened.QuerySpatial(context.Background(), "cad", types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 0, MinY: 0, MaxX: 50, MaxY: 50},
		Limit:  5,
	})
	require.NoError(t, err)
	assert.Equal(t, types.SpatialQueryStrategyEnvelopeCache, result.Strategy)
	assert.False(t, result.HasMore)
	require.Len(t, result.Features, 4)

	expectedTypes := map[int]interface{}{
		1: &types.CadPointGeometry{},
		2: &types.CadLineGeometry{},
		3: &types.CadRegionGeometry{},
		4: &types.CadTextGeometry{},
	}
	expectedBBoxes := map[int][]float64{
		1: {1, 1, 1, 1},
		2: {10, 10, 12, 12},
		3: {20, 20, 24, 24},
		4: {40, 40, 42, 42},
	}
	for _, feature := range result.Features {
		assert.IsType(t, expectedTypes[feature.ID], feature.Geometry)
		assert.Equal(t, expectedBBoxes[feature.ID], feature.Geometry.GetBBox())
		assert.Zero(t, feature.Geometry.GetSRID())
	}
}

func TestDataSourceTextAndCadSpatialQueryInvalidatesCacheAfterUpdate(t *testing.T) {
	tests := []struct {
		name   string
		create func(*testing.T, *DataSource) (string, func() error)
	}{
		{
			name: "Text",
			create: func(t *testing.T, ds *DataSource) (string, func() error) {
				text, err := ds.CreateTextDataset("labels", 4326, nil)
				require.NoError(t, err)
				require.NoError(t, text.Insert(dataSourceTextFeature(1, 0, 0)))
				return text.Info().Name, func() error {
					return text.Update(1, &FeatureChanges{Geometry: &types.TextGeometry{Text: "label", Anchor: []float64{100, 100}}})
				}
			},
		},
		{
			name: "CAD",
			create: func(t *testing.T, ds *DataSource) (string, func() error) {
				cad, err := ds.CreateCadDataset("cad", nil)
				require.NoError(t, err)
				require.NoError(t, cad.Insert(dataSourceCadFeature(1, 0, 0)))
				return cad.Info().Name, func() error {
					return cad.Update(1, &FeatureChanges{Geometry: &types.CadPointGeometry{XCoord: 100, YCoord: 100}})
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds, err := Create(filepath.Join(t.TempDir(), "spatial-cache-update.udbx"))
			require.NoError(t, err)
			defer ds.Close()

			datasetName, update := tt.create(t, ds)
			oldBounds := types.BoundingBox{MinX: -1, MinY: -1, MaxX: 1, MaxY: 1}
			newBounds := types.BoundingBox{MinX: 99, MinY: 99, MaxX: 101, MaxY: 101}
			oldResult, err := ds.QuerySpatial(context.Background(), datasetName, types.SpatialQueryOptions{Bounds: oldBounds, Limit: 10})
			require.NoError(t, err)
			require.Len(t, oldResult.Features, 1)
			require.Equal(t, 1, oldResult.Features[0].ID)
			require.Equal(t, types.SpatialQueryStrategyEnvelopeCache, oldResult.Strategy)

			require.NoError(t, update())
			oldResult, err = ds.QuerySpatial(context.Background(), datasetName, types.SpatialQueryOptions{Bounds: oldBounds, Limit: 10})
			require.NoError(t, err)
			assert.Empty(t, oldResult.Features)
			newResult, err := ds.QuerySpatial(context.Background(), datasetName, types.SpatialQueryOptions{Bounds: newBounds, Limit: 10})
			require.NoError(t, err)
			require.Len(t, newResult.Features, 1)
			assert.Equal(t, 1, newResult.Features[0].ID)
		})
	}
}

func buildDataSourceEnvelopeCacheForSpatialDataset(t *testing.T, ds *DataSource, info *types.DatasetInfo) {
	t.Helper()
	_, err := ds.db.Exec(
		`UPDATE SmRegister
		 SET SmDatasetType = ?, SmIDColName = ?, SmGeoColName = ?
		 WHERE SmDatasetName = ?`,
		int(types.DatasetKindPoint), "SmID", "SmIndexKey", info.Name,
	)
	require.NoError(t, err)

	result, err := ds.QuerySpatial(context.Background(), info.Name, types.SpatialQueryOptions{
		Bounds: types.BoundingBox{MinX: 1000, MinY: 1000, MaxX: 1001, MaxY: 1001},
		Limit:  10,
	})
	require.NoError(t, err)
	require.Empty(t, result.Features)
	require.Equal(t, types.SpatialQueryStrategyEnvelopeCache, result.Strategy)

	_, err = ds.db.Exec(
		`UPDATE SmRegister SET SmDatasetType = ?, SmGeoColName = ? WHERE SmDatasetName = ?`,
		int(info.Kind), "SmGeometry", info.Name,
	)
	require.NoError(t, err)
}

func dataSourceTextFeature(id int, x, y float64) *types.Feature {
	return &types.Feature{
		ID:       id,
		Geometry: &types.TextGeometry{Text: "label", Anchor: []float64{x, y}},
	}
}

func dataSourceCadFeature(id int, x, y float64) *types.Feature {
	return &types.Feature{
		ID:       id,
		Geometry: &types.CadPointGeometry{XCoord: x, YCoord: y},
	}
}

func assertPublicSpatialTimeout(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	reason, ok := SpatialQueryReasonOf(err)
	require.True(t, ok)
	assert.Equal(t, SpatialQueryReasonQueryTimeout, reason)
	assert.True(t, stderrors.Is(err, context.Canceled))
}
