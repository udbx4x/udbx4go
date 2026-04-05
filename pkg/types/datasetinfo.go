package types

// DatasetInfo holds metadata about a dataset.
type DatasetInfo struct {
	// ID is the unique identifier in SmRegister
	ID int
	// Name is the dataset name
	Name string
	// TableName is the SQLite table name
	TableName string
	// Kind is the dataset type
	Kind DatasetKind
	// SRID is the coordinate reference system ID (nil if not set)
	SRID *int
	// ObjectCount is the number of records/features
	ObjectCount int
	// GeometryType is the GAIA geometry type (nil for non-spatial)
	GeometryType *int
}

// FieldInfo holds metadata about a field.
type FieldInfo struct {
	// Name is the field name
	Name string
	// FieldType is the field type
	FieldType FieldType
	// Alias is the field alias/caption (optional)
	Alias *string
	// Required indicates if the field is required
	Required bool
	// Nullable indicates if the field allows NULL values
	Nullable bool
	// DefaultValue is the default value (optional)
	DefaultValue interface{}
}

// QueryOptions provides options for querying features.
type QueryOptions struct {
	// IDs filters by specific feature IDs (nil = all)
	IDs []int
	// Limit limits the number of results (0 = unlimited)
	Limit int
	// Offset skips the first N results
	Offset int
}
