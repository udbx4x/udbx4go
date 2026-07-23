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
	Type        string        `json:"type"`
	Coordinates [][][]float64 `json:"coordinates"`
	SRID        int           `json:"srid,omitempty"`
	HasZValue   bool          `json:"hasZ,omitempty"`
	BBox        []float64     `json:"bbox,omitempty"`
	GeoType     int           `json:"geoType,omitempty"`
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
	Type        string          `json:"type"`
	Coordinates [][][][]float64 `json:"coordinates"`
	SRID        int             `json:"srid,omitempty"`
	HasZValue   bool            `json:"hasZ,omitempty"`
	BBox        []float64       `json:"bbox,omitempty"`
	GeoType     int             `json:"geoType,omitempty"`
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

// Color represents a UDBX Color value in ABGR byte order.
type Color struct {
	A int
	B int
	G int
	R int
}

// TextStyle represents the GeoText TextStyle payload.
type TextStyle struct {
	Color           *Color
	BackgroundColor *Color
	FixedSize       int
	Weight          int
	StyleFlag       int
	AlignFlag       int
	FontWidth       float64
	FontHeight      float64
	Anchor          []float64
	FaceName        string
}

// TextSubText represents one GeoText sub-text object.
type TextSubText struct {
	Text     string
	Anchor   []float64
	Rotation float64
}

// TextGeometry represents a UDBX GeoText geometry.
type TextGeometry struct {
	Type     string
	Text     string
	Anchor   []float64
	Rotation float64
	SRID     int
	BBox     []float64
	GeoType  int
	Style    *TextStyle
	SubTexts []*TextSubText
}

// GeometryType returns "Text".
func (g *TextGeometry) GeometryType() string { return "Text" }

// GetSRID returns the SRID.
func (g *TextGeometry) GetSRID() int { return g.SRID }

// HasZ returns false for current TextGeometry.
func (g *TextGeometry) HasZ() bool { return false }

// GetBBox returns the bounding box.
func (g *TextGeometry) GetBBox() []float64 { return g.BBox }

// CadStyle represents a CAD GeoHeader style payload.
type CadStyle interface {
	CadStyleKind() string
}

// CadMarkerStyle represents point marker style.
type CadMarkerStyle struct {
	MarkerStyle       int
	MarkerSize        int
	MarkerAngle       int
	MarkerColor       int
	MarkerWidth       int
	MarkerHeight      int
	FillOpaqueRate    int8
	FillGradientType  int8
	FillAngle         int16
	FillCenterOffsetX int16
	FillCenterOffsetY int16
	FillBackcolor     int
}

func (s *CadMarkerStyle) CadStyleKind() string { return "marker" }

// CadLineStyle represents line style.
type CadLineStyle struct {
	LineStyle int
	LineWidth int
	LineColor int
}

func (s *CadLineStyle) CadStyleKind() string { return "line" }

// CadFillStyle represents fill style.
type CadFillStyle struct {
	LineStyle         int
	LineWidth         int
	LineColor         int
	FillStyle         int
	FillForecolor     int
	FillBackcolor     int
	FillOpaquerate    int8
	FillGadientType   int8
	FillAngle         int16
	FillCenterOffsetX int16
	FillCenterOffsetY int16
}

func (s *CadFillStyle) CadStyleKind() string { return "fill" }

// CadGeometry represents a minimal CAD GeoHeader geometry.
type CadGeometry interface {
	Geometry
	CadGeoType() int
	CadStyle() CadStyle
}

// CadPointGeometry represents a CAD point geometry.
type CadPointGeometry struct {
	XCoord float64
	YCoord float64
	SRID   int
	BBox   []float64
	Style  CadStyle
}

func (g *CadPointGeometry) GeometryType() string { return "CadPoint" }
func (g *CadPointGeometry) GetSRID() int         { return g.SRID }
func (g *CadPointGeometry) HasZ() bool           { return false }
func (g *CadPointGeometry) GetBBox() []float64 {
	if len(g.BBox) >= 4 {
		return g.BBox
	}
	return []float64{g.XCoord, g.YCoord, g.XCoord, g.YCoord}
}
func (g *CadPointGeometry) CadGeoType() int    { return 1 }
func (g *CadPointGeometry) CadStyle() CadStyle { return g.Style }

// CadLineGeometry represents a CAD line geometry.
type CadLineGeometry struct {
	NumSub         int
	SubPointCounts []int
	Coordinates    [][2]float64
	SRID           int
	BBox           []float64
	Style          CadStyle
}

func (g *CadLineGeometry) GeometryType() string { return "CadLine" }
func (g *CadLineGeometry) GetSRID() int         { return g.SRID }
func (g *CadLineGeometry) HasZ() bool           { return false }
func (g *CadLineGeometry) GetBBox() []float64 {
	if len(g.BBox) >= 4 {
		return g.BBox
	}
	return cadBBox(g.Coordinates)
}
func (g *CadLineGeometry) CadGeoType() int    { return 3 }
func (g *CadLineGeometry) CadStyle() CadStyle { return g.Style }

// CadRegionGeometry represents a CAD region geometry.
type CadRegionGeometry struct {
	NumSub         int
	SubPointCounts []int
	Coordinates    [][2]float64
	SRID           int
	BBox           []float64
	Style          CadStyle
}

func (g *CadRegionGeometry) GeometryType() string { return "CadRegion" }
func (g *CadRegionGeometry) GetSRID() int         { return g.SRID }
func (g *CadRegionGeometry) HasZ() bool           { return false }
func (g *CadRegionGeometry) GetBBox() []float64 {
	if len(g.BBox) >= 4 {
		return g.BBox
	}
	return cadBBox(g.Coordinates)
}
func (g *CadRegionGeometry) CadGeoType() int    { return 5 }
func (g *CadRegionGeometry) CadStyle() CadStyle { return g.Style }

// CadTextGeometry represents a CAD text geometry stored in a CAD GeoHeader dataset.
type CadTextGeometry struct {
	Text         string
	Anchor       []float64
	Rotation     float64
	SRID         int
	BBox         []float64
	TextStyle    *TextStyle
	SubTexts     []*TextSubText
	CadStyleData CadStyle
}

func (g *CadTextGeometry) GeometryType() string { return "CadText" }
func (g *CadTextGeometry) GetSRID() int         { return g.SRID }
func (g *CadTextGeometry) HasZ() bool           { return false }
func (g *CadTextGeometry) GetBBox() []float64 {
	if len(g.BBox) >= 4 {
		return g.BBox
	}
	if len(g.Anchor) >= 2 {
		return []float64{g.Anchor[0], g.Anchor[1], g.Anchor[0], g.Anchor[1]}
	}
	return nil
}
func (g *CadTextGeometry) CadGeoType() int    { return 7 }
func (g *CadTextGeometry) CadStyle() CadStyle { return g.CadStyleData }

func cadBBox(coordinates [][2]float64) []float64 {
	if len(coordinates) == 0 {
		return nil
	}
	minX, minY := coordinates[0][0], coordinates[0][1]
	maxX, maxY := minX, minY
	for _, coordinate := range coordinates[1:] {
		if coordinate[0] < minX {
			minX = coordinate[0]
		}
		if coordinate[0] > maxX {
			maxX = coordinate[0]
		}
		if coordinate[1] < minY {
			minY = coordinate[1]
		}
		if coordinate[1] > maxY {
			maxY = coordinate[1]
		}
	}
	return []float64{minX, minY, maxX, maxY}
}
