package udbx4go

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/dataset"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestUdbx4SpecComplianceDatabaseRead(t *testing.T) {
	assertComplianceDatabaseReadable(t, udbx4SpecFixturePath(t, "compliance", "compliance.udbx"))
}

func TestUdbx4SpecUdbx4TsRoundtripDatabaseRead(t *testing.T) {
	assertComplianceDatabaseReadable(t, udbx4SpecFixturePath(t, "compliance", "roundtrip", "udbx4ts-roundtrip.udbx"))
}

func TestUdbx4SpecUdbx4GoRoundtripDatabaseRead(t *testing.T) {
	assertComplianceDatabaseReadable(t, udbx4SpecFixturePath(t, "compliance", "roundtrip", "udbx4go-roundtrip.udbx"))
}

func TestUdbx4SpecUdbx4JRoundtripDatabaseRead(t *testing.T) {
	assertComplianceDatabaseReadable(t, udbx4SpecFixturePath(t, "compliance", "roundtrip", "udbx4j-roundtrip.udbx"))
}

func TestUdbx4SpecTextAndCadSpatialQueryContract(t *testing.T) {
	source, err := Open(udbx4SpecFixturePath(t, "compliance", "compliance.udbx"))
	require.NoError(t, err)
	defer source.Close()

	datasetNames := []string{"test_text", "test_cad"}
	snapshots := make(map[string]*datasetSnapshot, len(datasetNames))
	targetPath := filepath.Join(t.TempDir(), "text-cad-spatial-contract.udbx")
	target, err := Create(targetPath)
	require.NoError(t, err)
	for _, datasetName := range datasetNames {
		snapshots[datasetName] = snapshotDataset(t, source, datasetName)
		copySnapshotToDataSource(t, target, snapshots[datasetName])
	}
	require.NoError(t, target.Close())

	reopened, err := Open(targetPath)
	require.NoError(t, err)
	defer reopened.Close()

	for _, datasetName := range datasetNames {
		t.Run(datasetName, func(t *testing.T) {
			expected := snapshots[datasetName].records.([]*types.Feature)
			result, err := reopened.QuerySpatial(context.Background(), datasetName, SpatialQueryOptions{
				Bounds: complianceFeatureBounds(t, expected),
				Limit:  len(expected) + 1,
			})
			require.NoError(t, err)
			assert.Equal(t, SpatialQueryStrategyEnvelopeCache, result.Strategy)
			assert.False(t, result.HasMore)
			require.Len(t, result.Features, len(expected))
			for index := range expected {
				assert.Equal(t, expected[index].ID, result.Features[index].ID)
				assert.Equal(t, expected[index].Geometry.GeometryType(), result.Features[index].Geometry.GeometryType())
			}
		})
	}
}

func complianceFeatureBounds(t *testing.T, features []*types.Feature) BoundingBox {
	t.Helper()
	require.NotEmpty(t, features)

	first := featureBoundingBox(t, features[0])
	bounds := first
	for _, feature := range features[1:] {
		bbox := featureBoundingBox(t, feature)
		bounds.MinX = min(bounds.MinX, bbox.MinX)
		bounds.MinY = min(bounds.MinY, bbox.MinY)
		bounds.MaxX = max(bounds.MaxX, bbox.MaxX)
		bounds.MaxY = max(bounds.MaxY, bbox.MaxY)
	}
	return bounds
}

func udbx4SpecFixturePath(t testing.TB, elements ...string) string {
	t.Helper()
	for _, root := range []string{
		filepath.Join("..", "udbx4spec"),
		filepath.Join("..", "..", "..", "udbx4spec"),
	} {
		path := filepath.Join(append([]string{root}, elements...)...)
		if available, _ := externalFixtureAvailable(path, false); available {
			return path
		}
	}
	return requireExternalFixturePath(t, filepath.Join(append([]string{"..", "udbx4spec"}, elements...)...))
}

