package codec

import (
	"fmt"

	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// GaiaPointCodec provides encoding and decoding for GAIA Point geometries.
type GaiaPointCodec struct{}

// NewGaiaPointCodec creates a new point codec.
func NewGaiaPointCodec() *GaiaPointCodec {
	return &GaiaPointCodec{}
}

// DecodePoint decodes a GAIA Point BLOB into a PointGeometry.
func (c *GaiaPointCodec) DecodePoint(data []byte) (*types.PointGeometry, error) {
	if len(data) < GaiaHeaderLength+16+1 { // Header + 2 doubles (or 3) + end marker
		return nil, errors.FormatError(fmt.Sprintf("point data too short: %d bytes", len(data)))
	}

	header, err := ReadGaiaHeader(data)
	if err != nil {
		return nil, err
	}

	if header.GeoType != GeoTypePoint {
		return nil, errors.FormatError(fmt.Sprintf("expected geoType %d (Point), got %d", GeoTypePoint, header.GeoType))
	}

	reader := NewBinaryReader(data)
	reader.pos = GaiaHeaderLength

	// Read X coordinate
	x, err := reader.ReadFloat64()
	if err != nil {
		return nil, err
	}

	// Read Y coordinate
	y, err := reader.ReadFloat64()
	if err != nil {
		return nil, err
	}

	// Read end marker
	endMarker, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if endMarker != GaiaEndMarker {
		return nil, errors.FormatError(fmt.Sprintf("invalid end marker: expected 0x%02X, got 0x%02X", GaiaEndMarker, endMarker))
	}

	return &types.PointGeometry{
		Type:        "Point",
		Coordinates: []float64{x, y},
		SRID:        int(header.SRID),
		HasZValue:   false,
		BBox:        []float64{x, y, x, y},
		GeoType:     GeoTypePoint,
	}, nil
}

// DecodePointZ decodes a GAIA PointZ BLOB into a PointGeometry.
func (c *GaiaPointCodec) DecodePointZ(data []byte) (*types.PointGeometry, error) {
	if len(data) < GaiaHeaderLength+24+1 { // Header + 3 doubles + end marker
		return nil, errors.FormatError(fmt.Sprintf("point Z data too short: %d bytes", len(data)))
	}

	header, err := ReadGaiaHeader(data)
	if err != nil {
		return nil, err
	}

	if header.GeoType != GeoTypePointZ {
		return nil, errors.FormatError(fmt.Sprintf("expected geoType %d (PointZ), got %d", GeoTypePointZ, header.GeoType))
	}

	reader := NewBinaryReader(data)
	reader.pos = GaiaHeaderLength

	// Read X coordinate
	x, err := reader.ReadFloat64()
	if err != nil {
		return nil, err
	}

	// Read Y coordinate
	y, err := reader.ReadFloat64()
	if err != nil {
		return nil, err
	}

	// Read Z coordinate
	z, err := reader.ReadFloat64()
	if err != nil {
		return nil, err
	}

	// Read end marker
	endMarker, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if endMarker != GaiaEndMarker {
		return nil, errors.FormatError(fmt.Sprintf("invalid end marker: expected 0x%02X, got 0x%02X", GaiaEndMarker, endMarker))
	}

	return &types.PointGeometry{
		Type:        "Point",
		Coordinates: []float64{x, y, z},
		SRID:        int(header.SRID),
		HasZValue:   true,
		BBox:        []float64{x, y, x, y},
		GeoType:     GeoTypePointZ,
	}, nil
}

// EncodePoint encodes a PointGeometry into a GAIA Point BLOB.
func (c *GaiaPointCodec) EncodePoint(geometry *types.PointGeometry, srid int) ([]byte, error) {
	if len(geometry.Coordinates) < 2 {
		return nil, errors.FormatError("point must have at least 2 coordinates")
	}

	x := geometry.Coordinates[0]
	y := geometry.Coordinates[1]

	mbr := CalculatePointMBR(x, y)
	header := WriteGaiaHeader(int32(srid), mbr, GeoTypePoint)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteFloat64(x)
	writer.WriteFloat64(y)
	writer.WriteByte(GaiaEndMarker)

	return writer.Bytes(), nil
}

// EncodePointZ encodes a PointGeometry into a GAIA PointZ BLOB.
func (c *GaiaPointCodec) EncodePointZ(geometry *types.PointGeometry, srid int) ([]byte, error) {
	if len(geometry.Coordinates) < 3 {
		return nil, errors.FormatError("point Z must have 3 coordinates")
	}

	x := geometry.Coordinates[0]
	y := geometry.Coordinates[1]
	z := geometry.Coordinates[2]

	mbr := CalculatePointMBR(x, y)
	header := WriteGaiaHeader(int32(srid), mbr, GeoTypePointZ)

	writer := NewBinaryWriter()
	writer.WriteBytes(header)
	writer.WriteFloat64(x)
	writer.WriteFloat64(y)
	writer.WriteFloat64(z)
	writer.WriteByte(GaiaEndMarker)

	return writer.Bytes(), nil
}
