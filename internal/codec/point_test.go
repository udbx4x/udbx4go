package codec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestGaiaPointCodec_DecodePoint(t *testing.T) {
	codec := NewGaiaPointCodec()

	// Create a valid GAIA Point BLOB
	mbr := CalculatePointMBR(116.4, 39.9)
	header := WriteGaiaHeader(4326, mbr, GeoTypePoint)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteFloat64(116.4)
	writer.WriteFloat64(39.9)
	writer.WriteByte(GaiaEndMarker)

	data := writer.Bytes()

	// Decode
	point, err := codec.DecodePoint(data)
	require.NoError(t, err)
	require.NotNil(t, point)

	assert.Equal(t, "Point", point.Type)
	assert.InDelta(t, 116.4, point.Coordinates[0], 0.0001)
	assert.InDelta(t, 39.9, point.Coordinates[1], 0.0001)
	assert.Equal(t, 4326, point.SRID)
	assert.False(t, point.HasZ())
}

func TestGaiaPointCodec_DecodePoint_InvalidData(t *testing.T) {
	codec := NewGaiaPointCodec()

	// Too short data
	_, err := codec.DecodePoint([]byte{0x00, 0x01})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestGaiaPointCodec_DecodePoint_WrongGeoType(t *testing.T) {
	codec := NewGaiaPointCodec()

	// Create a header with wrong geoType
	mbr := CalculatePointMBR(116.4, 39.9)
	header := WriteGaiaHeader(4326, mbr, GeoTypePointZ)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteFloat64(116.4)
	writer.WriteFloat64(39.9)
	writer.WriteByte(GaiaEndMarker)

	_, err := codec.DecodePoint(writer.Bytes())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected geoType")
}

func TestGaiaPointCodec_DecodePointZ(t *testing.T) {
	codec := NewGaiaPointCodec()

	// Create a valid GAIA PointZ BLOB
	mbr := CalculatePointMBR(116.4, 39.9)
	header := WriteGaiaHeader(4326, mbr, GeoTypePointZ)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteFloat64(116.4)
	writer.WriteFloat64(39.9)
	writer.WriteFloat64(100.0)
	writer.WriteByte(GaiaEndMarker)

	data := writer.Bytes()

	// Decode
	point, err := codec.DecodePointZ(data)
	require.NoError(t, err)
	require.NotNil(t, point)

	assert.Equal(t, "Point", point.Type)
	assert.InDelta(t, 116.4, point.Coordinates[0], 0.0001)
	assert.InDelta(t, 39.9, point.Coordinates[1], 0.0001)
	assert.InDelta(t, 100.0, point.Coordinates[2], 0.0001)
	assert.Equal(t, 4326, point.SRID)
	assert.True(t, point.HasZ())
}

func TestGaiaPointCodec_EncodePoint(t *testing.T) {
	codec := NewGaiaPointCodec()

	point := &types.PointGeometry{
		Type:        "Point",
		Coordinates: []float64{116.4, 39.9},
	}

	data, err := codec.EncodePoint(point, 4326)
	require.NoError(t, err)
	require.NotNil(t, data)

	// Decode back to verify
	decoded, err := codec.DecodePoint(data)
	require.NoError(t, err)

	assert.InDelta(t, 116.4, decoded.Coordinates[0], 0.0001)
	assert.InDelta(t, 39.9, decoded.Coordinates[1], 0.0001)
	assert.Equal(t, 4326, decoded.SRID)
}

func TestGaiaPointCodec_EncodePoint_NotEnoughCoordinates(t *testing.T) {
	codec := NewGaiaPointCodec()

	point := &types.PointGeometry{
		Type:        "Point",
		Coordinates: []float64{116.4}, // Only 1 coordinate
	}

	_, err := codec.EncodePoint(point, 4326)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 2 coordinates")
}

func TestGaiaPointCodec_EncodePointZ(t *testing.T) {
	codec := NewGaiaPointCodec()

	point := &types.PointGeometry{
		Type:        "Point",
		Coordinates: []float64{116.4, 39.9, 100.0},
	}

	data, err := codec.EncodePointZ(point, 4326)
	require.NoError(t, err)
	require.NotNil(t, data)

	// Decode back to verify
	decoded, err := codec.DecodePointZ(data)
	require.NoError(t, err)

	assert.InDelta(t, 116.4, decoded.Coordinates[0], 0.0001)
	assert.InDelta(t, 39.9, decoded.Coordinates[1], 0.0001)
	assert.InDelta(t, 100.0, decoded.Coordinates[2], 0.0001)
}

func TestGaiaPointCodec_EncodePointZ_NotEnoughCoordinates(t *testing.T) {
	codec := NewGaiaPointCodec()

	point := &types.PointGeometry{
		Type:        "Point",
		Coordinates: []float64{116.4, 39.9}, // Only 2 coordinates
	}

	_, err := codec.EncodePointZ(point, 4326)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "3 coordinates")
}

func TestGaiaPointCodec_RoundTrip(t *testing.T) {
	codec := NewGaiaPointCodec()

	tests := []struct {
		name string
		x    float64
		y    float64
		z    float64
		srid int
		is3D bool
	}{
		{"Beijing 2D", 116.4, 39.9, 0, 4326, false},
		{"Shanghai 2D", 121.5, 31.2, 0, 4326, false},
		{"Beijing 3D", 116.4, 39.9, 100.0, 4326, true},
		{"Custom SRID", 100.0, 50.0, 0, 3857, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var original *types.PointGeometry
			var encoded []byte
			var err error

			if tt.is3D {
				original = &types.PointGeometry{
					Type:        "Point",
					Coordinates: []float64{tt.x, tt.y, tt.z},
					SRID:        tt.srid,
				}
				encoded, err = codec.EncodePointZ(original, tt.srid)
			} else {
				original = &types.PointGeometry{
					Type:        "Point",
					Coordinates: []float64{tt.x, tt.y},
					SRID:        tt.srid,
				}
				encoded, err = codec.EncodePoint(original, tt.srid)
			}
			require.NoError(t, err)

			var decoded *types.PointGeometry
			if tt.is3D {
				decoded, err = codec.DecodePointZ(encoded)
			} else {
				decoded, err = codec.DecodePoint(encoded)
			}
			require.NoError(t, err)

			assert.InDelta(t, tt.x, decoded.Coordinates[0], 0.0001)
			assert.InDelta(t, tt.y, decoded.Coordinates[1], 0.0001)
			if tt.is3D {
				assert.InDelta(t, tt.z, decoded.Coordinates[2], 0.0001)
			}
			assert.Equal(t, tt.srid, decoded.SRID)
		})
	}
}
