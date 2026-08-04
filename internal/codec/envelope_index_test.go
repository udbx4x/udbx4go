package codec

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	udbxerrors "github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestEncodeEnvelopeIndexKeyRoundTripsHeader(t *testing.T) {
	bbox := []float64{-2, 3, 10, 20}

	blob, err := EncodeEnvelopeIndexKey(bbox, 4326)

	require.NoError(t, err)
	require.Len(t, blob, 132)

	header, err := ReadGaiaHeader(blob)
	require.NoError(t, err)
	assert.Equal(t, int32(4326), header.SRID)
	assert.Equal(t, int32(3), header.GeoType)

	envelope, err := ReadGaiaEnvelope(blob)
	require.NoError(t, err)
	assert.Equal(t, types.BoundingBox{MinX: -2, MinY: 3, MaxX: 10, MaxY: 20}, envelope)

	reader := NewBinaryReader(blob)
	reader.pos = GaiaHeaderLength
	ringCount, err := reader.ReadInt32()
	require.NoError(t, err)
	assert.Equal(t, int32(0), ringCount)
	pointCount, err := reader.ReadInt32()
	require.NoError(t, err)
	assert.Equal(t, int32(5), pointCount)

	wantPoints := [][2]float64{{-2, 3}, {10, 3}, {10, 20}, {-2, 20}, {-2, 3}}
	for _, want := range wantPoints {
		x, readErr := reader.ReadFloat64()
		require.NoError(t, readErr)
		y, readErr := reader.ReadFloat64()
		require.NoError(t, readErr)
		assert.Equal(t, want, [2]float64{x, y})
	}
	endMarker, err := reader.ReadByte()
	require.NoError(t, err)
	assert.Equal(t, byte(GaiaEndMarker), endMarker)
	assert.Zero(t, reader.Remaining())
}

func TestEncodeEnvelopeIndexKeyAcceptsZeroArea(t *testing.T) {
	blob, err := EncodeEnvelopeIndexKey([]float64{116, 39, 116, 39}, 0)

	require.NoError(t, err)
	require.Len(t, blob, 132)
	envelope, err := ReadGaiaEnvelope(blob)
	require.NoError(t, err)
	assert.Equal(t, types.BoundingBox{MinX: 116, MinY: 39, MaxX: 116, MaxY: 39}, envelope)
}

func TestEncodeEnvelopeIndexKeyRejectsInvalidBounds(t *testing.T) {
	tests := []struct {
		name string
		bbox []float64
	}{
		{name: "nil", bbox: nil},
		{name: "too short", bbox: []float64{1, 2, 3}},
		{name: "too long", bbox: []float64{1, 2, 3, 4, 5}},
		{name: "inverted X", bbox: []float64{2, 0, 1, 1}},
		{name: "inverted Y", bbox: []float64{0, 2, 1, 1}},
		{name: "NaN", bbox: []float64{0, math.NaN(), 1, 1}},
		{name: "positive infinity", bbox: []float64{0, 0, math.Inf(1), 1}},
		{name: "negative infinity", bbox: []float64{math.Inf(-1), 0, 1, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := EncodeEnvelopeIndexKey(tt.bbox, 0)

			require.Error(t, err)
			assert.True(t, udbxerrors.IsConstraintViolation(err))
		})
	}
}

func TestEncodeTextIndexKeyPreservesEnvelopeCalculationAndLayout(t *testing.T) {
	tests := []struct {
		name       string
		geometry   *types.TextGeometry
		wantBounds []float64
	}{
		{
			name:       "default font height",
			geometry:   &types.TextGeometry{Anchor: []float64{10, 20}},
			wantBounds: []float64{9.7967529296875, 19.7967529296875, 10.2032470703125, 20.2032470703125},
		},
		{
			name:       "custom font height",
			geometry:   &types.TextGeometry{Anchor: []float64{10, 20}, Style: &types.TextStyle{FontHeight: 2}},
			wantBounds: []float64{9, 19, 11, 21},
		},
		{
			name:       "negative font height",
			geometry:   &types.TextGeometry{Anchor: []float64{10, 20}, Style: &types.TextStyle{FontHeight: -4}},
			wantBounds: []float64{8, 18, 12, 22},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			textBlob, err := EncodeTextIndexKey(tt.geometry, 3857)
			require.NoError(t, err)

			envelopeBlob, err := EncodeEnvelopeIndexKey(tt.wantBounds, 3857)
			require.NoError(t, err)
			assert.Equal(t, envelopeBlob, textBlob)

			envelope, err := ReadGaiaEnvelope(textBlob)
			require.NoError(t, err)
			assert.Equal(t, types.BoundingBox{
				MinX: tt.wantBounds[0],
				MinY: tt.wantBounds[1],
				MaxX: tt.wantBounds[2],
				MaxY: tt.wantBounds[3],
			}, envelope)
		})
	}
}
