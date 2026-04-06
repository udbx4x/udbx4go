package codec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestGaiaPolygonCodec_DecodeMultiPolygon(t *testing.T) {
	codec := NewGaiaPolygonCodec()

	// Create a valid GAIA MultiPolygon BLOB (simple triangle)
	points := [][2]float64{
		{116.4, 39.9},
		{116.5, 39.9},
		{116.5, 39.8},
		{116.4, 39.9}, // Close the ring
	}
	mbr := CalculatePointsMBR(points)
	header := WriteGaiaHeader(4326, mbr, GeoTypeMultiPolygon)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteInt32(1) // 1 polygon
	writer.WriteByte(GaiaEntityMark) // entity marker
	writer.WriteInt32(3) // polygonGeoType for 2D
	writer.WriteInt32(1) // 1 ring
	writer.WriteInt32(4) // 4 points
	for _, p := range points {
		writer.WriteFloat64(p[0])
		writer.WriteFloat64(p[1])
	}
	writer.WriteByte(GaiaEndMarker)

	data := writer.Bytes()

	// Decode
	polygon, err := codec.DecodeMultiPolygon(data)
	require.NoError(t, err)
	require.NotNil(t, polygon)

	assert.Equal(t, "MultiPolygon", polygon.Type)
	assert.Len(t, polygon.Coordinates, 1)
	assert.Len(t, polygon.Coordinates[0], 1) // 1 ring
	assert.Len(t, polygon.Coordinates[0][0], 4)
	assert.False(t, polygon.HasZ())
}

func TestGaiaPolygonCodec_DecodeMultiPolygonZ(t *testing.T) {
	codec := NewGaiaPolygonCodec()

	// Create a valid GAIA MultiPolygonZ BLOB
	points := [][3]float64{
		{116.4, 39.9, 100.0},
		{116.5, 39.9, 100.0},
		{116.5, 39.8, 100.0},
		{116.4, 39.9, 100.0},
	}

	points2D := [][2]float64{
		{116.4, 39.9},
		{116.5, 39.9},
		{116.5, 39.8},
		{116.4, 39.9},
	}
	mbr := CalculatePointsMBR(points2D)
	header := WriteGaiaHeader(4326, mbr, GeoTypeMultiPolygonZ)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteInt32(1) // 1 polygon
	writer.WriteByte(GaiaEntityMark) // entity marker
	writer.WriteInt32(1003) // polygonGeoType for 3D
	writer.WriteInt32(1) // 1 ring
	writer.WriteInt32(4) // 4 points
	for _, p := range points {
		writer.WriteFloat64(p[0])
		writer.WriteFloat64(p[1])
		writer.WriteFloat64(p[2])
	}
	writer.WriteByte(GaiaEndMarker)

	data := writer.Bytes()

	// Decode
	polygon, err := codec.DecodeMultiPolygonZ(data)
	require.NoError(t, err)
	require.NotNil(t, polygon)

	assert.Equal(t, "MultiPolygon", polygon.Type)
	assert.Len(t, polygon.Coordinates, 1)
	assert.Len(t, polygon.Coordinates[0], 1)
	assert.Len(t, polygon.Coordinates[0][0], 4)
	assert.True(t, polygon.HasZ())

	// Check first point has Z
	assert.Len(t, polygon.Coordinates[0][0][0], 3)
	assert.InDelta(t, 100.0, polygon.Coordinates[0][0][0][2], 0.0001)
}

