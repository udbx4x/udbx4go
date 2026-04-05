package types

// Feature represents a spatial feature with geometry and attributes.
type Feature struct {
	// ID is the unique identifier (maps to SmID)
	ID int
	// Geometry is the spatial geometry
	Geometry Geometry
	// Attributes is a map of field names to values
	Attributes map[string]interface{}
}

// GetAttribute returns an attribute value by name.
// Returns (nil, false) if the attribute doesn't exist.
func (f *Feature) GetAttribute(name string) (interface{}, bool) {
	if f.Attributes == nil {
		return nil, false
	}
	val, ok := f.Attributes[name]
	return val, ok
}

// SetAttribute sets an attribute value.
func (f *Feature) SetAttribute(name string, value interface{}) {
	if f.Attributes == nil {
		f.Attributes = make(map[string]interface{})
	}
	f.Attributes[name] = value
}

// TabularRecord represents a non-spatial record with only attributes.
type TabularRecord struct {
	// ID is the unique identifier (maps to SmID)
	ID int
	// Attributes is a map of field names to values
	Attributes map[string]interface{}
}

// GetAttribute returns an attribute value by name.
// Returns (nil, false) if the attribute doesn't exist.
func (r *TabularRecord) GetAttribute(name string) (interface{}, bool) {
	if r.Attributes == nil {
		return nil, false
	}
	val, ok := r.Attributes[name]
	return val, ok
}

// SetAttribute sets an attribute value.
func (r *TabularRecord) SetAttribute(name string, value interface{}) {
	if r.Attributes == nil {
		r.Attributes = make(map[string]interface{})
	}
	r.Attributes[name] = value
}
