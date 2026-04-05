package types

// Geometry is the interface for all geometry types.
type Geometry interface {
	// GeometryType returns the geometry type name (e.g., "Point", "MultiLineString", "MultiPolygon")
	GeometryType() string
	// GetSRID returns the SRID (coordinate reference system identifier), or 0 if not set
	GetSRID() int
	// HasZ returns true if the geometry has Z coordinates
	HasZ() bool
	// GetBBox returns the bounding box [minX, minY, maxX, maxY], or nil if not set
	GetBBox() []float64
}

// PointGeometry represents a GeoJSON-like Point geometry.
type PointGeometry struct {
	Type        string    `json:"type"`
	Coordinates []float64 `json:"coordinates"`
	SRID        int       `json:"srid,omitempty"`
	HasZValue   bool      `json:"hasZ,omitempty"`
	BBox        []float64 `json:"bbox,omitempty"`
	// GeoType is the GAIA geoType (1 for 2D, 1001 for 3D)
	GeoType int `json:"geoType,omitempty"`
}

// GeometryType returns "Point".
func (g PointGeometry) GeometryType() string {
	return "Point"
}

// GetSRID returns the SRID.
func (g PointGeometry) GetSRID() int {
	return g.SRID
}

// HasZ returns true if the point has Z coordinates.
func (g PointGeometry) HasZ() bool {
	if g.HasZValue {
		return true
	}
	return len(g.Coordinates) == 3
}

// GetBBox returns the bounding box.
func (g PointGeometry) GetBBox() []float64 {
	return g.BBox
}

// X returns the X coordinate.
func (g PointGeometry) X() float64 {
	if len(g.Coordinates) > 0 {
		return g.Coordinates[0]
	}
	return 0
}

// Y returns the Y coordinate.
func (g PointGeometry) Y() float64 {
	if len(g.Coordinates) > 1 {
		return g.Coordinates[1]
	}
	return 0
}

// Z returns the Z coordinate (0 if not present).
func (g PointGeometry) Z() float64 {
	if len(g.Coordinates) > 2 {
		return g.Coordinates[2]
	}
	return 0
}

// MultiLineStringGeometry represents a GeoJSON-like MultiLineString geometry.
type MultiLineStringGeometry struct {
	Type        string          `json:"type"`
	Coordinates [][][]float64   `json:"coordinates"`
	SRID        int             `json:"srid,omitempty"`
	HasZValue   bool            `json:"hasZ,omitempty"`
	BBox        []float64       `json:"bbox,omitempty"`
	GeoType     int             `json:"geoType,omitempty"`
}

// GeometryType returns "MultiLineString".
func (g MultiLineStringGeometry) GeometryType() string {
	return "MultiLineString"
}

// GetSRID returns the SRID.
func (g MultiLineStringGeometry) GetSRID() int {
	return g.SRID
}

// HasZ returns true if any line has Z coordinates.
func (g MultiLineStringGeometry) HasZ() bool {
	if g.HasZValue {
		return true
	}
	if len(g.Coordinates) > 0 && len(g.Coordinates[0]) > 0 {
		return len(g.Coordinates[0][0]) == 3
	}
	return false
}

// GetBBox returns the bounding box.
func (g MultiLineStringGeometry) GetBBox() []float64 {
	return g.BBox
}

// MultiPolygonGeometry represents a GeoJSON-like MultiPolygon geometry.
type MultiPolygonGeometry struct {
	Type        string            `json:"type"`
	Coordinates [][][][]float64   `json:"coordinates"`
	SRID        int               `json:"srid,omitempty"`
	HasZValue   bool              `json:"hasZ,omitempty"`
	BBox        []float64         `json:"bbox,omitempty"`
	GeoType     int               `json:"geoType,omitempty"`
}

// GeometryType returns "MultiPolygon".
func (g MultiPolygonGeometry) GeometryType() string {
	return "MultiPolygon"
}

// GetSRID returns the SRID.
func (g MultiPolygonGeometry) GetSRID() int {
	return g.SRID
}

// HasZ returns true if any polygon has Z coordinates.
func (g MultiPolygonGeometry) HasZ() bool {
	if g.HasZValue {
		return true
	}
	if len(g.Coordinates) > 0 && len(g.Coordinates[0]) > 0 && len(g.Coordinates[0][0]) > 0 {
		return len(g.Coordinates[0][0][0]) == 3
	}
	return false
}

// GetBBox returns the bounding box.
func (g MultiPolygonGeometry) GetBBox() []float64 {
	return g.BBox
}
