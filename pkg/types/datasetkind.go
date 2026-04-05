package types

// DatasetKind represents the type of dataset in UDBX.
// It corresponds to SmDatasetType in the SmRegister system table.
type DatasetKind int

const (
	DatasetKindTabular   DatasetKind = 0   // Tabular dataset (no geometry)
	DatasetKindPoint     DatasetKind = 1   // 2D Point dataset
	DatasetKindLine      DatasetKind = 3   // 2D Line dataset
	DatasetKindRegion    DatasetKind = 5   // 2D Region dataset
	DatasetKindText      DatasetKind = 7   // Text annotation dataset
	DatasetKindPointZ    DatasetKind = 101 // 3D Point dataset
	DatasetKindLineZ     DatasetKind = 103 // 3D Line dataset
	DatasetKindRegionZ   DatasetKind = 105 // 3D Region dataset
	DatasetKindCAD       DatasetKind = 149 // CAD dataset
)

// String returns the string representation of DatasetKind.
func (k DatasetKind) String() string {
	switch k {
	case DatasetKindTabular:
		return "tabular"
	case DatasetKindPoint:
		return "point"
	case DatasetKindLine:
		return "line"
	case DatasetKindRegion:
		return "region"
	case DatasetKindText:
		return "text"
	case DatasetKindPointZ:
		return "pointZ"
	case DatasetKindLineZ:
		return "lineZ"
	case DatasetKindRegionZ:
		return "regionZ"
	case DatasetKindCAD:
		return "cad"
	default:
		return "unknown"
	}
}

// FromDatasetKindString converts a string to DatasetKind.
func FromDatasetKindString(s string) (DatasetKind, bool) {
	switch s {
	case "tabular":
		return DatasetKindTabular, true
	case "point":
		return DatasetKindPoint, true
	case "line":
		return DatasetKindLine, true
	case "region":
		return DatasetKindRegion, true
	case "text":
		return DatasetKindText, true
	case "pointZ":
		return DatasetKindPointZ, true
	case "lineZ":
		return DatasetKindLineZ, true
	case "regionZ":
		return DatasetKindRegionZ, true
	case "cad":
		return DatasetKindCAD, true
	default:
		return DatasetKindTabular, false
	}
}

// IsSpatial returns true if the dataset kind has geometry.
func (k DatasetKind) IsSpatial() bool {
	switch k {
	case DatasetKindPoint, DatasetKindLine, DatasetKindRegion,
		DatasetKindPointZ, DatasetKindLineZ, DatasetKindRegionZ,
		DatasetKindText, DatasetKindCAD:
		return true
	default:
		return false
	}
}

// Is3D returns true if the dataset kind is 3D (Z-variant).
func (k DatasetKind) Is3D() bool {
	switch k {
	case DatasetKindPointZ, DatasetKindLineZ, DatasetKindRegionZ:
		return true
	default:
		return false
	}
}

// GeometryType returns the GAIA geoType for this dataset kind.
// Returns 0 for non-spatial datasets.
func (k DatasetKind) GeometryType() int {
	switch k {
	case DatasetKindPoint:
		return 1    // GAIAPoint
	case DatasetKindLine:
		return 5    // GAIAMultiLineString
	case DatasetKindRegion:
		return 6    // GAIAMultiPolygon
	case DatasetKindPointZ:
		return 1001 // GAIAPointZ
	case DatasetKindLineZ:
		return 1005 // GAIAMultiLineStringZ
	case DatasetKindRegionZ:
		return 1006 // GAIAMultiPolygonZ
	default:
		return 0
	}
}

// CoordDimension returns the coordinate dimension (2 or 3).
func (k DatasetKind) CoordDimension() int {
	if k.Is3D() {
		return 3
	}
	return 2
}
