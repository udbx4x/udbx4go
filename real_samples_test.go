package udbx4go

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/dataset"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func requireExternalFixturePath(t *testing.T, path string) string {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("external fixture not available: %s", path)
		}
		require.NoError(t, err)
	}
	return path
}

func sampleDataFixturePath(t *testing.T) string {
	t.Helper()
	return requireExternalFixturePath(t, filepath.Join("..", "data", "SampleData.udbx"))
}

func henanFixturePath(t *testing.T) string {
	t.Helper()
	return requireExternalFixturePath(t, filepath.Join("..", "data", "henan.udbx"))
}

func TestRealSampleDataPointDataset(t *testing.T) {
	ds, err := Open(sampleDataFixturePath(t))
	require.NoError(t, err)
	defer ds.Close()

	pointDataset, err := ds.GetPointDataset("BaseMap_P")
	require.NoError(t, err)
	assert.Equal(t, "BaseMap_P", pointDataset.Info().Name)
	assert.Equal(t, types.DatasetKindPoint, pointDataset.Info().Kind)
	count, err := pointDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 15, count)

	feature, err := pointDataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, 1, feature.ID)
	assert.Equal(t, "蓟县", feature.Attributes["NAME"])
	assert.Equal(t, int64(120225), feature.Attributes["CODE"])
	assert.Equal(t, int64(4), feature.Attributes["ADCLASS"])

	geometry, ok := feature.Geometry.(*types.PointGeometry)
	require.True(t, ok)
	assert.Equal(t, "Point", geometry.GeometryType())
	assert.Len(t, geometry.Coordinates, 2)
}

func TestRealSampleDataCadDataset(t *testing.T) {
	ds, err := Open(sampleDataFixturePath(t))
	require.NoError(t, err)
	defer ds.Close()

	cadDataset, err := ds.GetCadDataset("CADDT")
	require.NoError(t, err)
	assert.Equal(t, "CADDT", cadDataset.Info().Name)
	assert.Equal(t, types.DatasetKindCAD, cadDataset.Info().Kind)
	count, err := cadDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 92, count)

	features, err := cadDataset.List(&types.QueryOptions{Limit: 100})
	require.NoError(t, err)
	require.Len(t, features, 92)
	require.NotNil(t, features[0].Geometry)
	assert.Contains(t, []string{"CadPoint", "CadLine", "CadRegion"}, features[0].Geometry.GeometryType())

	geometryTypes := map[string]bool{}
	for _, feature := range features {
		require.NotNil(t, feature.Geometry)
		geometryTypes[feature.Geometry.GeometryType()] = true
	}
	assert.True(t, geometryTypes["CadPoint"])
	assert.True(t, geometryTypes["CadLine"])
	assert.True(t, geometryTypes["CadRegion"])
	assert.True(t, geometryTypes["CadText"])
}

func TestRealSampleDataLineAndRegionDatasets(t *testing.T) {
	ds, err := Open(sampleDataFixturePath(t))
	require.NoError(t, err)
	defer ds.Close()

	lineDataset, err := ds.GetLineDataset("BaseMap_L")
	require.NoError(t, err)
	assert.Equal(t, "BaseMap_L", lineDataset.Info().Name)
	assert.Equal(t, types.DatasetKindLine, lineDataset.Info().Kind)
	assertSRID(t, lineDataset.Info(), 4326)
	lineCount, err := lineDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 47, lineCount)

	lineFeatures, err := lineDataset.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	require.Len(t, lineFeatures, 1)
	lineGeometry, ok := lineFeatures[0].Geometry.(*types.MultiLineStringGeometry)
	require.True(t, ok)
	assert.Equal(t, "MultiLineString", lineGeometry.GeometryType())
	assert.False(t, lineGeometry.HasZ())

	regionDataset, err := ds.GetRegionDataset("BaseMap_R")
	require.NoError(t, err)
	assert.Equal(t, "BaseMap_R", regionDataset.Info().Name)
	assert.Equal(t, types.DatasetKindRegion, regionDataset.Info().Kind)
	assertSRID(t, regionDataset.Info(), 4326)
	regionCount, err := regionDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 15, regionCount)

	regionFeatures, err := regionDataset.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	require.Len(t, regionFeatures, 1)
	regionGeometry, ok := regionFeatures[0].Geometry.(*types.MultiPolygonGeometry)
	require.True(t, ok)
	assert.Equal(t, "MultiPolygon", regionGeometry.GeometryType())
	assert.False(t, regionGeometry.HasZ())
}

