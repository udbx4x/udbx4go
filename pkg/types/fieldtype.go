package types

// FieldType represents the type of a field in UDBX.
// It corresponds to SmFieldType in the SmFieldInfo system table.
type FieldType int

const (
	FieldTypeBoolean FieldType = 1   // Boolean (0/1)
	FieldTypeByte    FieldType = 2   // Single byte integer
	FieldTypeInt16   FieldType = 3   // 16-bit signed integer
	FieldTypeInt32   FieldType = 4   // 32-bit signed integer
	FieldTypeInt64   FieldType = 5   // 64-bit signed integer
	FieldTypeSingle  FieldType = 6   // Single precision float
	FieldTypeDouble  FieldType = 7   // Double precision float
	FieldTypeDate    FieldType = 8   // Date (TEXT storage)
	FieldTypeBinary  FieldType = 9   // Binary BLOB
	FieldTypeGeometry FieldType = 10 // Geometry BLOB (not SmGeometry system field)
	FieldTypeChar    FieldType = 11  // Fixed-length character
	FieldTypeNText   FieldType = 127 // Unicode long text
	FieldTypeText    FieldType = 128 // Long text
	FieldTypeTime    FieldType = 16  // Time (TEXT storage)
)

// String returns the string representation of FieldType.
func (t FieldType) String() string {
	switch t {
	case FieldTypeBoolean:
		return "boolean"
	case FieldTypeByte:
		return "byte"
	case FieldTypeInt16:
		return "int16"
	case FieldTypeInt32:
		return "int32"
	case FieldTypeInt64:
		return "int64"
	case FieldTypeSingle:
		return "single"
	case FieldTypeDouble:
		return "double"
	case FieldTypeDate:
		return "date"
	case FieldTypeBinary:
		return "binary"
	case FieldTypeGeometry:
		return "geometry"
	case FieldTypeChar:
		return "char"
	case FieldTypeNText:
		return "ntext"
	case FieldTypeText:
		return "text"
	case FieldTypeTime:
		return "time"
	default:
		return "unknown"
	}
}

// FromFieldTypeString converts a string to FieldType.
func FromFieldTypeString(s string) (FieldType, bool) {
	switch s {
	case "boolean":
		return FieldTypeBoolean, true
	case "byte":
		return FieldTypeByte, true
	case "int16":
		return FieldTypeInt16, true
	case "int32":
		return FieldTypeInt32, true
	case "int64":
		return FieldTypeInt64, true
	case "single":
		return FieldTypeSingle, true
	case "double":
		return FieldTypeDouble, true
	case "date":
		return FieldTypeDate, true
	case "binary":
		return FieldTypeBinary, true
	case "geometry":
		return FieldTypeGeometry, true
	case "char":
		return FieldTypeChar, true
	case "ntext":
		return FieldTypeNText, true
	case "text":
		return FieldTypeText, true
	case "time":
		return FieldTypeTime, true
	default:
		return FieldTypeText, false
	}
}

// SQLiteType returns the SQLite storage type for this field type.
func (t FieldType) SQLiteType() string {
	switch t {
	case FieldTypeBoolean, FieldTypeByte, FieldTypeInt16, FieldTypeInt32, FieldTypeInt64:
		return "INTEGER"
	case FieldTypeSingle, FieldTypeDouble:
		return "REAL"
	case FieldTypeBinary, FieldTypeGeometry:
		return "BLOB"
	default:
		return "TEXT"
	}
}

// GoType returns a string describing the Go type for this field type.
func (t FieldType) GoType() string {
	switch t {
	case FieldTypeBoolean:
		return "bool"
	case FieldTypeByte:
		return "int8"
	case FieldTypeInt16:
		return "int16"
	case FieldTypeInt32:
		return "int32"
	case FieldTypeInt64:
		return "int64"
	case FieldTypeSingle:
		return "float32"
	case FieldTypeDouble:
		return "float64"
	case FieldTypeBinary, FieldTypeGeometry:
		return "[]byte"
	default:
		return "string"
	}
}
