package udbx4go

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/dataset"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func requireExternalFixturePath(t testing.TB, path string) string {
	t.Helper()
	available, err := externalFixtureAvailable(path, os.Getenv("UDBX_REAL_SAMPLES") == "1")
	require.NoError(t, err, "real sample is required when UDBX_REAL_SAMPLES=1")
	if !available {
		t.Skipf("external fixture not available: %s", path)
	}
	return path
}

func externalFixtureAvailable(path string, required bool) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) && !required {
		return false, nil
	}
	return false, err
}

func TestExternalFixtureAvailabilityPolicy(t *testing.T) {
	existing := filepath.Join(t.TempDir(), "sample.udbx")
	require.NoError(t, os.WriteFile(existing, nil, 0o600))

	available, err := externalFixtureAvailable(existing, false)
	require.NoError(t, err)
	assert.True(t, available)

	missing := filepath.Join(t.TempDir(), "missing.udbx")
	available, err = externalFixtureAvailable(missing, false)
	require.NoError(t, err)
	assert.False(t, available)

	available, err = externalFixtureAvailable(missing, true)
	assert.False(t, available)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func sampleDataFixturePath(t testing.TB) string {
	t.Helper()
	return requireExternalFixturePath(t, filepath.Join("..", "data", "SampleData.udbx"))
}

func henanFixturePath(t testing.TB) string {
	t.Helper()
	return requireExternalFixturePath(t, filepath.Join("..", "data", "henan.udbx"))
}

func openRealHenanReadOnly(t testing.TB) *DataSource {
	t.Helper()
	path, err := filepath.Abs(henanFixturePath(t))
	require.NoError(t, err)

	dsn := url.URL{Scheme: "file", Path: path}
	query := dsn.Query()
	query.Set("mode", "ro")
	query.Set("immutable", "1")
	dsn.RawQuery = query.Encode()

	db, err := sql.Open("sqlite3", dsn.String())
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	require.NoError(t, db.Ping())
	return newDataSource(db)
}

func TestRealHenanWeiboSpatialQueryUsesRTreeAndViewportMBR(t *testing.T) {
	ds := openRealHenanReadOnly(t)
	defer ds.Close()
	ctx := context.Background()
	bounds := BoundingBox{MinX: 113.5, MinY: 34.5, MaxX: 114.0, MaxY: 35.0}
	const limit = 1000

	capability, err := ds.GetSpatialQueryCapability(ctx, "weibo")
	require.NoError(t, err)
	assert.True(t, capability.RTreeAvailable)

	result, err := ds.QuerySpatial(ctx, "weibo", SpatialQueryOptions{Bounds: bounds, Limit: limit})
	require.NoError(t, err)
	assert.Equal(t, SpatialQueryStrategyRTree, result.Strategy)
	assert.LessOrEqual(t, len(result.Features), limit)
	assertOrdinaryFeaturesIntersect(t, result.Features, bounds)
}

func TestRealHenanCountySpatialQueryUsesFallbackWithoutRTree(t *testing.T) {
	ds := openRealHenanReadOnly(t)
	defer ds.Close()
	ctx := context.Background()
	bounds := BoundingBox{MinX: 113.5, MinY: 34.5, MaxX: 114.0, MaxY: 35.0}

	capability, err := ds.GetSpatialQueryCapability(ctx, "县级行政区划")
	require.NoError(t, err)
	assert.False(t, capability.RTreeAvailable)

	result, err := ds.QuerySpatial(ctx, "县级行政区划", SpatialQueryOptions{Bounds: bounds, Limit: 100})
	require.NoError(t, err)
	switch result.Strategy {
	case SpatialQueryStrategyEnvelopeCache:
		assert.Empty(t, result.DegradedReason)
		assertOrdinaryFeaturesIntersect(t, result.Features, bounds)
	case SpatialQueryStrategyBoundedSample:
		assert.Equal(t, SpatialQueryReasonEnvelopeCacheBudgetExceeded, result.DegradedReason)
	default:
		t.Fatalf("unexpected county spatial query strategy: %s", result.Strategy)
	}
}

func TestRealHenanRoadSpatialQueryUsesRTreeWithChinesePhysicalTable(t *testing.T) {
	ds := openRealHenanReadOnly(t)
	defer ds.Close()
	ctx := context.Background()
	bounds := BoundingBox{MinX: 113.5, MinY: 34.5, MaxX: 114.0, MaxY: 35.0}

	capability, err := ds.GetSpatialQueryCapability(ctx, "公路")
	require.NoError(t, err)
	assert.True(t, capability.RTreeAvailable)

	result, err := ds.QuerySpatial(ctx, "公路", SpatialQueryOptions{Bounds: bounds, Limit: 100})
	require.NoError(t, err)
	assert.Equal(t, SpatialQueryStrategyRTree, result.Strategy)
	assert.NotEmpty(t, result.Features)
	assert.LessOrEqual(t, len(result.Features), 100)
	assertOrdinaryFeaturesIntersect(t, result.Features, bounds)
}

func TestSpatialQueryLatencyP95UsesNearestRank(t *testing.T) {
	samples := make([]time.Duration, 20)
	for i := range samples {
		samples[i] = time.Duration(i+1) * time.Millisecond
	}
	assert.Equal(t, 19*time.Millisecond, spatialQueryLatencyP95(samples))
}

func TestRealHenanSpatialQueryLatencyP95(t *testing.T) {
	const sampleCount = 20
	ctx := context.Background()
	weiboOptions := SpatialQueryOptions{Bounds: BoundingBox{
		MinX: 113.5, MinY: 34.5, MaxX: 114.0, MaxY: 35.0,
	}, Limit: 1000}
	countyOptions := SpatialQueryOptions{Bounds: weiboOptions.Bounds, Limit: 100}

	t.Run("WeiboRTreeHot", func(t *testing.T) {
		ds := openRealHenanReadOnly(t)
		defer ds.Close()
		warm, err := ds.QuerySpatial(ctx, "weibo", weiboOptions)
		require.NoError(t, err)
		require.Equal(t, SpatialQueryStrategyRTree, warm.Strategy)

		samples := make([]time.Duration, sampleCount)
		for i := range samples {
			started := time.Now()
			result, err := ds.QuerySpatial(ctx, "weibo", weiboOptions)
			samples[i] = time.Since(started)
			require.NoError(t, err)
			require.Equal(t, SpatialQueryStrategyRTree, result.Strategy)
		}
		assertSpatialQueryLatencyP95(t, "Weibo RTree hot", samples, 100*time.Millisecond)
	})

	t.Run("CountyColdBuild", func(t *testing.T) {
		samples := make([]time.Duration, sampleCount)
		for i := range samples {
			ds := openRealHenanReadOnly(t)
			started := time.Now()
			result, err := ds.QuerySpatial(ctx, "县级行政区划", countyOptions)
			samples[i] = time.Since(started)
			closeErr := ds.Close()
			require.NoError(t, err)
			require.Equal(t, SpatialQueryStrategyEnvelopeCache, result.Strategy)
			require.NoError(t, closeErr)
		}
		assertSpatialQueryLatencyP95(t, "County cold build", samples, 500*time.Millisecond)
	})

	t.Run("CountyHotFilter", func(t *testing.T) {
		ds := openRealHenanReadOnly(t)
		defer ds.Close()
		warm, err := ds.QuerySpatial(ctx, "县级行政区划", countyOptions)
		require.NoError(t, err)
		require.Equal(t, SpatialQueryStrategyEnvelopeCache, warm.Strategy)

		samples := make([]time.Duration, sampleCount)
		for i := range samples {
			started := time.Now()
			result, err := ds.QuerySpatial(ctx, "县级行政区划", countyOptions)
			samples[i] = time.Since(started)
			require.NoError(t, err)
			require.Equal(t, SpatialQueryStrategyEnvelopeCache, result.Strategy)
		}
		assertSpatialQueryLatencyP95(t, "County hot filter", samples, 100*time.Millisecond)
	})
}

func TestRealHenanWeiboSpatialQueryRequiredOutsideViewportDoesNotAffectHasMore(t *testing.T) {
	ds := openRealHenanReadOnly(t)
	defer ds.Close()
	ctx := context.Background()
	bounds := BoundingBox{MinX: 113.5, MinY: 34.5, MaxX: 114.0, MaxY: 35.0}
	const limit = 1

	ordinary, err := ds.QuerySpatial(ctx, "weibo", SpatialQueryOptions{Bounds: bounds, Limit: limit})
	require.NoError(t, err)
	require.NotEmpty(t, ordinary.Features)

	weibo, err := ds.GetPointDataset("weibo")
	require.NoError(t, err)
	candidates, err := weibo.List(&types.QueryOptions{Limit: 100})
	require.NoError(t, err)
	requiredID := firstFeatureOutsideBounds(t, candidates, bounds)
	required, err := ds.QuerySpatial(ctx, "weibo", SpatialQueryOptions{
		Bounds: bounds, Limit: limit, RequiredIDs: []int{requiredID},
	})
	require.NoError(t, err)
	assert.Equal(t, ordinary.HasMore, required.HasMore)
	assert.LessOrEqual(t, len(required.Features), limit+1)

	foundRequired := false
	for _, feature := range required.Features {
		if feature.ID == requiredID {
			foundRequired = true
			assert.False(t, bounds.Intersects(featureBoundingBox(t, feature)))
		}
	}
	assert.True(t, foundRequired)
}

func spatialQueryLatencyP95(samples []time.Duration) time.Duration {
	ordered := append([]time.Duration(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	rank := (95*len(ordered) + 99) / 100
	return ordered[rank-1]
}

func assertSpatialQueryLatencyP95(t *testing.T, name string, samples []time.Duration, threshold time.Duration) {
	t.Helper()
	p95 := spatialQueryLatencyP95(samples)
	t.Logf("%s: samples=%d p95=%s threshold=%s", name, len(samples), p95, threshold)
	assert.LessOrEqual(t, p95, threshold)
}

func firstFeatureOutsideBounds(t *testing.T, features []*Feature, bounds BoundingBox) int {
	t.Helper()
	for _, feature := range features {
		if !bounds.Intersects(featureBoundingBox(t, feature)) {
			return feature.ID
		}
	}
	t.Fatal("expected the real sample candidate page to contain an offscreen feature")
	return 0
}

func assertOrdinaryFeaturesIntersect(t *testing.T, features []*Feature, bounds BoundingBox) {
	t.Helper()
	for _, feature := range features {
		assert.Truef(t, bounds.Intersects(featureBoundingBox(t, feature)), "feature %d MBR must intersect viewport", feature.ID)
	}
}

func featureBoundingBox(t *testing.T, feature *Feature) BoundingBox {
	t.Helper()
	require.NotNil(t, feature.Geometry)
	bbox := feature.Geometry.GetBBox()
	require.Len(t, bbox, 4)
	return BoundingBox{MinX: bbox[0], MinY: bbox[1], MaxX: bbox[2], MaxY: bbox[3]}
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
