package codec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestNewGaiaGeometryCodec(t *testing.T) {
	codec := NewGaiaGeometryCodec()
	assert.NotNil(t, codec)
	assert.NotNil(t, codec.pointCodec)
	assert.NotNil(t, codec.lineCodec)
	assert.NotNil(t, codec.polygonCodec)
}

func TestIsValidGeoType(t *testing.T) {
	tests := []struct {
		geoType  int32
		expected bool
	}{
		{GeoTypePoint, true},
		{GeoTypeMultiLineString, true},
		{GeoTypeMultiPolygon, true},
		{GeoTypePointZ, true},
		{GeoTypeMultiLineStringZ, true},
		{GeoTypeMultiPolygonZ, true},
		{999, false},
		{0, false},
		{-1, false},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.geoType)), func(t *testing.T) {
			assert.Equal(t, tt.expected, IsValidGeoType(tt.geoType))
		})
	}
}

func TestGeoTypeToGeometryType(t *testing.T) {
	tests := []struct {
		geoType         int32
		expectedType    string
		expectedValid   bool
	}{
		{GeoTypePoint, "Point", true},
		{GeoTypeMultiLineString, "MultiLineString", true},
		{GeoTypeMultiPolygon, "MultiPolygon", true},
		{GeoTypePointZ, "Point", true},
		{GeoTypeMultiLineStringZ, "MultiLineString", true},
		{GeoTypeMultiPolygonZ, "MultiPolygon", true},
		{999, "", false},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.geoType)), func(t *testing.T) {
			geomType, valid := GeoTypeToGeometryType(tt.geoType)
			assert.Equal(t, tt.expectedValid, valid)
			if tt.expectedValid {
				assert.Equal(t, tt.expectedType, geomType)
			}
		})
	}
}

func TestIsZGeoType(t *testing.T) {
	tests := []struct {
		geoType  int32
		expected bool
	}{
		{GeoTypePoint, false},
		{GeoTypeMultiLineString, false},
		{GeoTypeMultiPolygon, false},
		{GeoTypePointZ, true},
		{GeoTypeMultiLineStringZ, true},
		{GeoTypeMultiPolygonZ, true},
	}

	for _, tt := range tests {
		t.Run(string(rune(tt.geoType)), func(t *testing.T) {
			assert.Equal(t, tt.expected, IsZGeoType(tt.geoType))
		})
	}
}

func TestGaiaGeometryCodec_EncodeDecode_Point(t *testing.T) {
	codec := NewGaiaGeometryCodec()

	original := &types.PointGeometry{
		Type:        "Point",
		Coordinates: []float64{116.4, 39.9},
		SRID:        4326,
		BBox:        []float64{116.4, 39.9, 116.4, 39.9},
	}

	// Encode
	data, err := codec.Encode(original, 4326)
	require.NoError(t, err)
	require.NotNil(t, data)

	// Decode
	decoded, err := codec.Decode(data)
	require.NoError(t, err)
	require.NotNil(t, decoded)

	// Verify
	point, ok := decoded.(*types.PointGeometry)
	require.True(t, ok)
	assert.Equal(t, "Point", point.Type)
	assert.InDelta(t, 116.4, point.Coordinates[0], 0.0001)
	assert.InDelta(t, 39.9, point.Coordinates[1], 0.0001)
	assert.Equal(t, 4326, point.SRID)
}

func TestGaiaGeometryCodec_EncodeDecode_PointZ(t *testing.T) {
	codec := NewGaiaGeometryCodec()

	original := &types.PointGeometry{
		Type:        "Point",
		Coordinates: []float64{116.4, 39.9, 100.0},
		SRID:        4326,
		HasZValue:   true,
	}

	// Encode
	data, err := codec.Encode(original, 4326)
	require.NoError(t, err)
	require.NotNil(t, data)

	// Decode
	decoded, err := codec.Decode(data)
	require.NoError(t, err)
	require.NotNil(t, decoded)

	// Verify
	point, ok := decoded.(*types.PointGeometry)
	require.True(t, ok)
	assert.Equal(t, "Point", point.Type)
	assert.InDelta(t, 116.4, point.Coordinates[0], 0.0001)
	assert.InDelta(t, 39.9, point.Coordinates[1], 0.0001)
	assert.InDelta(t, 100.0, point.Coordinates[2], 0.0001)
	assert.True(t, point.HasZ())
}

func TestGaiaGeometryCodec_Decode_UnsupportedGeoType(t *testing.T) {
	codec := NewGaiaGeometryCodec()

	// Create a valid GAIA header with unsupported geoType
	header := WriteGaiaHeader(4326, [4]float64{0, 0, 1, 1}, 999)

	_, err := codec.Decode(header)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}