func assertComplianceDatabaseReadable(t *testing.T, path string) {
	t.Helper()

	ds, err := Open(path)
	require.NoError(t, err)
	defer ds.Close()

	datasets, err := ds.ListDatasets()
	require.NoError(t, err)

	datasetByName := make(map[string]*types.DatasetInfo)
	for _, info := range datasets {
		datasetByName[info.Name] = info
	}

	assertDatasetInfo(t, datasetByName, "test_points", types.DatasetKindPoint, 3)
	assertDatasetInfo(t, datasetByName, "test_lines", types.DatasetKindLine, 2)
	assertDatasetInfo(t, datasetByName, "test_regions", types.DatasetKindRegion, 1)
	assertDatasetInfo(t, datasetByName, "test_points_z", types.DatasetKindPointZ, 2)
	assertDatasetInfo(t, datasetByName, "test_lines_z", types.DatasetKindLineZ, 1)
	assertDatasetInfo(t, datasetByName, "test_regions_z", types.DatasetKindRegionZ, 1)
	assertDatasetInfo(t, datasetByName, "test_tabular", types.DatasetKindTabular, 2)
	assertDatasetInfo(t, datasetByName, "test_cad", types.DatasetKindCAD, 3)
	if _, ok := datasetByName["test_text"]; ok {
		assertDatasetInfo(t, datasetByName, "test_text", types.DatasetKindText, 1)
	}

	pointDataset, err := ds.GetPointDataset("test_points")
	require.NoError(t, err)
	assertFieldTypes(t, pointDataset, map[string]types.FieldType{
		"NAME":      types.FieldTypeText,
		"CATEGORY":  types.FieldTypeText,
		"ELEVATION": types.FieldTypeDouble,
	})

	pointFeature, err := pointDataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, 1, pointFeature.ID)
	assert.Equal(t, "Alpha City", pointFeature.Attributes["NAME"])
	assert.Equal(t, "capital", pointFeature.Attributes["CATEGORY"])
	assert.InDelta(t, 39.5, numericValue(t, pointFeature.Attributes["ELEVATION"]), 0.000000001)
	pointGeometry, ok := pointFeature.Geometry.(*types.PointGeometry)
	require.True(t, ok)
	assert.InDeltaSlice(t, []float64{116.123, 39.456}, pointGeometry.Coordinates, 0.000000001)

	lineDataset, err := ds.GetLineDataset("test_lines")
	require.NoError(t, err)
	assertFieldTypes(t, lineDataset, map[string]types.FieldType{
		"NAME":      types.FieldTypeText,
		"LEVEL":     types.FieldTypeInt32,
		"LENGTH_KM": types.FieldTypeDouble,
	})

	lineFeature, err := lineDataset.GetByID(10)
	require.NoError(t, err)
	assert.Equal(t, "North Corridor", lineFeature.Attributes["NAME"])
	assert.Equal(t, int64(1), lineFeature.Attributes["LEVEL"])
	assert.InDelta(t, 128.4, numericValue(t, lineFeature.Attributes["LENGTH_KM"]), 0.000000001)
	lineGeometry, ok := lineFeature.Geometry.(*types.MultiLineStringGeometry)
	require.True(t, ok)
	require.Len(t, lineGeometry.Coordinates, 1)
	require.Len(t, lineGeometry.Coordinates[0], 3)
	assert.InDeltaSlice(t, []float64{116.123, 39.456}, lineGeometry.Coordinates[0][0], 0.000000001)

	regionDataset, err := ds.GetRegionDataset("test_regions")
	require.NoError(t, err)
	assertFieldTypes(t, regionDataset, map[string]types.FieldType{
		"NAME":     types.FieldTypeText,
		"LEVEL":    types.FieldTypeInt32,
		"AREA_KM2": types.FieldTypeDouble,
	})

	regionFeature, err := regionDataset.GetByID(20)
	require.NoError(t, err)
	assert.Equal(t, "Core Region", regionFeature.Attributes["NAME"])
	assert.Equal(t, int64(3), regionFeature.Attributes["LEVEL"])
	assert.InDelta(t, 845.6, numericValue(t, regionFeature.Attributes["AREA_KM2"]), 0.000000001)
	regionGeometry, ok := regionFeature.Geometry.(*types.MultiPolygonGeometry)
	require.True(t, ok)
	require.Len(t, regionGeometry.Coordinates, 1)
	require.Len(t, regionGeometry.Coordinates[0], 2, "Core Region 应包含外环和一个内环")

	pointZDatasetGeneric, err := ds.GetDataset("test_points_z")
	require.NoError(t, err)
	pointZDataset, ok := pointZDatasetGeneric.(*dataset.PointZDataset)
	require.True(t, ok)
	assertFieldTypes(t, pointZDataset, map[string]types.FieldType{
		"NAME":      types.FieldTypeText,
		"CATEGORY":  types.FieldTypeText,
		"ELEVATION": types.FieldTypeDouble,
	})

	pointZFeature, err := pointZDataset.GetByID(101)
	require.NoError(t, err)
	assert.Equal(t, 101, pointZFeature.ID)
	assert.Equal(t, "Alpha Tower", pointZFeature.Attributes["NAME"])
	assert.Equal(t, "control", pointZFeature.Attributes["CATEGORY"])
	assert.InDelta(t, 88.8, numericValue(t, pointZFeature.Attributes["ELEVATION"]), 0.000000001)
	pointZGeometry, ok := pointZFeature.Geometry.(*types.PointGeometry)
	require.True(t, ok)
	assert.True(t, pointZGeometry.HasZ())
	assert.InDeltaSlice(t, []float64{116.123, 39.456, 12.5}, pointZGeometry.Coordinates, 0.000000001)

	lineZDatasetGeneric, err := ds.GetDataset("test_lines_z")
	require.NoError(t, err)
	lineZDataset, ok := lineZDatasetGeneric.(*dataset.LineZDataset)
	require.True(t, ok)
	assertFieldTypes(t, lineZDataset, map[string]types.FieldType{
		"NAME":      types.FieldTypeText,
		"LEVEL":     types.FieldTypeInt32,
		"LENGTH_KM": types.FieldTypeDouble,
	})

	lineZFeature, err := lineZDataset.GetByID(110)
	require.NoError(t, err)
	assert.Equal(t, "Sky Corridor", lineZFeature.Attributes["NAME"])
	assert.Equal(t, int64(5), lineZFeature.Attributes["LEVEL"])
	assert.InDelta(t, 128.4, numericValue(t, lineZFeature.Attributes["LENGTH_KM"]), 0.000000001)
	lineZGeometry, ok := lineZFeature.Geometry.(*types.MultiLineStringGeometry)
	require.True(t, ok)
	assert.True(t, lineZGeometry.HasZ())
	require.Len(t, lineZGeometry.Coordinates, 1)
	require.Len(t, lineZGeometry.Coordinates[0], 3)
	assert.InDeltaSlice(t, []float64{116.123, 39.456, 12.5}, lineZGeometry.Coordinates[0][0], 0.000000001)

	regionZDatasetGeneric, err := ds.GetDataset("test_regions_z")
	require.NoError(t, err)
	regionZDataset, ok := regionZDatasetGeneric.(*dataset.RegionZDataset)
	require.True(t, ok)
	assertFieldTypes(t, regionZDataset, map[string]types.FieldType{
		"NAME":     types.FieldTypeText,
		"LEVEL":    types.FieldTypeInt32,
		"AREA_KM2": types.FieldTypeDouble,
	})

	regionZFeature, err := regionZDataset.GetByID(120)
	require.NoError(t, err)
	assert.Equal(t, "Elevated Region", regionZFeature.Attributes["NAME"])
	assert.Equal(t, int64(7), regionZFeature.Attributes["LEVEL"])
	assert.InDelta(t, 845.6, numericValue(t, regionZFeature.Attributes["AREA_KM2"]), 0.000000001)
	regionZGeometry, ok := regionZFeature.Geometry.(*types.MultiPolygonGeometry)
	require.True(t, ok)
	assert.True(t, regionZGeometry.HasZ())
	require.Len(t, regionZGeometry.Coordinates, 1)
	require.Len(t, regionZGeometry.Coordinates[0], 1)
	assert.InDeltaSlice(t, []float64{116, 39.2, 10}, regionZGeometry.Coordinates[0][0][0], 0.000000001)

	tabularDataset, err := ds.GetTabularDataset("test_tabular")
	require.NoError(t, err)
	assertFieldTypes(t, tabularDataset, map[string]types.FieldType{
		"NAME":  types.FieldTypeText,
		"VALUE": types.FieldTypeInt32,
		"SCORE": types.FieldTypeDouble,
	})

	tabularRecord, err := tabularDataset.GetByID(30)
	require.NoError(t, err)
	assert.Equal(t, "config.maxZoom", tabularRecord.Attributes["NAME"])
	assert.Equal(t, int64(18), tabularRecord.Attributes["VALUE"])
	assert.InDelta(t, 0.95, numericValue(t, tabularRecord.Attributes["SCORE"]), 0.000000001)

	cadDataset, err := ds.GetCadDataset("test_cad")
	require.NoError(t, err)
	assertFieldTypes(t, cadDataset, map[string]types.FieldType{
		"NAME":  types.FieldTypeText,
		"LEVEL": types.FieldTypeInt32,
	})

	cadPointFeature, err := cadDataset.GetByID(130)
	require.NoError(t, err)
	assert.Equal(t, "CAD Point", cadPointFeature.Attributes["NAME"])
	assert.Equal(t, int64(1), cadPointFeature.Attributes["LEVEL"])
	cadPoint, ok := cadPointFeature.Geometry.(*types.CadPointGeometry)
	require.True(t, ok)
	assert.InDelta(t, 116.123, cadPoint.XCoord, 0.000000001)
	assert.InDelta(t, 39.456, cadPoint.YCoord, 0.000000001)

	cadLineFeature, err := cadDataset.GetByID(131)
	require.NoError(t, err)
	cadLine, ok := cadLineFeature.Geometry.(*types.CadLineGeometry)
	require.True(t, ok)
	assert.Equal(t, 1, cadLine.NumSub)
	assert.Equal(t, []int{3}, cadLine.SubPointCounts)

	cadRegionFeature, err := cadDataset.GetByID(132)
	require.NoError(t, err)
	cadRegion, ok := cadRegionFeature.Geometry.(*types.CadRegionGeometry)
	require.True(t, ok)
	assert.Equal(t, 1, cadRegion.NumSub)
	assert.Equal(t, []int{5}, cadRegion.SubPointCounts)

	if _, ok := datasetByName["test_text"]; ok {
		textDataset, err := ds.GetTextDataset("test_text")
		require.NoError(t, err)
		assertFieldTypes(t, textDataset, map[string]types.FieldType{
			"NAME":  types.FieldTypeText,
			"LEVEL": types.FieldTypeInt32,
		})

		textFeature, err := textDataset.GetByID(1)
		require.NoError(t, err)
		assert.Equal(t, "Text Label", textFeature.Attributes["NAME"])
		assert.Equal(t, int64(1), textFeature.Attributes["LEVEL"])
		textGeometry, ok := textFeature.Geometry.(*types.TextGeometry)
		require.True(t, ok)
		assert.Equal(t, "Text", textGeometry.GeometryType())
		assert.Equal(t, "河南省", textGeometry.Text)
		assert.InDeltaSlice(t, []float64{113.165187569688, 33.875453985}, textGeometry.Anchor, 0.000000001)
		assert.InDelta(t, 0, textGeometry.Rotation, 0.000000001)
		require.NotNil(t, textGeometry.Style)
		assert.Equal(t, "宋体", textGeometry.Style.FaceName)
		assert.InDelta(t, 0.406494140625, textGeometry.Style.FontHeight, 0.000000001)
		require.NotNil(t, textGeometry.Style.Color)
		assert.Equal(t, &types.Color{A: 0, B: 0, G: 0, R: 255}, textGeometry.Style.Color)
		require.Len(t, textGeometry.SubTexts, 1)
		assert.Equal(t, "河南省", textGeometry.SubTexts[0].Text)
	}
}

