package codec

import (
	"fmt"

	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// GaiaPolygonCodec provides encoding and decoding for GAIA MultiPolygon geometries.
type GaiaPolygonCodec struct{}

// NewGaiaPolygonCodec creates a new polygon codec.
func NewGaiaPolygonCodec() *GaiaPolygonCodec {
	return &GaiaPolygonCodec{}
}

// DecodeMultiPolygon decodes a GAIA MultiPolygon BLOB.
func (c *GaiaPolygonCodec) DecodeMultiPolygon(data []byte) (*types.MultiPolygonGeometry, error) {
	return c.decodeMultiPolygonInternal(data, false)
}

// DecodeMultiPolygonZ decodes a GAIA MultiPolygonZ BLOB.
func (c *GaiaPolygonCodec) DecodeMultiPolygonZ(data []byte) (*types.MultiPolygonGeometry, error) {
	return c.decodeMultiPolygonInternal(data, true)
}

func (c *GaiaPolygonCodec) decodeMultiPolygonInternal(data []byte, is3D bool) (*types.MultiPolygonGeometry, error) {
	if len(data) < GaiaHeaderLength+4+1 {
		return nil, errors.FormatError(fmt.Sprintf("multipolygon data too short: %d bytes", len(data)))
	}

	header, err := ReadGaiaHeader(data)
	if err != nil {
		return nil, err
	}

	expectedGeoType := GeoTypeMultiPolygon
	if is3D {
		expectedGeoType = GeoTypeMultiPolygonZ
	}
	if header.GeoType != int32(expectedGeoType) {
		return nil, errors.FormatError(fmt.Sprintf("expected geoType %d, got %d", expectedGeoType, header.GeoType))
	}

	reader := NewBinaryReader(data)
	reader.pos = GaiaHeaderLength

	// Read number of polygons
	numPolygons, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}

	if numPolygons < 0 || numPolygons > 1000000 {
		return nil, errors.FormatError(fmt.Sprintf("invalid number of polygons: %d", numPolygons))
	}

	coordinates := make([][][][]float64, numPolygons)
	allPoints := make([][2]float64, 0)

	for i := int32(0); i < numPolygons; i++ {
		// Read entity marker (0x69)
		entityMark, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		if entityMark != GaiaEntityMark {
			return nil, errors.FormatError(fmt.Sprintf("invalid Polygon entity mark: expected 0x%02X, got 0x%02X", GaiaEntityMark, entityMark))
		}

		// Read polygon geo type (3 for 2D, 1003 for 3D)
		polygonGeoType, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}
		expectedPolygonGeoType := int32(3)
		if is3D {
			expectedPolygonGeoType = 1003
		}
		if polygonGeoType != expectedPolygonGeoType {
			return nil, errors.FormatError(fmt.Sprintf("invalid Polygon geoType: expected %d, got %d", expectedPolygonGeoType, polygonGeoType))
		}

		// Read number of rings in this polygon
		numRings, err := reader.ReadInt32()
		if err != nil {
			return nil, err
		}

		if numRings < 0 || numRings > 100000 {
			return nil, errors.FormatError(fmt.Sprintf("invalid number of rings: %d", numRings))
		}

		polygon := make([][][]float64, numRings)

		for j := int32(0); j < numRings; j++ {
			// Read number of points in this ring
			numPoints, err := reader.ReadInt32()
			if err != nil {
				return nil, err
			}

			if numPoints < 0 || numPoints > 10000000 {
				return nil, errors.FormatError(fmt.Sprintf("invalid number of points: %d", numPoints))
			}

			ring := make([][]float64, numPoints)

			for k := int32(0); k < numPoints; k++ {
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
					ring[k] = []float64{x, y, z}
				} else {
					ring[k] = []float64{x, y}
				}
			}

			polygon[j] = ring
		}

		coordinates[i] = polygon
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

	return &types.MultiPolygonGeometry{
		Type:        "MultiPolygon",
		Coordinates: coordinates,
		SRID:        int(header.SRID),
		HasZValue:   is3D,
		BBox:        []float64{mbr[0], mbr[1], mbr[2], mbr[3]},
		GeoType:     int(header.GeoType),
	}, nil
}

// EncodeMultiPolygon encodes a MultiPolygonGeometry into a GAIA BLOB.
func (c *GaiaPolygonCodec) EncodeMultiPolygon(geometry *types.MultiPolygonGeometry, srid int) ([]byte, error) {
	return c.encodeMultiPolygonInternal(geometry, srid, false)
}

// EncodeMultiPolygonZ encodes a MultiPolygonGeometry into a GAIA PointZ BLOB.
func (c *GaiaPolygonCodec) EncodeMultiPolygonZ(geometry *types.MultiPolygonGeometry, srid int) ([]byte, error) {
	return c.encodeMultiPolygonInternal(geometry, srid, true)
}

func (c *GaiaPolygonCodec) encodeMultiPolygonInternal(geometry *types.MultiPolygonGeometry, srid int, is3D bool) ([]byte, error) {
	// Collect all points for MBR calculation
	allPoints := make([][2]float64, 0)
	for _, polygon := range geometry.Coordinates {
		for _, ring := range polygon {
			for _, coord := range ring {
				if len(coord) >= 2 {
					allPoints = append(allPoints, [2]float64{coord[0], coord[1]})
				}
			}
		}
	}

	mbr := CalculatePointsMBR(allPoints)

	geoType := GeoTypeMultiPolygon
	if is3D {
		geoType = GeoTypeMultiPolygonZ
	}

	header := WriteGaiaHeader(int32(srid), mbr, int32(geoType))

	writer := NewBinaryWriter()
	writer.WriteBytes(header)

	// Write number of polygons
	writer.WriteInt32(int32(len(geometry.Coordinates)))

	for _, polygon := range geometry.Coordinates {
		// Write entity marker
		writer.WriteByte(GaiaEntityMark)

		// Write polygon geo type (3 for 2D, 1003 for 3D)
		if is3D {
			writer.WriteInt32(1003)
		} else {
			writer.WriteInt32(3)
		}

		// Write number of rings
		writer.WriteInt32(int32(len(polygon)))

		for _, ring := range polygon {
			// Write number of points
			writer.WriteInt32(int32(len(ring)))

			for _, coord := range ring {
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
	}

	writer.WriteByte(GaiaEndMarker)

	return writer.Bytes(), nil
}