func TestRealSampleDataVector3DDatasets(t *testing.T) {
	ds, err := Open(sampleDataFixturePath(t))
	require.NoError(t, err)
	defer ds.Close()

	pointZGeneric, err := ds.GetDataset("BaseMap_PZ")
	require.NoError(t, err)
	pointZDataset, ok := pointZGeneric.(*dataset.PointZDataset)
	require.True(t, ok)
	assert.Equal(t, types.DatasetKindPointZ, pointZDataset.Info().Kind)
	assertSRID(t, pointZDataset.Info(), 0)
	pointZCount, err := pointZDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 15, pointZCount)

	pointFeature, err := pointZDataset.GetByID(1)
	require.NoError(t, err)
	pointGeometry, ok := pointFeature.Geometry.(*types.PointGeometry)
	require.True(t, ok)
	assert.True(t, pointGeometry.HasZ())
	assert.Len(t, pointGeometry.Coordinates, 3)

	lineZGeneric, err := ds.GetDataset("BaseMap_LZ")
	require.NoError(t, err)
	lineZDataset, ok := lineZGeneric.(*dataset.LineZDataset)
	require.True(t, ok)
	assert.Equal(t, types.DatasetKindLineZ, lineZDataset.Info().Kind)
	assertSRID(t, lineZDataset.Info(), 0)
	lineZCount, err := lineZDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 47, lineZCount)

	lineFeatures, err := lineZDataset.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	require.Len(t, lineFeatures, 1)
	lineGeometry, ok := lineFeatures[0].Geometry.(*types.MultiLineStringGeometry)
	require.True(t, ok)
	assert.True(t, lineGeometry.HasZ())

	regionZGeneric, err := ds.GetDataset("BaseMap_RZ")
	require.NoError(t, err)
	regionZDataset, ok := regionZGeneric.(*dataset.RegionZDataset)
	require.True(t, ok)
	assert.Equal(t, types.DatasetKindRegionZ, regionZDataset.Info().Kind)
	assertSRID(t, regionZDataset.Info(), 0)
	regionZCount, err := regionZDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 15, regionZCount)

	regionFeatures, err := regionZDataset.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	require.Len(t, regionFeatures, 1)
	regionGeometry, ok := regionFeatures[0].Geometry.(*types.MultiPolygonGeometry)
	require.True(t, ok)
	assert.True(t, regionGeometry.HasZ())
}

func TestRealSampleDataCountyTextDataset(t *testing.T) {
	ds, err := Open(sampleDataFixturePath(t))
	require.NoError(t, err)
	defer ds.Close()

	textDataset, err := ds.GetTextDataset("County_T")
	require.NoError(t, err)
	assert.Equal(t, "County_T", textDataset.Info().Name)
	assert.Equal(t, types.DatasetKindText, textDataset.Info().Kind)
	assertSRID(t, textDataset.Info(), 4326)
	textCount, err := textDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 15, textCount)

	features, err := textDataset.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	require.Len(t, features, 1)

	geometry, ok := features[0].Geometry.(*types.TextGeometry)
	require.True(t, ok)
	assert.Len(t, geometry.Anchor, 2)
	require.NotNil(t, geometry.Style)
}

func TestRealSampleDataTabularDataset(t *testing.T) {
	ds, err := Open(sampleDataFixturePath(t))
	require.NoError(t, err)
	defer ds.Close()

	tabularDataset, err := ds.GetTabularDataset("TabularDT")
	require.NoError(t, err)
	assert.Equal(t, "TabularDT", tabularDataset.Info().Name)
	assert.Equal(t, "TabularDT", tabularDataset.Info().TableName)
	assert.Equal(t, types.DatasetKindTabular, tabularDataset.Info().Kind)
	assertSRID(t, tabularDataset.Info(), 0)
	tabularCount, err := tabularDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 15, tabularCount)

	record, err := tabularDataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, 1, record.ID)
	assert.Equal(t, int64(110227), record.Attributes["ADMI"])
	assert.Equal(t, "怀柔区", record.Attributes["NAME"])
	assert.Equal(t, "北京市", record.Attributes["City"])
	assert.InDelta(t, 26.5, numericValue(t, record.Attributes["POP_1999"]), 0.000000001)
	_, hasGeometry := record.Attributes["SmGeometry"]
	assert.False(t, hasGeometry)
}

func TestRealHenanTextDataset(t *testing.T) {
	ds, err := Open(henanFixturePath(t))
	require.NoError(t, err)
	defer ds.Close()

	textDataset, err := ds.GetTextDataset("河南省标签")
	require.NoError(t, err)
	assert.Equal(t, "河南省标签", textDataset.Info().Name)
	assert.Equal(t, types.DatasetKindText, textDataset.Info().Kind)
	henanTextCount, err := textDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 1, henanTextCount)

	feature, err := textDataset.GetByID(1)
	require.NoError(t, err)
	assert.Equal(t, 1, feature.ID)

	geometry, ok := feature.Geometry.(*types.TextGeometry)
	require.True(t, ok)
	assert.Equal(t, "河南省", geometry.Text)
	assert.InDeltaSlice(t, []float64{113.165187569688, 33.875453985}, geometry.Anchor, 0.000000001)
	require.NotNil(t, geometry.Style)
	assert.Equal(t, "宋体", geometry.Style.FaceName)
	assert.InDelta(t, 0.406494140625, geometry.Style.FontHeight, 0.000000001)
}

