// Package codec provides binary codecs for UDBX geometry formats.
//
// This package implements the GAIA (SpatiaLite) binary format for geometry
// encoding and decoding, as defined in the UDBX specification.
package codec

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/udbx4x/udbx4go/pkg/errors"
)

// GAIA constants.
const (
	GaiaHeaderStart  = 0x00
	GaiaByteOrder    = 0x01 // Little-endian
	GaiaMarker       = 0x7c
	GaiaEndMarker    = 0xFE
	GaiaHeaderLength = 43   // Bytes from start to geoType
)

// GAIA geoType constants.
const (
	GeoTypePoint              = 1
	GeoTypeMultiLineString    = 5
	GeoTypeMultiPolygon       = 6
	GeoTypePointZ             = 1001
	GeoTypeMultiLineStringZ   = 1005
	GeoTypeMultiPolygonZ      = 1006
)

// BinaryReader provides methods for reading binary data in little-endian format.
type BinaryReader struct {
	data []byte
	pos  int
}

// NewBinaryReader creates a new binary reader.
func NewBinaryReader(data []byte) *BinaryReader {
	return &BinaryReader{data: data, pos: 0}
}

// Remaining returns the number of bytes remaining.
func (r *BinaryReader) Remaining() int {
	return len(r.data) - r.pos
}

// ReadByte reads a single byte.
func (r *BinaryReader) ReadByte() (byte, error) {
	if r.pos >= len(r.data) {
		return 0, errors.FormatError("unexpected end of data")
	}
	b := r.data[r.pos]
	r.pos++
	return b, nil
}

// ReadInt32 reads a 32-bit integer (little-endian).
func (r *BinaryReader) ReadInt32() (int32, error) {
	if r.pos+4 > len(r.data) {
		return 0, errors.FormatError("unexpected end of data reading int32")
	}
	val := int32(binary.LittleEndian.Uint32(r.data[r.pos:]))
	r.pos += 4
	return val, nil
}

// ReadInt64 reads a 64-bit integer (little-endian).
func (r *BinaryReader) ReadInt64() (int64, error) {
	if r.pos+8 > len(r.data) {
		return 0, errors.FormatError("unexpected end of data reading int64")
	}
	val := int64(binary.LittleEndian.Uint64(r.data[r.pos:]))
	r.pos += 8
	return val, nil
}

// ReadFloat64 reads a 64-bit float (little-endian).
func (r *BinaryReader) ReadFloat64() (float64, error) {
	if r.pos+8 > len(r.data) {
		return 0, errors.FormatError("unexpected end of data reading float64")
	}
	val := math.Float64frombits(binary.LittleEndian.Uint64(r.data[r.pos:]))
	r.pos += 8
	return val, nil
}

// ReadBytes reads n bytes.
func (r *BinaryReader) ReadBytes(n int) ([]byte, error) {
	if r.pos+n > len(r.data) {
		return nil, errors.FormatError(fmt.Sprintf("unexpected end of data reading %d bytes", n))
	}
	data := make([]byte, n)
	copy(data, r.data[r.pos:r.pos+n])
	r.pos += n
	return data, nil
}

// Position returns the current position.
func (r *BinaryReader) Position() int {
	return r.pos
}

// BinaryWriter provides methods for writing binary data in little-endian format.
type BinaryWriter struct {
	data []byte
}

// NewBinaryWriter creates a new binary writer.
func NewBinaryWriter() *BinaryWriter {
	return &BinaryWriter{data: make([]byte, 0)}
}

// WriteByte writes a single byte.
func (w *BinaryWriter) WriteByte(b byte) {
	w.data = append(w.data, b)
}

// WriteInt32 writes a 32-bit integer (little-endian).
func (w *BinaryWriter) WriteInt32(val int32) {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(val))
	w.data = append(w.data, b...)
}

// WriteInt64 writes a 64-bit integer (little-endian).
func (w *BinaryWriter) WriteInt64(val int64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(val))
	w.data = append(w.data, b...)
}

// WriteFloat64 writes a 64-bit float (little-endian).
func (w *BinaryWriter) WriteFloat64(val float64) {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, math.Float64bits(val))
	w.data = append(w.data, b...)
}

// WriteBytes writes bytes.
func (w *BinaryWriter) WriteBytes(data []byte) {
	w.data = append(w.data, data...)
}

// Bytes returns the written bytes.
func (w *BinaryWriter) Bytes() []byte {
	return w.data
}

// Len returns the number of bytes written.
func (w *BinaryWriter) Len() int {
	return len(w.data)
}

