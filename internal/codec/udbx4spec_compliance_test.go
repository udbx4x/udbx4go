package codec

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestUdbx4SpecGoldenBytesDecodeAndEncode(t *testing.T) {
	fixtures := []struct {
		name       string
		path       string
		srid       int
		assertGeom func(t *testing.T, geometry types.Geometry)
	}{
		{
			name: "point-2d",
			path: "point-2d/simple.bin",
			srid: 4326,
			assertGeom: func(t *testing.T, geometry types.Geometry) {
				point, ok := geometry.(*types.PointGeometry)
				require.True(t, ok)
				assert.Equal(t, "Point", point.GeometryType())
				assert.False(t, point.HasZ())
				assert.Equal(t, 4326, point.GetSRID())
				assert.InDeltaSlice(t, []float64{116.123, 39.456}, point.Coordinates, 0.000000001)
				assert.InDeltaSlice(t, []float64{116.123, 39.456, 116.123, 39.456}, point.GetBBox(), 0.000000001)
			},
		},
		{
			name: "point-3d",
			path: "point-3d/simple.bin",
			srid: 4326,
			assertGeom: func(t *testing.T, geometry types.Geometry) {
				point, ok := geometry.(*types.PointGeometry)
				require.True(t, ok)
				assert.Equal(t, "Point", point.GeometryType())
				assert.True(t, point.HasZ())
				assert.Equal(t, 4326, point.GetSRID())
				assert.InDeltaSlice(t, []float64{116.123, 39.456, 12.5}, point.Coordinates, 0.000000001)
				assert.InDeltaSlice(t, []float64{116.123, 39.456, 116.123, 39.456}, point.GetBBox(), 0.000000001)
			},
		},
		{
			name: "multilinestring-2d",
			path: "multilinestring-2d/simple.bin",
			srid: 4326,
			assertGeom: func(t *testing.T, geometry types.Geometry) {
				line, ok := geometry.(*types.MultiLineStringGeometry)
				require.True(t, ok)
				assert.Equal(t, "MultiLineString", line.GeometryType())
				assert.False(t, line.HasZ())
				assert.Equal(t, 4326, line.GetSRID())
				require.Len(t, line.Coordinates, 1)
				require.Len(t, line.Coordinates[0], 2)
				assert.InDeltaSlice(t, []float64{116.123, 39.456}, line.Coordinates[0][0], 0.000000001)
				assert.InDeltaSlice(t, []float64{117, 40}, line.Coordinates[0][1], 0.000000001)
				assert.InDeltaSlice(t, []float64{116.123, 39.456, 117, 40}, line.GetBBox(), 0.000000001)
			},
		},
		{
			name: "multilinestring-3d",
			path: "multilinestring-3d/simple.bin",
			srid: 4326,
			assertGeom: func(t *testing.T, geometry types.Geometry) {
				line, ok := geometry.(*types.MultiLineStringGeometry)
				require.True(t, ok)
				assert.Equal(t, "MultiLineString", line.GeometryType())
				assert.True(t, line.HasZ())
				assert.Equal(t, 4326, line.GetSRID())
				require.Len(t, line.Coordinates, 1)
				require.Len(t, line.Coordinates[0], 2)
				assert.InDeltaSlice(t, []float64{116.123, 39.456, 12.5}, line.Coordinates[0][0], 0.000000001)
				assert.InDeltaSlice(t, []float64{117, 40, 18.75}, line.Coordinates[0][1], 0.000000001)
				assert.InDeltaSlice(t, []float64{116.123, 39.456, 117, 40}, line.GetBBox(), 0.000000001)
			},
		},
		{
			name: "multipolygon-2d",
			path: "multipolygon-2d/simple.bin",
			srid: 4326,
			assertGeom: func(t *testing.T, geometry types.Geometry) {
				polygon, ok := geometry.(*types.MultiPolygonGeometry)
				require.True(t, ok)
				assert.Equal(t, "MultiPolygon", polygon.GeometryType())
				assert.False(t, polygon.HasZ())
				assert.Equal(t, 4326, polygon.GetSRID())
				require.Len(t, polygon.Coordinates, 1)
				require.Len(t, polygon.Coordinates[0], 1)
				require.Len(t, polygon.Coordinates[0][0], 5)
				assert.InDeltaSlice(t, []float64{116.123, 39.456}, polygon.Coordinates[0][0][0], 0.000000001)
				assert.InDeltaSlice(t, []float64{117, 40}, polygon.Coordinates[0][0][2], 0.000000001)
				assert.InDeltaSlice(t, []float64{116.123, 39.456, 117, 40}, polygon.GetBBox(), 0.000000001)
			},
		},
		{
			name: "multipolygon-3d",
			path: "multipolygon-3d/simple.bin",
			srid: 4326,
			assertGeom: func(t *testing.T, geometry types.Geometry) {
				polygon, ok := geometry.(*types.MultiPolygonGeometry)
				require.True(t, ok)
				assert.Equal(t, "MultiPolygon", polygon.GeometryType())
				assert.True(t, polygon.HasZ())
				assert.Equal(t, 4326, polygon.GetSRID())
				require.Len(t, polygon.Coordinates, 1)
				require.Len(t, polygon.Coordinates[0], 1)
				require.Len(t, polygon.Coordinates[0][0], 5)
				assert.InDeltaSlice(t, []float64{116.123, 39.456, 12.5}, polygon.Coordinates[0][0][0], 0.000000001)
				assert.InDeltaSlice(t, []float64{117, 40, 14}, polygon.Coordinates[0][0][2], 0.000000001)
				assert.InDeltaSlice(t, []float64{116.123, 39.456, 117, 40}, polygon.GetBBox(), 0.000000001)
			},
		},
	}

	codec := NewGaiaGeometryCodec()
	baseDir := requireExternalFixtureDir(t, filepath.Join("..", "..", "..", "udbx4spec", "compliance", "golden-gaia-bytes"))

	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			goldenBytes, err := os.ReadFile(filepath.Join(baseDir, fixture.path))
			require.NoError(t, err)

			geometry, err := codec.Decode(goldenBytes)
			require.NoError(t, err)
			fixture.assertGeom(t, geometry)

			encodedBytes, err := codec.Encode(geometry, fixture.srid)
			require.NoError(t, err)
			assert.True(t, bytes.Equal(goldenBytes, encodedBytes), "重新编码结果必须与 Golden Bytes 字节级一致")
		})
	}
}