func TestRealHenanRepresentativeVectorDatasets(t *testing.T) {
	ds, err := Open(henanFixturePath(t))
	require.NoError(t, err)
	defer ds.Close()

	pointDataset, err := ds.GetPointDataset("居民地地名")
	require.NoError(t, err)
	pointCount, err := pointDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 1863, pointCount)
	pointFeatures, err := pointDataset.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	assert.Len(t, pointFeatures, 1)

	lineDataset, err := ds.GetLineDataset("公路")
	require.NoError(t, err)
	lineCount, err := lineDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 7805, lineCount)
	lineFeatures, err := lineDataset.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	assert.Len(t, lineFeatures, 1)

	regionDataset, err := ds.GetRegionDataset("省级行政区划")
	require.NoError(t, err)
	regionCount, err := regionDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 1, regionCount)
	regionFeatures, err := regionDataset.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	assert.Len(t, regionFeatures, 1)
}

func TestRealHenanMixedSridRegionDataset(t *testing.T) {
	ds, err := Open(henanFixturePath(t))
	require.NoError(t, err)
	defer ds.Close()

	regionDataset, err := ds.GetRegionDataset("水库面")
	require.NoError(t, err)
	assert.Equal(t, "水库面", regionDataset.Info().Name)
	assert.Equal(t, types.DatasetKindRegion, regionDataset.Info().Kind)
	assertSRID(t, regionDataset.Info(), 3857)
	mixedSridCount, err := regionDataset.Count()
	require.NoError(t, err)
	assert.Equal(t, 71, mixedSridCount)

	features, err := regionDataset.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	require.Len(t, features, 1)
	geometry, ok := features[0].Geometry.(*types.MultiPolygonGeometry)
	require.True(t, ok)
	assert.Equal(t, "MultiPolygon", geometry.GeometryType())
}

func TestRealHenanDatasetNameCanDifferFromTableName(t *testing.T) {
	ds, err := Open(henanFixturePath(t))
	require.NoError(t, err)
	defer ds.Close()

	streetRoad, err := ds.GetPointDataset("streetroad")
	require.NoError(t, err)
	assert.Equal(t, "streetroad", streetRoad.Info().Name)
	assert.Equal(t, "街道", streetRoad.Info().TableName)
	streetRoadCount, err := streetRoad.Count()
	require.NoError(t, err)
	assert.Equal(t, 63, streetRoadCount)
	streetRoadFeatures, err := streetRoad.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	require.Len(t, streetRoadFeatures, 1)

	gRoad, err := ds.GetLineDataset("groad")
	require.NoError(t, err)
	assert.Equal(t, "groad", gRoad.Info().Name)
	assert.Equal(t, "国道", gRoad.Info().TableName)
	gRoadCount, err := gRoad.Count()
	require.NoError(t, err)
	assert.Equal(t, 164, gRoadCount)
	gRoadFeatures, err := gRoad.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	require.Len(t, gRoadFeatures, 1)

	city, err := ds.GetRegionDataset("city")
	require.NoError(t, err)
	assert.Equal(t, "city", city.Info().Name)
	assert.Equal(t, "市级行政区划", city.Info().TableName)
	cityCount, err := city.Count()
	require.NoError(t, err)
	assert.Equal(t, 18, cityCount)
	cityFeatures, err := city.List(&types.QueryOptions{Limit: 1})
	require.NoError(t, err)
	require.Len(t, cityFeatures, 1)
}

func TestRealHenanLargePointDatasetPaginationViaTableNameMapping(t *testing.T) {
	ds, err := Open(henanFixturePath(t))
	require.NoError(t, err)
	defer ds.Close()

	weibo, err := ds.GetPointDataset("weibo")
	require.NoError(t, err)
	assert.Equal(t, "weibo", weibo.Info().Name)
	assert.Equal(t, "henan_P", weibo.Info().TableName)
	assert.Equal(t, types.DatasetKindPoint, weibo.Info().Kind)
	assertSRID(t, weibo.Info(), 4326)
	weiboCount, err := weibo.Count()
	require.NoError(t, err)
	assert.Equal(t, 469308, weiboCount)

	firstPage, err := weibo.List(&types.QueryOptions{Limit: 3, Offset: 0})
	require.NoError(t, err)
	require.Len(t, firstPage, 3)
	assert.Equal(t, []int{1, 2, 3}, []int{firstPage[0].ID, firstPage[1].ID, firstPage[2].ID})

	secondPage, err := weibo.List(&types.QueryOptions{Limit: 3, Offset: 3})
	require.NoError(t, err)
	require.Len(t, secondPage, 3)
	assert.Equal(t, []int{4, 5, 6}, []int{secondPage[0].ID, secondPage[1].ID, secondPage[2].ID})
	assert.Equal(t, int64(0), firstPage[0].Attributes["count1"])
	assert.Equal(t, int64(0), firstPage[0].Attributes["count2"])
}

func assertSRID(t *testing.T, info *types.DatasetInfo, expected int) {
	t.Helper()
	require.NotNil(t, info.SRID)
	assert.Equal(t, expected, *info.SRID)
}
