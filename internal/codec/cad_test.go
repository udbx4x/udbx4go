package codec

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestCadGeometryCodec_EncodeDecodeEncode(t *testing.T) {
	fixtures := []struct {
		name     string
		geometry types.CadGeometry
	}{
		{
			name: "point",
			geometry: &types.CadPointGeometry{
				XCoord: 116.123,
				YCoord: 39.456,
				Style: &types.CadMarkerStyle{
					MarkerStyle:       1,
					MarkerSize:        20,
					MarkerAngle:       0,
					MarkerColor:       255,
					MarkerWidth:       20,
					MarkerHeight:      20,
					FillOpaqueRate:    100,
					FillGradientType:  0,
					FillAngle:         0,
					FillCenterOffsetX: 0,
					FillCenterOffsetY: 0,
					FillBackcolor:     16777215,
				},
			},
		},
		{
			name: "line",
			geometry: &types.CadLineGeometry{
				NumSub:         1,
				SubPointCounts: []int{3},
				Coordinates:    [][2]float64{{116.123, 39.456}, {116.5, 39.8}, {117, 40}},
				Style:          &types.CadLineStyle{LineStyle: 1, LineWidth: 1, LineColor: 65280},
			},
		},
		{
			name: "region",
			geometry: &types.CadRegionGeometry{
				NumSub:         1,
				SubPointCounts: []int{5},
				Coordinates:    [][2]float64{{116, 39.2}, {117, 39.2}, {117, 40}, {116, 40}, {116, 39.2}},
				Style: &types.CadFillStyle{
					LineStyle:         1,
					LineWidth:         1,
					LineColor:         0,
					FillStyle:         0,
					FillForecolor:     16711680,
					FillBackcolor:     16777215,
					FillOpaquerate:    100,
					FillGadientType:   0,
					FillAngle:         0,
					FillCenterOffsetX: 0,
					FillCenterOffsetY: 0,
				},
			},
		},
	}

	codec := NewCadGeometryCodec()
	for _, fixture := range fixtures {
		t.Run(fixture.name, func(t *testing.T) {
			encoded, err := codec.Encode(fixture.geometry)
			require.NoError(t, err)

			decoded, err := codec.Decode(encoded)
			require.NoError(t, err)
			assert.Equal(t, fixture.geometry.CadGeoType(), decoded.CadGeoType())

			reencoded, err := codec.Encode(decoded)
			require.NoError(t, err)
			assert.True(t, bytes.Equal(encoded, reencoded), "CAD 重新编码结果必须保持字节级一致")
		})
	}
}