func TestUdbx4SpecComplianceDatabaseSemanticRoundtrip(t *testing.T) {
	source, err := Open(udbx4SpecFixturePath(t, "compliance", "compliance.udbx"))
	require.NoError(t, err)
	defer source.Close()

	datasetNames := []string{
		"test_points",
		"test_lines",
		"test_regions",
		"test_points_z",
		"test_lines_z",
		"test_regions_z",
		"test_tabular",
		"test_cad",
		"test_text",
	}
	sourceSnapshots := make(map[string]*datasetSnapshot)
	for _, name := range datasetNames {
		sourceSnapshots[name] = snapshotDataset(t, source, name)
	}

	roundtripPath := filepath.Join(t.TempDir(), "roundtrip.udbx")
	target, err := Create(roundtripPath)
	require.NoError(t, err)

	for _, name := range datasetNames {
		copySnapshotToDataSource(t, target, sourceSnapshots[name])
	}
	require.NoError(t, target.Close())

	reopened, err := Open(roundtripPath)
	require.NoError(t, err)
	defer reopened.Close()

	for _, name := range datasetNames {
		assertSnapshotEquivalent(t, sourceSnapshots[name], snapshotDataset(t, reopened, name))
	}
}

type datasetSnapshot struct {
	info    *types.DatasetInfo
	fields  []*types.FieldInfo
	records interface{}
}