func TestUdbx4SpecSourceDerivedGeoTextDecode(t *testing.T) {
	baseDir := requireExternalFixtureDir(t, filepath.Join("..", "..", "..", "udbx4spec", "compliance", "source-derived"))
	fixtureBytes, err := os.ReadFile(filepath.Join(baseDir, "sampledata/county-t/smid-1-smgeometry.bin"))
	require.NoError(t, err)

	geometry, err := NewGeoTextCodec().Decode(fixtureBytes)
	require.NoError(t, err)

	assert.Equal(t, "Text", geometry.GeometryType())
	assert.Equal(t, "����                          ", geometry.Text)
	assert.InDeltaSlice(t, []float64{117.30733958523862, 39.95693237286111}, geometry.Anchor, 0.000000001)
	assert.InDelta(t, -3.2, geometry.Rotation, 0.000000001)
	require.NotNil(t, geometry.Style)
	assert.Equal(t, "����", geometry.Style.FaceName)
	assert.Equal(t, 0, geometry.Style.FixedSize)
	assert.Equal(t, 80, geometry.Style.Weight)
	assert.Equal(t, 0, geometry.Style.StyleFlag)
	assert.Equal(t, 37, geometry.Style.AlignFlag)
	assert.InDelta(t, 3.7, geometry.Style.FontHeight, 0.000000001)
	require.Len(t, geometry.SubTexts, 1)
	assert.Equal(t, "����                          ", geometry.SubTexts[0].Text)
	assert.InDelta(t, -3.2, geometry.SubTexts[0].Rotation, 0.000000001)
}

func TestUdbx4SpecStableSourceDerivedMetadata(t *testing.T) {
	baseDir := requireExternalFixtureDir(t, filepath.Join("..", "..", "..", "udbx4spec", "compliance", "source-derived"))
	manifestBytes, err := os.ReadFile(filepath.Join(baseDir, "manifest.json"))
	require.NoError(t, err)

	var manifest struct {
		Fixtures []struct {
			ID              string `json:"id"`
			Tier            string `json:"tier"`
			Stability       string `json:"stability"`
			LicenseStatus   string `json:"licenseStatus"`
			LicenseDocument string `json:"licenseDocument"`
			Generator       struct {
				Product        string `json:"product"`
				ProductVersion string `json:"productVersion"`
			} `json:"generator"`
		} `json:"fixtures"`
	}
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))

	expectedIDs := map[string]bool{
		"sampledata-county-t-smid-1-smgeometry": false,
		"sampledata-caddt-smid-1-smgeometry":    false,
		"sampledata-caddt-smid-16-smgeometry":   false,
		"sampledata-caddt-smid-63-smgeometry":   false,
		"sampledata-3d-srid-zero-metadata":      false,
	}
	for _, fixture := range manifest.Fixtures {
		if _, ok := expectedIDs[fixture.ID]; !ok {
			continue
		}
		expectedIDs[fixture.ID] = true
		assert.Equal(t, "T3", fixture.Tier)
		assert.Equal(t, "stable", fixture.Stability)
		assert.Equal(t, "public-confirmed", fixture.LicenseStatus)
		assert.Equal(t, "docs/samples/licenses/sampledata-public-distribution-confirmation.md", fixture.LicenseDocument)
		assert.Equal(t, "SuperMap iDesktopX 2025", fixture.Generator.Product)
		assert.Equal(t, "V12.0.1.0", fixture.Generator.ProductVersion)
	}
	for id, seen := range expectedIDs {
		assert.True(t, seen, "missing stable T3 fixture %s", id)
	}

	metadataBytes, err := os.ReadFile(filepath.Join(baseDir, "sampledata/3d-srid-zero/metadata.json"))
	require.NoError(t, err)
	var metadata struct {
		Datasets []struct {
			Name               string `json:"name"`
			SmSRID             int    `json:"smSRID"`
			GeometryColumnSRID int    `json:"geometryColumnSRID"`
			CoordDimension     int    `json:"coordDimension"`
			SampleFeature      struct {
				GaiaHeaderSRID int `json:"gaiaHeaderSRID"`
			} `json:"sampleFeature"`
		} `json:"datasets"`
	}
	require.NoError(t, json.Unmarshal(metadataBytes, &metadata))
	require.Len(t, metadata.Datasets, 3)
	assert.Equal(t, "BaseMap_PZ", metadata.Datasets[0].Name)
	assert.Equal(t, "BaseMap_LZ", metadata.Datasets[1].Name)
	assert.Equal(t, "BaseMap_RZ", metadata.Datasets[2].Name)
	for _, dataset := range metadata.Datasets {
		assert.Equal(t, 0, dataset.SmSRID)
		assert.Equal(t, 0, dataset.GeometryColumnSRID)
		assert.Equal(t, 3, dataset.CoordDimension)
		assert.Equal(t, 0, dataset.SampleFeature.GaiaHeaderSRID)
	}
}

