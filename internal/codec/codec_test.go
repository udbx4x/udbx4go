package codec

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	udbxerrors "github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestReadGaiaEnvelopeReads2DAnd3DHeaders(t *testing.T) {
	tests := []struct {
		name    string
		geoType int32
	}{
		{name: "point", geoType: GeoTypePoint},
		{name: "line", geoType: GeoTypeMultiLineString},
		{name: "region", geoType: GeoTypeMultiPolygon},
		{name: "point z", geoType: GeoTypePointZ},
		{name: "line z", geoType: GeoTypeMultiLineStringZ},
		{name: "region z", geoType: GeoTypeMultiPolygonZ},
	}

	want := types.BoundingBox{MinX: -10.5, MinY: -2.25, MaxX: 30.75, MaxY: 40.5}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blob := WriteGaiaHeader(4326, [4]float64{want.MinX, want.MinY, want.MaxX, want.MaxY}, tt.geoType)

			got, err := ReadGaiaEnvelope(blob)

			require.NoError(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestReadGaiaEnvelopeRejectsMalformedHeader(t *testing.T) {
	valid := WriteGaiaHeader(4326, [4]float64{0, 1, 2, 3}, GeoTypePoint)
	tests := []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{name: "short data", mutate: func(data []byte) []byte { return data[:GaiaHeaderLength-1] }},
		{name: "start marker", mutate: func(data []byte) []byte { data[0] = 0xff; return data }},
		{name: "byte order", mutate: func(data []byte) []byte { data[1] = 0x00; return data }},
		{name: "MBR marker", mutate: func(data []byte) []byte { data[38] = 0xff; return data }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blob := tt.mutate(append([]byte(nil), valid...))

			_, err := ReadGaiaEnvelope(blob)

			require.Error(t, err)
			assert.True(t, udbxerrors.IsFormatError(err))
		})
	}
}

func TestReadGaiaEnvelopeRejectsInvalidMBR(t *testing.T) {
	tests := []struct {
		name string
		mbr  [4]float64
	}{
		{name: "NaN", mbr: [4]float64{math.NaN(), 0, 1, 1}},
		{name: "positive infinity", mbr: [4]float64{0, 0, math.Inf(1), 1}},
		{name: "negative infinity", mbr: [4]float64{0, math.Inf(-1), 1, 1}},
		{name: "inverted X", mbr: [4]float64{2, 0, 1, 1}},
		{name: "inverted Y", mbr: [4]float64{0, 2, 1, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blob := WriteGaiaHeader(4326, tt.mbr, GeoTypePoint)

			_, err := ReadGaiaEnvelope(blob)

			require.Error(t, err)
			assert.True(t, udbxerrors.IsFormatError(err))
		})
	}
}

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