func snapshotDataset(t *testing.T, ds *DataSource, name string) *datasetSnapshot {
	t.Helper()

	genericDataset, err := ds.GetDataset(name)
	require.NoError(t, err)

	fields, err := genericDataset.GetFields()
	require.NoError(t, err)

	switch typedDataset := genericDataset.(type) {
	case *dataset.PointZDataset:
		features, err := typedDataset.List(nil)
		require.NoError(t, err)
		return &datasetSnapshot{info: typedDataset.Info(), fields: fields, records: features}
	case *dataset.LineZDataset:
		features, err := typedDataset.List(nil)
		require.NoError(t, err)
		return &datasetSnapshot{info: typedDataset.Info(), fields: fields, records: features}
	case *dataset.RegionZDataset:
		features, err := typedDataset.List(nil)
		require.NoError(t, err)
		return &datasetSnapshot{info: typedDataset.Info(), fields: fields, records: features}
	case *dataset.PointDataset:
		features, err := typedDataset.List(nil)
		require.NoError(t, err)
		return &datasetSnapshot{info: typedDataset.Info(), fields: fields, records: features}
	case *dataset.LineDataset:
		features, err := typedDataset.List(nil)
		require.NoError(t, err)
		return &datasetSnapshot{info: typedDataset.Info(), fields: fields, records: features}
	case *dataset.RegionDataset:
		features, err := typedDataset.List(nil)
		require.NoError(t, err)
		return &datasetSnapshot{info: typedDataset.Info(), fields: fields, records: features}
	case *dataset.TabularDataset:
		records, err := typedDataset.List(nil)
		require.NoError(t, err)
		return &datasetSnapshot{info: typedDataset.Info(), fields: fields, records: records}
	case *dataset.TextDataset:
		features, err := typedDataset.List(nil)
		require.NoError(t, err)
		return &datasetSnapshot{info: typedDataset.Info(), fields: fields, records: features}
	case *dataset.CadDataset:
		features, err := typedDataset.List(nil)
		require.NoError(t, err)
		return &datasetSnapshot{info: typedDataset.Info(), fields: fields, records: features}
	default:
		require.Failf(t, "不支持的数据集类型", "%s: %T", name, genericDataset)
		return nil
	}
}