func TestGaiaPolygonCodec_DecodeMultiPolygon_WithHoles(t *testing.T) {
	codec := NewGaiaPolygonCodec()

	// Create a polygon with outer ring and inner ring (hole)
	outerPoints := [][2]float64{
		{0, 0},
		{10, 0},
		{10, 10},
		{0, 10},
		{0, 0},
	}
	innerPoints := [][2]float64{
		{3, 3},
		{7, 3},
		{7, 7},
		{3, 7},
		{3, 3},
	}

	allPoints := append(outerPoints, innerPoints...)
	mbr := CalculatePointsMBR(allPoints)
	header := WriteGaiaHeader(4326, mbr, GeoTypeMultiPolygon)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteInt32(1) // 1 polygon
	writer.WriteByte(GaiaEntityMark) // entity marker
	writer.WriteInt32(3) // polygonGeoType for 2D
	writer.WriteInt32(2) // 2 rings (outer + hole)

	// Outer ring
	writer.WriteInt32(5)
	for _, p := range outerPoints {
		writer.WriteFloat64(p[0])
		writer.WriteFloat64(p[1])
	}

	// Inner ring (hole)
	writer.WriteInt32(5)
	for _, p := range innerPoints {
		writer.WriteFloat64(p[0])
		writer.WriteFloat64(p[1])
	}

	writer.WriteByte(GaiaEndMarker)

	data := writer.Bytes()

	// Decode
	polygon, err := codec.DecodeMultiPolygon(data)
	require.NoError(t, err)
	require.NotNil(t, polygon)

	assert.Len(t, polygon.Coordinates, 1)
	assert.Len(t, polygon.Coordinates[0], 2) // 2 rings
	assert.Len(t, polygon.Coordinates[0][0], 5)
	assert.Len(t, polygon.Coordinates[0][1], 5)
}

func TestGaiaPolygonCodec_DecodeMultiPolygon_InvalidNumPolygons(t *testing.T) {
	codec := NewGaiaPolygonCodec()

	mbr := [4]float64{0, 0, 1, 1}
	header := WriteGaiaHeader(4326, mbr, GeoTypeMultiPolygon)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteInt32(-1) // Invalid negative count
	writer.WriteInt32(0)  // Need some padding to pass length check
	writer.WriteByte(GaiaEndMarker)

	data := writer.Bytes()

	_, err := codec.DecodeMultiPolygon(data)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid number of polygons")
}

func TestGaiaPolygonCodec_EncodeMultiPolygon(t *testing.T) {
	codec := NewGaiaPolygonCodec()

	polygon := &types.MultiPolygonGeometry{
		Type: "MultiPolygon",
		Coordinates: [][][][]float64{
			{
				{
					{116.4, 39.9},
					{116.5, 39.9},
					{116.5, 39.8},
					{116.4, 39.9},
				},
			},
		},
	}

	data, err := codec.EncodeMultiPolygon(polygon, 4326)
	require.NoError(t, err)
	require.NotNil(t, data)

	// Decode back to verify
	decoded, err := codec.DecodeMultiPolygon(data)
	require.NoError(t, err)

	assert.Len(t, decoded.Coordinates, 1)
	assert.Len(t, decoded.Coordinates[0], 1)
	assert.Len(t, decoded.Coordinates[0][0], 4)
	assert.InDelta(t, 116.4, decoded.Coordinates[0][0][0][0], 0.0001)
	assert.InDelta(t, 39.9, decoded.Coordinates[0][0][0][1], 0.0001)
}

func TestGaiaPolygonCodec_EncodeMultiPolygonZ(t *testing.T) {
	codec := NewGaiaPolygonCodec()

	polygon := &types.MultiPolygonGeometry{
		Type: "MultiPolygon",
		Coordinates: [][][][]float64{
			{
				{
					{116.4, 39.9, 100.0},
					{116.5, 39.9, 100.0},
					{116.5, 39.8, 100.0},
					{116.4, 39.9, 100.0},
				},
			},
		},
	}

	data, err := codec.EncodeMultiPolygonZ(polygon, 4326)
	require.NoError(t, err)
	require.NotNil(t, data)

	// Decode back to verify
	decoded, err := codec.DecodeMultiPolygonZ(data)
	require.NoError(t, err)

	assert.Len(t, decoded.Coordinates, 1)
	assert.Len(t, decoded.Coordinates[0], 1)
	assert.Len(t, decoded.Coordinates[0][0], 4)
	assert.Len(t, decoded.Coordinates[0][0][0], 3) // Has Z
	assert.InDelta(t, 100.0, decoded.Coordinates[0][0][0][2], 0.0001)
}

