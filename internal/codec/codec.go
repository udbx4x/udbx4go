package codec

import (
	"fmt"

	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// GaiaGeometryCodec provides encoding and decoding for all GAIA geometry types.
type GaiaGeometryCodec struct {
	pointCodec   *GaiaPointCodec
	lineCodec    *GaiaLineCodec
	polygonCodec *GaiaPolygonCodec
}

// NewGaiaGeometryCodec creates a new GAIA geometry codec.
func NewGaiaGeometryCodec() *GaiaGeometryCodec {
	return &GaiaGeometryCodec{
		pointCodec:   NewGaiaPointCodec(),
		lineCodec:    NewGaiaLineCodec(),
		polygonCodec: NewGaiaPolygonCodec(),
	}
}

// Decode decodes a GAIA BLOB into the appropriate geometry type.
func (c *GaiaGeometryCodec) Decode(data []byte) (types.Geometry, error) {
	if len(data) < GaiaHeaderLength {
		return nil, errors.FormatError(fmt.Sprintf("data too short for GAIA header: %d bytes", len(data)))
	}

	header, err := ReadGaiaHeader(data)
	if err != nil {
		return nil, err
	}

	switch header.GeoType {
	case GeoTypePoint:
		return c.pointCodec.DecodePoint(data)
	case GeoTypePointZ:
		return c.pointCodec.DecodePointZ(data)
	case GeoTypeMultiLineString:
		return c.lineCodec.DecodeMultiLineString(data)
	case GeoTypeMultiLineStringZ:
		return c.lineCodec.DecodeMultiLineStringZ(data)
	case GeoTypeMultiPolygon:
		return c.polygonCodec.DecodeMultiPolygon(data)
	case GeoTypeMultiPolygonZ:
		return c.polygonCodec.DecodeMultiPolygonZ(data)
	default:
		return nil, errors.UnsupportedError(fmt.Sprintf("unsupported GAIA geoType: %d", header.GeoType))
	}
}

// Encode encodes a geometry into a GAIA BLOB.
func (c *GaiaGeometryCodec) Encode(geometry types.Geometry, srid int) ([]byte, error) {
	switch g := geometry.(type) {
	case *types.PointGeometry:
		if g.HasZ() {
			return c.pointCodec.EncodePointZ(g, srid)
		}
		return c.pointCodec.EncodePoint(g, srid)
	case types.PointGeometry:
		if g.HasZ() {
			return c.pointCodec.EncodePointZ(&g, srid)
		}
		return c.pointCodec.EncodePoint(&g, srid)
	case *types.MultiLineStringGeometry:
		if g.HasZ() {
			return c.lineCodec.EncodeMultiLineStringZ(g, srid)
		}
		return c.lineCodec.EncodeMultiLineString(g, srid)
	case types.MultiLineStringGeometry:
		if g.HasZ() {
			return c.lineCodec.EncodeMultiLineStringZ(&g, srid)
		}
		return c.lineCodec.EncodeMultiLineString(&g, srid)
	case *types.MultiPolygonGeometry:
		if g.HasZ() {
			return c.polygonCodec.EncodeMultiPolygonZ(g, srid)
		}
		return c.polygonCodec.EncodeMultiPolygon(g, srid)
	case types.MultiPolygonGeometry:
		if g.HasZ() {
			return c.polygonCodec.EncodeMultiPolygonZ(&g, srid)
		}
		return c.polygonCodec.EncodeMultiPolygon(&g, srid)
	default:
		return nil, errors.UnsupportedError(fmt.Sprintf("unsupported geometry type: %T", geometry))
	}
}

// DecodePoint decodes a GAIA Point BLOB.
func (c *GaiaGeometryCodec) DecodePoint(data []byte) (*types.PointGeometry, error) {
	geom, err := c.Decode(data)
	if err != nil {
		return nil, err
	}

	point, ok := geom.(*types.PointGeometry)
	if !ok {
		return nil, errors.FormatError("decoded geometry is not a Point")
	}

	return point, nil
}

// DecodeMultiLineString decodes a GAIA MultiLineString BLOB.
func (c *GaiaGeometryCodec) DecodeMultiLineString(data []byte) (*types.MultiLineStringGeometry, error) {
	geom, err := c.Decode(data)
	if err != nil {
		return nil, err
	}

	line, ok := geom.(*types.MultiLineStringGeometry)
	if !ok {
		return nil, errors.FormatError("decoded geometry is not a MultiLineString")
	}

	return line, nil
}

// DecodeMultiPolygon decodes a GAIA MultiPolygon BLOB.
func (c *GaiaGeometryCodec) DecodeMultiPolygon(data []byte) (*types.MultiPolygonGeometry, error) {
	geom, err := c.Decode(data)
	if err != nil {
		return nil, err
	}

	polygon, ok := geom.(*types.MultiPolygonGeometry)
	if !ok {
		return nil, errors.FormatError("decoded geometry is not a MultiPolygon")
	}

	return polygon, nil
}
