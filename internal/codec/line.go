package codec

import (
	"fmt"

	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// GaiaLineCodec provides encoding and decoding for GAIA MultiLineString geometries.
type GaiaLineCodec struct{}

// NewGaiaLineCodec creates a new line codec.
func NewGaiaLineCodec() *GaiaLineCodec {
	return &GaiaLineCodec{}
}

// DecodeMultiLineString decodes a GAIA MultiLineString BLOB.
func (c *GaiaLineCodec) DecodeMultiLineString(data []byte) (*types.MultiLineStringGeometry, error) {
	return c.decodeMultiLineStringInternal(data, false)
}

// DecodeMultiLineStringZ decodes a GAIA MultiLineStringZ BLOB.
func (c *GaiaLineCodec) DecodeMultiLineStringZ(data []byte) (*types.MultiLineStringGeometry, error) {
	return c.decodeMultiLineStringInternal(data, true)
}

func (c *GaiaLineCodec) decodeMultiLineStringInternal(data []byte, is3D bool) (*types.MultiLineStringGeometry, error) {
	if len(data) < GaiaHeaderLength+4+1 {
		return nil, errors.FormatError(fmt.Sprintf("multilinestring data too short: %d bytes", len(data)))
	}

	header, err := ReadGaiaHeader(data)
	if err != nil {
		return nil, err
	}

	expectedGeoType := GeoTypeMultiLineString
	if is3D {
		expectedGeoType = GeoTypeMultiLineStringZ
	}
	if header.GeoType != int32(expectedGeoType) {
		return nil, errors.FormatError(fmt.Sprintf("expected geoType %d, got %d", expectedGeoType, header.GeoType))
	}

	reader := NewBinaryReader(data)
	reader.pos = GaiaHeaderLength

	// Read number of linestrings
	numLineStrings, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}

	if numLineStrings < 0 || numLineStrings > 1000000 {
		return nil, errors.FormatError(fmt.Sprintf("invalid number of linestrings: %d", numLineStrings))
	}

	coordinates := make([][][]float64, numLineStrings)
	allPoints := make([][2]float64, 0)

	for i := int32(0); i < numLineStrings; i++ {
		// Read number of points in this linestring
		numPoints, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}

		if numPoints < 0 || numPoints > 10000000 {
			return nil, errors.FormatError(fmt.Sprintf("invalid number of points: %d", numPoints))
		}

		line := make([][]float64, numPoints)

		for j := int32(0); j < numPoints; j++ {
			x, err := reader.ReadFloat64()
			if err != nil {
				return nil, err
			}

			y, err := reader.ReadFloat64()
			if err != nil {
				return nil, err
			}

			allPoints = append(allPoints, [2]float64{x, y})

			if is3D {
				z, err := reader.ReadFloat64()
				if err != nil {
					return nil, err
				}
				line[j] = []float64{x, y, z}
			} else {
				line[j] = []float64{x, y}
			}
		}

		coordinates[i] = line
	}

	// Read end marker
	endMarker, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if endMarker != GaiaEndMarker {
		return nil, errors.FormatError(fmt.Sprintf("invalid end marker: expected 0x%02X, got 0x%02X", GaiaEndMarker, endMarker))
	}

	mbr := CalculatePointsMBR(allPoints)

	return &types.MultiLineStringGeometry{
		Type:        "MultiLineString",
		Coordinates: coordinates,
		SRID:        int(header.SRID),
		HasZValue:   is3D,
		BBox:        []float64{mbr[0], mbr[1], mbr[2], mbr[3]},
		GeoType:     int(header.GeoType),
	}, nil
}

// EncodeMultiLineString encodes a MultiLineStringGeometry into a GAIA BLOB.
func (c *GaiaLineCodec) EncodeMultiLineString(geometry *types.MultiLineStringGeometry, srid int) ([]byte, error) {
	return c.encodeMultiLineStringInternal(geometry, srid, false)
}

// EncodeMultiLineStringZ encodes a MultiLineStringGeometry into a GAIA PointZ BLOB.
func (c *GaiaLineCodec) EncodeMultiLineStringZ(geometry *types.MultiLineStringGeometry, srid int) ([]byte, error) {
	return c.encodeMultiLineStringInternal(geometry, srid, true)
}

func (c *GaiaLineCodec) encodeMultiLineStringInternal(geometry *types.MultiLineStringGeometry, srid int, is3D bool) ([]byte, error) {
	// Collect all points for MBR calculation
	allPoints := make([][2]float64, 0)
	for _, line := range geometry.Coordinates {
		for _, coord := range line {
			if len(coord) >= 2 {
				allPoints = append(allPoints, [2]float64{coord[0], coord[1]})
			}
		}
	}

	mbr := CalculatePointsMBR(allPoints)

	geoType := GeoTypeMultiLineString
	if is3D {
		geoType = GeoTypeMultiLineStringZ
	}

	header := WriteGaiaHeader(int32(srid), mbr, int32(geoType))

	writer := NewBinaryWriter()
	writer.WriteBytes(header)

	// Write number of linestrings
	writer.WriteInt32(int32(len(geometry.Coordinates)))

	for _, line := range geometry.Coordinates {
		// Write number of points
		writer.WriteInt32(int32(len(line)))

		for _, coord := range line {
			if len(coord) < 2 {
				return nil, errors.FormatError("coordinate must have at least 2 values")
			}

			writer.WriteFloat64(coord[0]) // X
			writer.WriteFloat64(coord[1]) // Y

			if is3D {
				if len(coord) < 3 {
					return nil, errors.FormatError("3D coordinate must have 3 values")
				}
				writer.WriteFloat64(coord[2]) // Z
			}
		}
	}

	writer.WriteByte(GaiaEndMarker)

	return writer.Bytes(), nil
}
