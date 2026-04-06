package codec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestGaiaLineCodec_DecodeMultiLineString(t *testing.T) {
	codec := NewGaiaLineCodec()

	// Create a valid GAIA MultiLineString BLOB
	points := [][2]float64{
		{116.4, 39.9},
		{116.5, 39.8},
		{116.6, 39.7},
	}
	mbr := CalculatePointsMBR(points)
	header := WriteGaiaHeader(4326, mbr, GeoTypeMultiLineString)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteInt32(1)       // 1 linestring
	writer.WriteByte(GaiaEntityMark) // entity marker
	writer.WriteInt32(2)       // lineGeoType for 2D
	writer.WriteInt32(3)       // 3 points
	for _, p := range points {
		writer.WriteFloat64(p[0])
		writer.WriteFloat64(p[1])
	}
	writer.WriteByte(GaiaEndMarker)

	data := writer.Bytes()

	// Decode
	line, err := codec.DecodeMultiLineString(data)
	require.NoError(t, err)
	require.NotNil(t, line)

	assert.Equal(t, "MultiLineString", line.Type)
	assert.Len(t, line.Coordinates, 1)
	assert.Len(t, line.Coordinates[0], 3)
	assert.False(t, line.HasZ())
}

func TestGaiaLineCodec_DecodeMultiLineStringZ(t *testing.T) {
	codec := NewGaiaLineCodec()

	// Create a valid GAIA MultiLineStringZ BLOB
	points := [][3]float64{
		{116.4, 39.9, 100.0},
		{116.5, 39.8, 100.0},
		{116.6, 39.7, 100.0},
	}

	points2D := [][2]float64{
		{116.4, 39.9},
		{116.5, 39.8},
		{116.6, 39.7},
	}
	mbr := CalculatePointsMBR(points2D)
	header := WriteGaiaHeader(4326, mbr, GeoTypeMultiLineStringZ)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteInt32(1)       // 1 linestring
	writer.WriteByte(GaiaEntityMark) // entity marker
	writer.WriteInt32(1002)    // lineGeoType for 3D
	writer.WriteInt32(3)       // 3 points
	for _, p := range points {
		writer.WriteFloat64(p[0])
		writer.WriteFloat64(p[1])
		writer.WriteFloat64(p[2])
	}
	writer.WriteByte(GaiaEndMarker)

	data := writer.Bytes()

	// Decode
	line, err := codec.DecodeMultiLineStringZ(data)
	require.NoError(t, err)
	require.NotNil(t, line)

	assert.Equal(t, "MultiLineString", line.Type)
	assert.Len(t, line.Coordinates, 1)
	assert.Len(t, line.Coordinates[0], 3)
	assert.True(t, line.HasZ())

	// Check first point has Z
	assert.Len(t, line.Coordinates[0][0], 3)
	assert.InDelta(t, 100.0, line.Coordinates[0][0][2], 0.0001)
}