func TestUdbx4SpecSourceDerivedCadDecode(t *testing.T) {
	baseDir := requireExternalFixtureDir(t, filepath.Join("..", "..", "..", "udbx4spec", "compliance", "source-derived"))
	codec := NewCadGeometryCodec()

	pointBytes, err := os.ReadFile(filepath.Join(baseDir, "sampledata/caddt/smid-1-smgeometry.bin"))
	require.NoError(t, err)
	pointGeometry, err := codec.Decode(pointBytes)
	require.NoError(t, err)
	point, ok := pointGeometry.(*types.CadPointGeometry)
	require.True(t, ok)
	assert.Equal(t, "CadPoint", point.GeometryType())
	assert.InDelta(t, 117.39993002089763, point.XCoord, 0.000000001)
	assert.InDelta(t, 40.0590434404585, point.YCoord, 0.000000001)
	assert.Nil(t, point.Style)

	lineBytes, err := os.ReadFile(filepath.Join(baseDir, "sampledata/caddt/smid-16-smgeometry.bin"))
	require.NoError(t, err)
	lineGeometry, err := codec.Decode(lineBytes)
	require.NoError(t, err)
	line, ok := lineGeometry.(*types.CadLineGeometry)
	require.True(t, ok)
	assert.Equal(t, "CadLine", line.GeometryType())
	assert.Equal(t, 1, line.NumSub)
	assert.Equal(t, []int{17}, line.SubPointCounts)
	require.Len(t, line.Coordinates, 17)
	assert.InDelta(t, 116.68328592226102, line.Coordinates[0][0], 0.000000001)
	assert.InDelta(t, 40.995215339741925, line.Coordinates[0][1], 0.000000001)
	assert.InDelta(t, 116.36793930503393, line.Coordinates[16][0], 0.000000001)
	assert.InDelta(t, 40.89292722915481, line.Coordinates[16][1], 0.000000001)
	assert.Nil(t, line.Style)

	regionBytes, err := os.ReadFile(filepath.Join(baseDir, "sampledata/caddt/smid-63-smgeometry.bin"))
	require.NoError(t, err)
	regionGeometry, err := codec.Decode(regionBytes)
	require.NoError(t, err)
	region, ok := regionGeometry.(*types.CadRegionGeometry)
	require.True(t, ok)
	assert.Equal(t, "CadRegion", region.GeometryType())
	assert.Equal(t, 1, region.NumSub)
	assert.Equal(t, []int{120}, region.SubPointCounts)
	require.Len(t, region.Coordinates, 120)
	assert.InDelta(t, 116.65601002454922, region.Coordinates[0][0], 0.000000001)
	assert.InDelta(t, 41.03663585095796, region.Coordinates[0][1], 0.000000001)
	assert.InDelta(t, 116.65601002454922, region.Coordinates[119][0], 0.000000001)
	assert.InDelta(t, 41.03663585095796, region.Coordinates[119][1], 0.000000001)
	assert.Nil(t, region.Style)
}

func requireExternalFixtureDir(t *testing.T, path string) string {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			t.Skipf("external fixture directory not available: %s", path)
		}
		require.NoError(t, err)
	}
	if !info.IsDir() {
		t.Fatalf("external fixture path is not a directory: %s", path)
	}
	return path
}