func copySnapshotToDataSource(t *testing.T, ds *DataSource, snapshot *datasetSnapshot) {
	t.Helper()

	srid := 0
	if snapshot.info.SRID != nil {
		srid = *snapshot.info.SRID
	}

	switch snapshot.info.Kind {
	case types.DatasetKindPoint:
		targetDataset, err := ds.CreatePointDataset(snapshot.info.Name, srid, snapshot.fields)
		require.NoError(t, err)
		require.NoError(t, targetDataset.InsertMany(snapshot.records.([]*types.Feature)))
	case types.DatasetKindPointZ:
		targetDataset, err := ds.CreatePointZDataset(snapshot.info.Name, srid, snapshot.fields)
		require.NoError(t, err)
		require.NoError(t, targetDataset.InsertMany(snapshot.records.([]*types.Feature)))
	case types.DatasetKindLine:
		targetDataset, err := ds.CreateLineDataset(snapshot.info.Name, srid, snapshot.fields)
		require.NoError(t, err)
		require.NoError(t, targetDataset.InsertMany(snapshot.records.([]*types.Feature)))
	case types.DatasetKindLineZ:
		targetDataset, err := ds.CreateLineZDataset(snapshot.info.Name, srid, snapshot.fields)
		require.NoError(t, err)
		require.NoError(t, targetDataset.InsertMany(snapshot.records.([]*types.Feature)))
	case types.DatasetKindRegion:
		targetDataset, err := ds.CreateRegionDataset(snapshot.info.Name, srid, snapshot.fields)
		require.NoError(t, err)
		require.NoError(t, targetDataset.InsertMany(snapshot.records.([]*types.Feature)))
	case types.DatasetKindRegionZ:
		targetDataset, err := ds.CreateRegionZDataset(snapshot.info.Name, srid, snapshot.fields)
		require.NoError(t, err)
		require.NoError(t, targetDataset.InsertMany(snapshot.records.([]*types.Feature)))
	case types.DatasetKindTabular:
		targetDataset, err := ds.CreateTabularDataset(snapshot.info.Name, snapshot.fields)
		require.NoError(t, err)
		require.NoError(t, targetDataset.InsertMany(snapshot.records.([]*types.TabularRecord)))
	case types.DatasetKindText:
		targetDataset, err := ds.CreateTextDataset(snapshot.info.Name, srid, snapshot.fields)
		require.NoError(t, err)
		require.NoError(t, targetDataset.InsertMany(snapshot.records.([]*types.Feature)))
	case types.DatasetKindCAD:
		targetDataset, err := ds.CreateCadDataset(snapshot.info.Name, snapshot.fields)
		require.NoError(t, err)
		require.NoError(t, targetDataset.InsertMany(snapshot.records.([]*types.Feature)))
	default:
		require.Failf(t, "不支持的数据集类型", "%s: %s", snapshot.info.Name, snapshot.info.Kind.String())
	}
}