func TestGaiaPolygonCodec_EncodeMultiPolygon_NotEnoughCoordinates(t *testing.T) {
	codec := NewGaiaPolygonCodec()

	polygon := &types.MultiPolygonGeometry{
		Type: "MultiPolygon",
		Coordinates: [][][][]float64{
			{
				{
					{116.4}, // Only 1 coordinate
				},
			},
		},
	}

	_, err := codec.EncodeMultiPolygon(polygon, 4326)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2 values")
}

func TestGaiaPolygonCodec_EncodeMultiPolygonZ_NotEnoughCoordinates(t *testing.T) {
	codec := NewGaiaPolygonCodec()

	polygon := &types.MultiPolygonGeometry{
		Type: "MultiPolygon",
		Coordinates: [][][][]float64{
			{
				{
					{116.4, 39.9}, // Only 2 coordinates
				},
			},
		},
	}

	_, err := codec.EncodeMultiPolygonZ(polygon, 4326)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "3 values")
}

func TestGaiaPolygonCodec_RoundTrip(t *testing.T) {
	codec := NewGaiaPolygonCodec()

	tests := []struct {
		name    string
		polygon [][][][]float64
		srid    int
		is3D    bool
	}{
		{
			name: "Simple 2D triangle",
			polygon: [][][][]float64{
				{
					{{116.4, 39.9}, {116.5, 39.9}, {116.5, 39.8}, {116.4, 39.9}},
				},
			},
			srid: 4326,
			is3D: false,
		},
		{
			name: "Multiple 2D polygons",
			polygon: [][][][]float64{
				{
					{{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}},
				},
				{
					{{100, 100}, {110, 100}, {110, 110}, {100, 110}, {100, 100}},
				},
			},
			srid: 4326,
			is3D: false,
		},
		{
			name: "3D polygon",
			polygon: [][][][]float64{
				{
					{{116.4, 39.9, 100.0}, {116.5, 39.9, 100.0}, {116.5, 39.8, 100.0}, {116.4, 39.9, 100.0}},
				},
			},
			srid: 4326,
			is3D: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := &types.MultiPolygonGeometry{
				Type:        "MultiPolygon",
				Coordinates: tt.polygon,
			}

			var encoded []byte
			var err error
			if tt.is3D {
				encoded, err = codec.EncodeMultiPolygonZ(original, tt.srid)
			} else {
				encoded, err = codec.EncodeMultiPolygon(original, tt.srid)
			}
			require.NoError(t, err)

			var decoded *types.MultiPolygonGeometry
			if tt.is3D {
				decoded, err = codec.DecodeMultiPolygonZ(encoded)
			} else {
				decoded, err = codec.DecodeMultiPolygon(encoded)
			}
			require.NoError(t, err)

			assert.Equal(t, len(tt.polygon), len(decoded.Coordinates))
			for i, expectedPolygon := range tt.polygon {
				assert.Equal(t, len(expectedPolygon), len(decoded.Coordinates[i]))
			}
		})
	}
}

func TestGaiaPolygonCodec_DecodeMultiPolygon_InvalidEndMarker(t *testing.T) {
	codec := NewGaiaPolygonCodec()

	points := [][2]float64{
		{116.4, 39.9},
		{116.5, 39.9},
		{116.5, 39.8},
		{116.4, 39.9},
	}
	mbr := CalculatePointsMBR(points)
	header := WriteGaiaHeader(4326, mbr, GeoTypeMultiPolygon)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteInt32(1) // 1 polygon
	writer.WriteByte(GaiaEntityMark) // entity marker
	writer.WriteInt32(3) // polygonGeoType for 2D
	writer.WriteInt32(1) // 1 ring
	writer.WriteInt32(4)
	for _, p := range points {
		writer.WriteFloat64(p[0])
		writer.WriteFloat64(p[1])
	}
	writer.WriteByte(0xFF) // Wrong end marker

	_, err := codec.DecodeMultiPolygon(writer.Bytes())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid end marker")
}