// GaiaHeader represents the GAIA geometry header.
type GaiaHeader struct {
	StartByte  byte
	ByteOrder  byte
	SRID       int32
	MBR        [4]float64 // minX, minY, maxX, maxY
	Marker     byte
	GeoType    int32
}

// ReadGaiaHeader reads the GAIA header from binary data.
func ReadGaiaHeader(data []byte) (*GaiaHeader, error) {
	if len(data) < GaiaHeaderLength {
		return nil, errors.FormatError(fmt.Sprintf("GAIA header too short: got %d bytes, need %d", len(data), GaiaHeaderLength))
	}

	reader := NewBinaryReader(data)

	header := &GaiaHeader{}

	// Read start byte
	startByte, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if startByte != GaiaHeaderStart {
		return nil, errors.FormatError(fmt.Sprintf("invalid GAIA start byte: expected 0x%02X, got 0x%02X", GaiaHeaderStart, startByte))
	}
	header.StartByte = startByte

	// Read byte order
	byteOrder, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if byteOrder != GaiaByteOrder {
		return nil, errors.FormatError(fmt.Sprintf("invalid GAIA byte order: expected 0x%02X (little-endian), got 0x%02X", GaiaByteOrder, byteOrder))
	}
	header.ByteOrder = byteOrder

	// Read SRID
	srid, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	header.SRID = srid

	// Read MBR (4 doubles)
	for i := 0; i < 4; i++ {
		val, err := reader.ReadFloat64()
		if err != nil {
			return nil, err
		}
		header.MBR[i] = val
	}

	// Read marker
	marker, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if marker != GaiaMarker {
		return nil, errors.FormatError(fmt.Sprintf("invalid GAIA marker: expected 0x%02X, got 0x%02X", GaiaMarker, marker))
	}
	header.Marker = marker

	// Read geoType
	geoType, err := reader.ReadInt32()
	if err != nil {
		return nil, err
	}
	header.GeoType = geoType

	return header, nil
}

// WriteGaiaHeader writes the GAIA header.
func WriteGaiaHeader(srid int32, mbr [4]float64, geoType int32) []byte {
	writer := NewBinaryWriter()

	writer.WriteByte(GaiaHeaderStart)
	writer.WriteByte(GaiaByteOrder)
	writer.WriteInt32(srid)
	for _, v := range mbr {
		writer.WriteFloat64(v)
	}
	writer.WriteByte(GaiaMarker)
	writer.WriteInt32(geoType)

	return writer.Bytes()
}

// CalculatePointMBR calculates the MBR for a point.
func CalculatePointMBR(x, y float64) [4]float64 {
	return [4]float64{x, y, x, y}
}

// CalculatePointsMBR calculates the MBR for a slice of points.
func CalculatePointsMBR(points [][2]float64) [4]float64 {
	if len(points) == 0 {
		return [4]float64{0, 0, 0, 0}
	}

	minX, minY := points[0][0], points[0][1]
	maxX, maxY := minX, minY

	for _, p := range points[1:] {
		if p[0] < minX {
			minX = p[0]
		}
		if p[0] > maxX {
			maxX = p[0]
		}
		if p[1] < minY {
			minY = p[1]
		}
		if p[1] > maxY {
			maxY = p[1]
		}
	}

	return [4]float64{minX, minY, maxX, maxY}
}

// IsValidGeoType checks if a geoType is valid.
func IsValidGeoType(geoType int32) bool {
	switch geoType {
	case GeoTypePoint, GeoTypeMultiLineString, GeoTypeMultiPolygon,
		GeoTypePointZ, GeoTypeMultiLineStringZ, GeoTypeMultiPolygonZ:
		return true
	default:
		return false
	}
}

// GeoTypeToGeometryType converts a GAIA geoType to a geometry type string.
func GeoTypeToGeometryType(geoType int32) (string, bool) {
	switch geoType {
	case GeoTypePoint, GeoTypePointZ:
		return "Point", true
	case GeoTypeMultiLineString, GeoTypeMultiLineStringZ:
		return "MultiLineString", true
	case GeoTypeMultiPolygon, GeoTypeMultiPolygonZ:
		return "MultiPolygon", true
	default:
		return "", false
	}
}

// IsZGeoType returns true if the geoType indicates 3D coordinates.
func IsZGeoType(geoType int32) bool {
	switch geoType {
	case GeoTypePointZ, GeoTypeMultiLineStringZ, GeoTypeMultiPolygonZ:
		return true
	default:
		return false
	}
}