func assertSnapshotEquivalent(t *testing.T, expected, actual *datasetSnapshot) {
	t.Helper()

	assert.Equal(t, expected.info.Name, actual.info.Name)
	assert.Equal(t, expected.info.Kind, actual.info.Kind)
	assert.Equal(t, expected.info.ObjectCount, actual.info.ObjectCount)
	assert.Equal(t, normalizeSRID(expected.info.SRID), normalizeSRID(actual.info.SRID))
	assert.Equal(t, normalizeFieldsForCompare(expected.fields), normalizeFieldsForCompare(actual.fields))
	assert.Equal(t, normalizeRecordsForCompare(expected.records), normalizeRecordsForCompare(actual.records),
		"dataset %s records differ after roundtrip", expected.info.Name)
}

func normalizeSRID(srid *int) int {
	if srid == nil {
		return 0
	}
	return *srid
}

func normalizeFieldsForCompare(fields []*types.FieldInfo) []map[string]interface{} {
	normalized := make([]map[string]interface{}, len(fields))
	for i, field := range fields {
		normalized[i] = map[string]interface{}{
			"name":      field.Name,
			"fieldType": field.FieldType,
			"nullable":  field.Nullable,
			"required":  field.Required,
		}
	}
	return normalized
}

func normalizeRecordsForCompare(records interface{}) interface{} {
	switch typedRecords := records.(type) {
	case []*types.Feature:
		normalized := make([]map[string]interface{}, len(typedRecords))
		for i, feature := range typedRecords {
			normalized[i] = map[string]interface{}{
				"id":         feature.ID,
				"geometry":   normalizeGeometryForCompare(feature.Geometry),
				"attributes": normalizedAttributes(feature.Attributes),
			}
		}
		return normalized
	case []*types.TabularRecord:
		normalized := make([]map[string]interface{}, len(typedRecords))
		for i, record := range typedRecords {
			normalized[i] = map[string]interface{}{
				"id":         record.ID,
				"attributes": normalizedAttributes(record.Attributes),
			}
		}
		return normalized
	default:
		return records
	}
}

func normalizeGeometryForCompare(geometry types.Geometry) types.Geometry {
	switch typed := geometry.(type) {
	case *types.TextGeometry:
		clone := *typed
		clone.BBox = nil
		return &clone
	case *types.CadPointGeometry:
		clone := *typed
		clone.BBox = nil
		return &clone
	case *types.CadLineGeometry:
		clone := *typed
		clone.BBox = nil
		return &clone
	case *types.CadRegionGeometry:
		clone := *typed
		clone.BBox = nil
		return &clone
	case *types.CadTextGeometry:
		clone := *typed
		clone.BBox = nil
		return &clone
	default:
		return geometry
	}
}

func normalizedAttributes(attributes map[string]interface{}) map[string]interface{} {
	normalized := make(map[string]interface{})
	for key, value := range attributes {
		if key == "SmUserID" {
			continue
		}
		normalized[key] = value
	}
	return normalized
}

type fieldProvider interface {
	GetFields() ([]*types.FieldInfo, error)
}

func assertFieldTypes(t *testing.T, provider fieldProvider, expected map[string]types.FieldType) {
	t.Helper()

	fields, err := provider.GetFields()
	require.NoError(t, err)

	actual := make(map[string]types.FieldType)
	for _, field := range fields {
		actual[field.Name] = field.FieldType
	}

	assert.Equal(t, expected, actual)
}

func assertDatasetInfo(
	t *testing.T,
	datasetByName map[string]*types.DatasetInfo,
	name string,
	kind types.DatasetKind,
	objectCount int,
) {
	t.Helper()

	info, ok := datasetByName[name]
	require.True(t, ok, "缺少数据集 %s", name)
	assert.Equal(t, kind, info.Kind)
	assert.Equal(t, objectCount, info.ObjectCount)
}

func numericValue(t *testing.T, value interface{}) float64 {
	t.Helper()

	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int64:
		return float64(v)
	case int:
		return float64(v)
	default:
		require.Failf(t, "非数值属性", "属性类型为 %T", value)
		return 0
	}
}
