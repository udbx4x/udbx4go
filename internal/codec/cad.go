package codec

import (
	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

const (
	cadGeoPoint  = 1
	cadGeoLine   = 3
	cadGeoRegion = 5
)

// CadGeometryCodec encodes and decodes the minimal CAD GeoHeader baseline.
type CadGeometryCodec struct{}

// NewCadGeometryCodec creates a CAD GeoHeader codec.
func NewCadGeometryCodec() *CadGeometryCodec {
	return &CadGeometryCodec{}
}

// Decode decodes a CAD GeoHeader geometry.
func (c *CadGeometryCodec) Decode(data []byte) (types.CadGeometry, error) {
	reader := NewBinaryReader(data)

	geoType, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	styleSize, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	styleBytes, err := reader.ReadBytes(int(styleSize))
	if err != nil {
		return nil, err
	}

	style, err := decodeCadStyle(styleBytes, int(geoType))
	if err != nil {
		return nil, err
	}

	switch int(geoType) {
	case cadGeoPoint:
		x, err := reader.ReadFloat64()
		if err != nil {
			return nil, err
		}
		y, err := reader.ReadFloat64()
		if err != nil {
			return nil, err
		}
		return &types.CadPointGeometry{XCoord: x, YCoord: y, Style: style}, nil
	case cadGeoLine:
		numSub, counts, coordinates, err := decodeCadLineOrRegion(reader)
		if err != nil {
			return nil, err
		}
		return &types.CadLineGeometry{NumSub: numSub, SubPointCounts: counts, Coordinates: coordinates, Style: style}, nil
	case cadGeoRegion:
		numSub, counts, coordinates, err := decodeCadLineOrRegion(reader)
		if err != nil {
			return nil, err
		}
		return &types.CadRegionGeometry{NumSub: numSub, SubPointCounts: counts, Coordinates: coordinates, Style: style}, nil
	default:
		return nil, errors.UnsupportedError("unsupported CAD geoType")
	}
}

// Encode encodes a CAD GeoHeader geometry.
func (c *CadGeometryCodec) Encode(geometry types.CadGeometry) ([]byte, error) {
	styleBytes, err := encodeCadStyle(geometry.CadStyle())
	if err != nil {
		return nil, err
	}

	writer := NewBinaryWriter()
	writer.WriteInt32(int32(geometry.CadGeoType()))
	writer.WriteInt32(int32(len(styleBytes)))
	writer.WriteBytes(styleBytes)

	switch g := geometry.(type) {
	case *types.CadPointGeometry:
		writer.WriteFloat64(g.XCoord)
		writer.WriteFloat64(g.YCoord)
	case *types.CadLineGeometry:
		encodeCadLineOrRegion(writer, g.NumSub, g.SubPointCounts, g.Coordinates)
	case *types.CadRegionGeometry:
		encodeCadLineOrRegion(writer, g.NumSub, g.SubPointCounts, g.Coordinates)
	default:
		return nil, errors.UnsupportedError("unsupported CAD geometry")
	}

	return writer.Bytes(), nil
}

func decodeCadStyle(data []byte, geoType int) (types.CadStyle, error) {
	if len(data) == 0 {
		return nil, nil
	}

	reader := NewBinaryReader(data)
	switch geoType {
	case cadGeoPoint:
		if _, err := reader.ReadInt32(); err != nil {
			return nil, err
		}
		markerStyle, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		markerSize, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		markerAngle, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		markerColor, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		markerWidth, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		markerHeight, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		reservedLength, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if _, err := reader.ReadBytes(int(reservedLength) + 4); err != nil {
			return nil, err
		}
		fillOpaqueRate, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		fillGradientType, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		fillAngle, err := reader.ReadInt16()
		if err != nil {
			return nil, err
		}
		fillCenterOffsetX, err := reader.ReadInt16()
		if err != nil {
			return nil, err
		}
		fillCenterOffsetY, err := reader.ReadInt16()
		if err != nil {
			return nil, err
		}
		fillBackcolor, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		return &types.CadMarkerStyle{
			MarkerStyle:       int(markerStyle),
			MarkerSize:        int(markerSize),
			MarkerAngle:       int(markerAngle),
			MarkerColor:       int(markerColor),
			MarkerWidth:       int(markerWidth),
			MarkerHeight:      int(markerHeight),
			FillOpaqueRate:    int8(fillOpaqueRate),
			FillGradientType:  int8(fillGradientType),
			FillAngle:         int16(fillAngle),
			FillCenterOffsetX: int16(fillCenterOffsetX),
			FillCenterOffsetY: int16(fillCenterOffsetY),
			FillBackcolor:     int(fillBackcolor),
		}, nil
	case cadGeoLine:
		lineStyle, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		lineWidth, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		lineColor, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		return &types.CadLineStyle{LineStyle: int(lineStyle), LineWidth: int(lineWidth), LineColor: int(lineColor)}, nil
	default:
		lineStyle, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		lineWidth, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		lineColor, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		fillStyle, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		fillForecolor, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		fillBackcolor, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		fillOpaquerate, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		fillGadientType, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		fillAngle, err := reader.ReadInt16()
		if err != nil {
			return nil, err
		}
		fillCenterOffsetX, err := reader.ReadInt16()
		if err != nil {
			return nil, err
		}
		fillCenterOffsetY, err := reader.ReadInt16()
		if err != nil {
			return nil, err
		}
		return &types.CadFillStyle{
			LineStyle:         int(lineStyle),
			LineWidth:         int(lineWidth),
			LineColor:         int(lineColor),
			FillStyle:         int(fillStyle),
			FillForecolor:     int(fillForecolor),
			FillBackcolor:     int(fillBackcolor),
			FillOpaquerate:    int8(fillOpaquerate),
			FillGadientType:   int8(fillGadientType),
			FillAngle:         int16(fillAngle),
			FillCenterOffsetX: int16(fillCenterOffsetX),
			FillCenterOffsetY: int16(fillCenterOffsetY),
		}, nil
	}
}

func encodeCadStyle(style types.CadStyle) ([]byte, error) {
	if style == nil {
		return []byte{}, nil
	}

	writer := NewBinaryWriter()
	switch s := style.(type) {
	case *types.CadMarkerStyle:
		writer.WriteInt32(41)
		writer.WriteInt32(int32(s.MarkerStyle))
		writer.WriteInt32(int32(s.MarkerSize))
		writer.WriteInt32(int32(s.MarkerAngle))
		writer.WriteInt32(int32(s.MarkerColor))
		writer.WriteInt32(int32(s.MarkerWidth))
		writer.WriteInt32(int32(s.MarkerHeight))
		writer.WriteByte(0)
		writer.WriteInt32(0)
		writer.WriteByte(byte(s.FillOpaqueRate))
		writer.WriteByte(byte(s.FillGradientType))
		writer.WriteInt16(int16(s.FillAngle))
		writer.WriteInt16(int16(s.FillCenterOffsetX))
		writer.WriteInt16(int16(s.FillCenterOffsetY))
		writer.WriteInt32(int32(s.FillBackcolor))
	case *types.CadLineStyle:
		writer.WriteInt32(int32(s.LineStyle))
		writer.WriteInt32(int32(s.LineWidth))
		writer.WriteInt32(int32(s.LineColor))
		writer.WriteByte(0)
		writer.WriteInt32(0)
	case *types.CadFillStyle:
		writer.WriteInt32(int32(s.LineStyle))
		writer.WriteInt32(int32(s.LineWidth))
		writer.WriteInt32(int32(s.LineColor))
		writer.WriteInt32(int32(s.FillStyle))
		writer.WriteInt32(int32(s.FillForecolor))
		writer.WriteInt32(int32(s.FillBackcolor))
		writer.WriteByte(byte(s.FillOpaquerate))
		writer.WriteByte(byte(s.FillGadientType))
		writer.WriteInt16(int16(s.FillAngle))
		writer.WriteInt16(int16(s.FillCenterOffsetX))
		writer.WriteInt16(int16(s.FillCenterOffsetY))
		writer.WriteByte(0)
		writer.WriteInt32(0)
		writer.WriteByte(0)
		writer.WriteInt32(0)
	default:
		return nil, errors.UnsupportedError("unsupported CAD style")
	}
	return writer.Bytes(), nil
}

func decodeCadLineOrRegion(reader *BinaryReader) (int, []int, [][2]float64, error) {
	numSubValue, err := reader.ReadInt32()
	if err != nil {
		return 0, nil, nil, err
	}
	numSub := int(numSubValue)
	counts := make([]int, numSub)
	total := 0
	for i := 0; i < numSub; i++ {
		count, err := reader.ReadInt32()
		if err != nil {
			return 0, nil, nil, err
		}
		counts[i] = int(count)
		total += int(count)
	}
	coordinates := make([][2]float64, total)
	for i := 0; i < total; i++ {
		x, err := reader.ReadFloat64()
		if err != nil {
			return 0, nil, nil, err
		}
		y, err := reader.ReadFloat64()
		if err != nil {
			return 0, nil, nil, err
		}
		coordinates[i] = [2]float64{x, y}
	}
	return numSub, counts, coordinates, nil
}

func encodeCadLineOrRegion(writer *BinaryWriter, numSub int, counts []int, coordinates [][2]float64) {
	writer.WriteInt32(int32(numSub))
	for _, count := range counts {
		writer.WriteInt32(int32(count))
	}
	for _, coordinate := range coordinates {
		writer.WriteFloat64(coordinate[0])
		writer.WriteFloat64(coordinate[1])
	}
}
