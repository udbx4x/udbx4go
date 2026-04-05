package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPointGeometry_GeometryType(t *testing.T) {
	g := PointGeometry{Type: "Point"}
	assert.Equal(t, "Point", g.GeometryType())
}

func TestPointGeometry_GetSRID(t *testing.T) {
	g := PointGeometry{SRID: 4326}
	assert.Equal(t, 4326, g.GetSRID())

	g2 := PointGeometry{}
	assert.Equal(t, 0, g2.GetSRID())
}

func TestPointGeometry_HasZ(t *testing.T) {
	// 2D point
	g2d := PointGeometry{Coordinates: []float64{116.4, 39.9}}
	assert.False(t, g2d.HasZ())

	// 3D point by coordinates
	g3d := PointGeometry{Coordinates: []float64{116.4, 39.9, 100.0}}
	assert.True(t, g3d.HasZ())

	// 2D point with HasZValue flag
	g2dFlag := PointGeometry{Coordinates: []float64{116.4, 39.9}, HasZValue: false}
	assert.False(t, g2dFlag.HasZ())

	// 3D point with HasZValue flag
	g3dFlag := PointGeometry{Coordinates: []float64{116.4, 39.9}, HasZValue: true}
	assert.True(t, g3dFlag.HasZ())
}

func TestPointGeometry_GetBBox(t *testing.T) {
	bbox := []float64{116.0, 39.0, 117.0, 40.0}
	g := PointGeometry{BBox: bbox}
	assert.Equal(t, bbox, g.GetBBox())

	g2 := PointGeometry{}
	assert.Nil(t, g2.GetBBox())
}

func TestPointGeometry_X(t *testing.T) {
	g := PointGeometry{Coordinates: []float64{116.4, 39.9}}
	assert.InDelta(t, 116.4, g.X(), 0.0001)
}

func TestPointGeometry_Y(t *testing.T) {
	g := PointGeometry{Coordinates: []float64{116.4, 39.9}}
	assert.InDelta(t, 39.9, g.Y(), 0.0001)
}

func TestPointGeometry_Z(t *testing.T) {
	// 3D point
	g3d := PointGeometry{Coordinates: []float64{116.4, 39.9, 100.0}}
	assert.InDelta(t, 100.0, g3d.Z(), 0.0001)

	// 2D point (Z should be 0)
	g2d := PointGeometry{Coordinates: []float64{116.4, 39.9}}
	assert.InDelta(t, 0.0, g2d.Z(), 0.0001)

	// Empty coordinates
	gEmpty := PointGeometry{Coordinates: []float64{}}
	assert.InDelta(t, 0.0, gEmpty.Z(), 0.0001)
}

func TestMultiLineStringGeometry_GeometryType(t *testing.T) {
	g := MultiLineStringGeometry{Type: "MultiLineString"}
	assert.Equal(t, "MultiLineString", g.GeometryType())
}

func TestMultiLineStringGeometry_GetSRID(t *testing.T) {
	g := MultiLineStringGeometry{SRID: 4326}
	assert.Equal(t, 4326, g.GetSRID())

	g2 := MultiLineStringGeometry{}
	assert.Equal(t, 0, g2.GetSRID())
}

func TestMultiLineStringGeometry_HasZ(t *testing.T) {
	// 2D line
	g2d := MultiLineStringGeometry{
		Coordinates: [][][]float64{
			{{116.4, 39.9}, {116.5, 39.8}},
		},
	}
	assert.False(t, g2d.HasZ())

	// 3D line by coordinates
	g3d := MultiLineStringGeometry{
		Coordinates: [][][]float64{
			{{116.4, 39.9, 100.0}, {116.5, 39.8, 100.0}},
		},
	}
	assert.True(t, g3d.HasZ())

	// With HasZValue flag
	gFlag := MultiLineStringGeometry{HasZValue: true}
	assert.True(t, gFlag.HasZ())

	// Empty coordinates
	gEmpty := MultiLineStringGeometry{Coordinates: [][][]float64{}}
	assert.False(t, gEmpty.HasZ())
}

func TestMultiLineStringGeometry_GetBBox(t *testing.T) {
	bbox := []float64{116.0, 39.0, 117.0, 40.0}
	g := MultiLineStringGeometry{BBox: bbox}
	assert.Equal(t, bbox, g.GetBBox())

	g2 := MultiLineStringGeometry{}
	assert.Nil(t, g2.GetBBox())
}

func TestMultiPolygonGeometry_GeometryType(t *testing.T) {
	g := MultiPolygonGeometry{Type: "MultiPolygon"}
	assert.Equal(t, "MultiPolygon", g.GeometryType())
}

func TestMultiPolygonGeometry_GetSRID(t *testing.T) {
	g := MultiPolygonGeometry{SRID: 4326}
	assert.Equal(t, 4326, g.GetSRID())

	g2 := MultiPolygonGeometry{}
	assert.Equal(t, 0, g2.GetSRID())
}

func TestMultiPolygonGeometry_HasZ(t *testing.T) {
	// 2D polygon
	g2d := MultiPolygonGeometry{
		Coordinates: [][][][]float64{
			{{{116.4, 39.9}, {116.5, 39.9}, {116.5, 39.8}, {116.4, 39.9}}},
		},
	}
	assert.False(t, g2d.HasZ())

	// 3D polygon by coordinates
	g3d := MultiPolygonGeometry{
		Coordinates: [][][][]float64{
			{{{116.4, 39.9, 100.0}, {116.5, 39.9, 100.0}, {116.5, 39.8, 100.0}, {116.4, 39.9, 100.0}}},
		},
	}
	assert.True(t, g3d.HasZ())

	// With HasZValue flag
	gFlag := MultiPolygonGeometry{HasZValue: true}
	assert.True(t, gFlag.HasZ())

	// Empty coordinates
	gEmpty := MultiPolygonGeometry{Coordinates: [][][][]float64{}}
	assert.False(t, gEmpty.HasZ())
}

func TestMultiPolygonGeometry_GetBBox(t *testing.T) {
	bbox := []float64{116.0, 39.0, 117.0, 40.0}
	g := MultiPolygonGeometry{BBox: bbox}
	assert.Equal(t, bbox, g.GetBBox())

	g2 := MultiPolygonGeometry{}
	assert.Nil(t, g2.GetBBox())
}