func TestGaiaLineCodec_DecodeMultiLineString_InvalidNumLineStrings(t *testing.T) {
	codec := NewGaiaLineCodec()

	mbr := [4]float64{0, 0, 1, 1}
	header := WriteGaiaHeader(4326, mbr, GeoTypeMultiLineString)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteInt32(-1) // Invalid negative count
	writer.WriteInt32(0)  // Need some padding to pass length check
	writer.WriteByte(GaiaEndMarker)

	data := writer.Bytes()

	_, err := codec.DecodeMultiLineString(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid number of linestrings")
}

func TestGaiaLineCodec_DecodeMultiLineString_TooManyLineStrings(t *testing.T) {
	codec := NewGaiaLineCodec()

	mbr := [4]float64{0, 0, 1, 1}
	header := WriteGaiaHeader(4326, mbr, GeoTypeMultiLineString)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteInt32(2000000) // Too many
	writer.WriteInt32(0)       // Need some padding to pass length check
	writer.WriteByte(GaiaEndMarker)

	data := writer.Bytes()

	_, err := codec.DecodeMultiLineString(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid number of linestrings")
}

func TestGaiaLineCodec_EncodeMultiLineString(t *testing.T) {
	codec := NewGaiaLineCodec()

	line := &types.MultiLineStringGeometry{
		Type: "MultiLineString",
		Coordinates: [][][]float64{
			{
				{116.4, 39.9},
				{116.5, 39.8},
				{116.6, 39.7},
			},
		},
	}

	data, err := codec.EncodeMultiLineString(line, 4326)
	require.NoError(t, err)
	require.NotNil(t, data)

	// Decode back to verify
	decoded, err := codec.DecodeMultiLineString(data)
	require.NoError(t, err)

	assert.Len(t, decoded.Coordinates, 1)
	assert.Len(t, decoded.Coordinates[0], 3)
	assert.InDelta(t, 116.4, decoded.Coordinates[0][0][0], 0.0001)
	assert.InDelta(t, 39.9, decoded.Coordinates[0][0][1], 0.0001)
}

func TestGaiaLineCodec_EncodeMultiLineStringZ(t *testing.T) {
	codec := NewGaiaLineCodec()

	line := &types.MultiLineStringGeometry{
		Type: "MultiLineString",
		Coordinates: [][][]float64{
			{
				{116.4, 39.9, 100.0},
				{116.5, 39.8, 100.0},
			},
		},
	}

	data, err := codec.EncodeMultiLineStringZ(line, 4326)
	require.NoError(t, err)
	require.NotNil(t, data)

	// Decode back to verify
	decoded, err := codec.DecodeMultiLineStringZ(data)
	require.NoError(t, err)

	assert.Len(t, decoded.Coordinates, 1)
	assert.Len(t, decoded.Coordinates[0], 2)
	assert.Len(t, decoded.Coordinates[0][0], 3) // Has Z
	assert.InDelta(t, 100.0, decoded.Coordinates[0][0][2], 0.0001)
}

func TestGaiaLineCodec_EncodeMultiLineString_NotEnoughCoordinates(t *testing.T) {
	codec := NewGaiaLineCodec()

	line := &types.MultiLineStringGeometry{
		Type: "MultiLineString",
		Coordinates: [][][]float64{
			{
				{116.4}, // Only 1 coordinate
			},
		},
	}

	_, err := codec.EncodeMultiLineString(line, 4326)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2 values")
}

func TestGaiaLineCodec_EncodeMultiLineStringZ_NotEnoughCoordinates(t *testing.T) {
	codec := NewGaiaLineCodec()

	line := &types.MultiLineStringGeometry{
		Type: "MultiLineString",
		Coordinates: [][][]float64{
			{
				{116.4, 39.9}, // Only 2 coordinates
			},
		},
	}

	_, err := codec.EncodeMultiLineStringZ(line, 4326)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "3 values")
}

func TestGaiaLineCodec_RoundTrip(t *testing.T) {
	codec := NewGaiaLineCodec()

	tests := []struct {
		name string
		line [][][]float64
		srid int
		is3D bool
	}{
		{
			name: "Single 2D line",
			line: [][][]float64{{{116.4, 39.9}, {116.5, 39.8}}},
			srid: 4326,
			is3D: false,
		},
		{
			name: "Multiple 2D lines",
			line: [][][]float64{
				{{116.4, 39.9}, {116.5, 39.8}},
				{{121.5, 31.2}, {121.6, 31.3}},
			},
			srid: 4326,
			is3D: false,
		},
		{
			name: "Single 3D line",
			line: [][][]float64{{{116.4, 39.9, 100.0}, {116.5, 39.8, 100.0}}},
			srid: 4326,
			is3D: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &types.MultiLineStringGeometry{
				Type:        "MultiLineString",
				Coordinates: tt.line,
			}

			var encoded []byte
			var err error
			if tt.is3D {
				encoded, err = codec.EncodeMultiLineStringZ(original, tt.srid)
			} else {
				encoded, err = codec.EncodeMultiLineString(original, tt.srid)
			}
			require.NoError(t, err)

			var decoded *types.MultiLineStringGeometry
			if tt.is3D {
				decoded, err = codec.DecodeMultiLineStringZ(encoded)
			} else {
				decoded, err = codec.DecodeMultiLineString(encoded)
			}
			require.NoError(t, err)

			assert.Equal(t, len(tt.line), len(decoded.Coordinates))
			for i, expectedLine := range tt.line {
				assert.Equal(t, len(expectedLine), len(decoded.Coordinates[i]))
			}
		})
	}
}
