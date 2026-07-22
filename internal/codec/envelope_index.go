package codec

import (
	"math"

	"github.com/udbx4x/udbx4go/pkg/errors"
)

// EncodeEnvelopeIndexKey encodes a bounding box as an SmIndexKey GAIA polygon.
func EncodeEnvelopeIndexKey(bbox []float64, srid int) ([]byte, error) {
	if len(bbox) != 4 {
		return nil, errors.ConstraintError("envelope bbox must contain minX, minY, maxX and maxY")
	}

	minX, minY, maxX, maxY := bbox[0], bbox[1], bbox[2], bbox[3]
	for _, value := range bbox {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, errors.ConstraintError("envelope bbox coordinates must be finite")
		}
	}
	if minX > maxX || minY > maxY {
		return nil, errors.ConstraintError("envelope bbox coordinates must be ordered")
	}

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
