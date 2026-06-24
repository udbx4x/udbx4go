package codec

import (
	"fmt"
	"math"
	"unicode/utf8"

	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

const geoTextType = 7

// GeoTextCodec decodes Text dataset SmGeometry GeoText BLOB values.
type GeoTextCodec struct{}

// NewGeoTextCodec creates a GeoText codec.
func NewGeoTextCodec() *GeoTextCodec {
	return &GeoTextCodec{}
}

// Decode decodes a minimal Text dataset GeoText BLOB.
func (c *GeoTextCodec) Decode(data []byte) (*types.TextGeometry, error) {
	reader := NewBinaryReader(data)

	geoType, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	if int(geoType) != geoTextType {
		return nil, errors.FormatError(fmt.Sprintf("unsupported GeoText geoType: %d", geoType))
	}

	styleSize, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	if styleSize != 0 {
		return nil, errors.FormatError(fmt.Sprintf("Text dataset GeoHeader.styleSize must be 0: %d", styleSize))
	}

	subCountValue, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	if subCountValue < 1 {
		return nil, errors.FormatError(fmt.Sprintf("GeoText subCount must be positive: %d", subCountValue))
	}

	style, err := decodeTextStyle(reader)
	if err != nil {
		return nil, err
	}

	subTexts := make([]*types.TextSubText, 0, int(subCountValue))
	text := ""
	for index := int32(0); index < subCountValue; index++ {
		subText, err := decodeSubText(reader)
		if err != nil {
			return nil, err
		}
		subTexts = append(subTexts, subText)
		text += subText.Text
	}

	if reader.Remaining() != 0 {
		return nil, errors.FormatError(fmt.Sprintf("GeoText has trailing bytes: %d", reader.Remaining()))
	}

	first := subTexts[0]
	return &types.TextGeometry{
		Type:     "Text",
		Text:     text,
		Anchor:   first.Anchor,
		Rotation: first.Rotation,
		GeoType:  geoTextType,
		Style:    style,
		SubTexts: subTexts,
	}, nil
}

// Encode encodes a Text geometry to the minimal GeoText BLOB layout used by
// udbx4spec compliance fixtures.
func (c *GeoTextCodec) Encode(geometry *types.TextGeometry) ([]byte, error) {
	if geometry == nil {
		return nil, errors.ConstraintError("Text geometry is required")
	}

	style := normalizeTextStyle(geometry)
	subTexts := normalizeSubTexts(geometry)

	writer := NewBinaryWriter()
	writer.WriteInt32(geoTextType)
	writer.WriteInt32(0)
	writer.WriteInt32(int32(len(subTexts)))
	writeTextStyle(writer, style)
	for _, subText := range subTexts {
		writeSubText(writer, subText)
	}
	return writer.Bytes(), nil
}

// EncodeTextIndexKey encodes the minimal SmIndexKey GAIA polygon envelope for
// a Text geometry.
func EncodeTextIndexKey(geometry *types.TextGeometry, srid int) ([]byte, error) {
	if geometry == nil {
		return nil, errors.ConstraintError("Text geometry is required")
	}
	if len(geometry.Anchor) < 2 {
		return nil, errors.ConstraintError("Text geometry anchor must contain x and y")
	}

	fontHeight := 0.406494140625
	if geometry.Style != nil && geometry.Style.FontHeight != 0 {
		fontHeight = geometry.Style.FontHeight
	}
	halfSize := math.Max(math.Abs(fontHeight), 0.01) / 2.0
	minX := geometry.Anchor[0] - halfSize
	minY := geometry.Anchor[1] - halfSize
	maxX := geometry.Anchor[0] + halfSize
	maxY := geometry.Anchor[1] + halfSize

	points := [][2]float64{
		{minX, minY},
		{maxX, minY},
		{maxX, maxY},
		{minX, maxY},
		{minX, minY},
	}

	writer := NewBinaryWriter()
	writer.WriteByte(GaiaHeaderStart)
	writer.WriteByte(GaiaByteOrder)
	writer.WriteInt32(int32(srid))
	writer.WriteFloat64(minX)
	writer.WriteFloat64(minY)
	writer.WriteFloat64(maxX)
	writer.WriteFloat64(maxY)
	writer.WriteByte(GaiaMarker)
	writer.WriteInt32(3)
	writer.WriteInt32(0)
	writer.WriteInt32(int32(len(points)))
	for _, point := range points {
		writer.WriteFloat64(point[0])
		writer.WriteFloat64(point[1])
	}
	writer.WriteByte(GaiaEndMarker)
	return writer.Bytes(), nil
}

func decodeTextStyle(reader *BinaryReader) (*types.TextStyle, error) {
	color, err := readTextColor(reader)
	if err != nil {
		return nil, err
	}
	fixedSize, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	weight, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	styleFlag, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	alignFlag, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	backgroundColor, err := readTextColor(reader)
	if err != nil {
		return nil, err
	}
	fontWidth, err := reader.ReadFloat64()
	if err != nil {
		return nil, err
	}
	fontHeight, err := reader.ReadFloat64()
	if err != nil {
		return nil, err
	}
	anchorX, err := reader.ReadFloat64()
	if err != nil {
		return nil, err
	}
	anchorY, err := reader.ReadFloat64()
	if err != nil {
		return nil, err
	}
	faceName, err := readTextString(reader)
	if err != nil {
		return nil, err
	}

	return &types.TextStyle{
		Color:           color,
		BackgroundColor: backgroundColor,
		FixedSize:       int(fixedSize),
		Weight:          int(weight),
		StyleFlag:       int(styleFlag),
		AlignFlag:       int(alignFlag),
		FontWidth:       fontWidth,
		FontHeight:      fontHeight,
		Anchor:          []float64{anchorX, anchorY},
		FaceName:        faceName,
	}, nil
}

func decodeSubText(reader *BinaryReader) (*types.TextSubText, error) {
	anchorX, err := reader.ReadFloat64()
	if err != nil {
		return nil, err
	}
	anchorY, err := reader.ReadFloat64()
	if err != nil {
		return nil, err
	}
	subAngle, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	reserved, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	if reserved != 0 {
		return nil, errors.FormatError(fmt.Sprintf("GeoSubText.reserved must be 0: %d", reserved))
	}
	text, err := readTextString(reader)
	if err != nil {
		return nil, err
	}
	return &types.TextSubText{
		Text:     text,
		Anchor:   []float64{anchorX, anchorY},
		Rotation: float64(subAngle) / 10,
	}, nil
}

func readTextColor(reader *BinaryReader) (*types.Color, error) {
	a, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	b, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	g, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	r, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	return &types.Color{A: int(a), B: int(b), G: int(g), R: int(r)}, nil
}

func readTextString(reader *BinaryReader) (string, error) {
	lengthValue, err := reader.ReadInt32()
	if err != nil {
		return "", err
	}
	if lengthValue < 0 || int(lengthValue) > reader.Remaining() {
		return "", errors.FormatError(fmt.Sprintf("invalid GeoText string byte length: %d", lengthValue))
	}
	bytes, err := reader.ReadBytes(int(lengthValue))
	if err != nil {
		return "", err
	}
	return decodeTextBytes(bytes), nil
}

func decodeTextBytes(bytes []byte) string {
	result := make([]rune, 0, len(bytes))
	for len(bytes) > 0 {
		value, size := utf8.DecodeRune(bytes)
		if value == utf8.RuneError && size == 1 {
			result = append(result, utf8.RuneError)
			bytes = bytes[1:]
			continue
		}
		result = append(result, value)
		bytes = bytes[size:]
	}
	return string(result)
}

func normalizeTextStyle(geometry *types.TextGeometry) *types.TextStyle {
	anchor := geometry.Anchor
	style := geometry.Style
	if style == nil {
		style = &types.TextStyle{}
	}

	color := style.Color
	if color == nil {
		color = &types.Color{A: 0, B: 0, G: 0, R: 255}
	}
	backgroundColor := style.BackgroundColor
	if backgroundColor == nil {
		backgroundColor = &types.Color{A: 255, B: 255, G: 255, R: 255}
	}
	styleAnchor := style.Anchor
	if len(styleAnchor) < 2 {
		styleAnchor = anchor
	}
	faceName := style.FaceName
	if faceName == "" {
		faceName = "宋体"
	}
	fontHeight := style.FontHeight
	if fontHeight == 0 {
		fontHeight = 0.406494140625
	}
	fixedSize := style.FixedSize
	if fixedSize == 0 {
		fixedSize = 10
	}
	weight := style.Weight
	if weight == 0 {
		weight = 64
	}
	styleFlag := style.StyleFlag
	if styleFlag == 0 {
		styleFlag = 6
	}

	return &types.TextStyle{
		Color:           color,
		BackgroundColor: backgroundColor,
		FixedSize:       fixedSize,
		Weight:          weight,
		StyleFlag:       styleFlag,
		AlignFlag:       style.AlignFlag,
		FontWidth:       style.FontWidth,
		FontHeight:      fontHeight,
		Anchor:          styleAnchor,
		FaceName:        faceName,
	}
}

func normalizeSubTexts(geometry *types.TextGeometry) []*types.TextSubText {
	if len(geometry.SubTexts) > 0 {
		normalized := make([]*types.TextSubText, 0, len(geometry.SubTexts))
		for _, subText := range geometry.SubTexts {
			anchor := subText.Anchor
			if len(anchor) < 2 {
				anchor = geometry.Anchor
			}
			normalized = append(normalized, &types.TextSubText{
				Text:     subText.Text,
				Anchor:   anchor,
				Rotation: subText.Rotation,
			})
		}
		return normalized
	}

	return []*types.TextSubText{{
		Text:     geometry.Text,
		Anchor:   geometry.Anchor,
		Rotation: geometry.Rotation,
	}}
}

func writeTextStyle(writer *BinaryWriter, style *types.TextStyle) {
	writeTextColor(writer, style.Color)
	writer.WriteByte(byte(style.FixedSize))
	writer.WriteByte(byte(style.Weight))
	writer.WriteByte(byte(style.StyleFlag))
	writer.WriteByte(byte(style.AlignFlag))
	writeTextColor(writer, style.BackgroundColor)
	writer.WriteFloat64(style.FontWidth)
	writer.WriteFloat64(style.FontHeight)
	writer.WriteFloat64(style.Anchor[0])
	writer.WriteFloat64(style.Anchor[1])
	writeTextString(writer, style.FaceName)
}

func writeSubText(writer *BinaryWriter, subText *types.TextSubText) {
	writer.WriteFloat64(subText.Anchor[0])
	writer.WriteFloat64(subText.Anchor[1])
	writer.WriteInt32(int32(math.Round(subText.Rotation * 10)))
	writer.WriteInt32(0)
	writeTextString(writer, subText.Text)
}

func writeTextColor(writer *BinaryWriter, color *types.Color) {
	writer.WriteByte(byte(color.A))
	writer.WriteByte(byte(color.B))
	writer.WriteByte(byte(color.G))
	writer.WriteByte(byte(color.R))
}

func writeTextString(writer *BinaryWriter, value string) {
	bytes := []byte(value)
	writer.WriteInt32(int32(len(bytes)))
	writer.WriteBytes(bytes)
}
