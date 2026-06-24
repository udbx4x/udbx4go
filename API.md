# API Documentation

Complete API reference for udbx4go.

Note: dataset concrete types currently come from the internal implementation package. External callers can use values returned by `DataSource` methods directly, but should not import internal packages or treat those concrete types as stable public API names.

[中文](./API.zh.md)

## Table of Contents

- [DataSource](#datasource)
- [Dataset Types](#dataset-types)
  - [Stable Dataset Semantics](#stable-dataset-semantics)
- [Geometry Types](#geometry-types)
- [Feature and Record](#feature-and-record)
- [Query Options](#query-options)
- [Error Handling](#error-handling)

## DataSource

The `DataSource` is the entry point for working with UDBX files.

### Functions

#### Open

```go
func Open(path string) (*DataSource, error)
```

Opens an existing UDBX file.

**Example:**
```go
ds, err := udbx4go.Open("data.udbx")
if err != nil {
    log.Fatal(err)
}
defer ds.Close()
```

#### Create

```go
func Create(path string) (*DataSource, error)
```

Creates a new UDBX file.

**Example:**
```go
ds, err := udbx4go.Create("newdata.udbx")
if err != nil {
    log.Fatal(err)
}
defer ds.Close()
```

### Methods

#### Close

```go
func (ds *DataSource) Close() error
```

Closes the data source and releases resources.

#### ListDatasets

```go
func (ds *DataSource) ListDatasets() ([]*DatasetInfo, error)
```

Returns a list of all datasets in the data source.

**Example:**
```go
datasets, err := ds.ListDatasets()
for _, info := range datasets {
    fmt.Printf("%s: %s\n", info.Name, info.Kind)
}
```

#### GetDataset

```go
func (ds *DataSource) GetDataset(name string) (dataset.Dataset, error)
```

Returns a dataset by name through the internal generic dataset interface. External callers normally prefer typed getters such as `GetPointDataset`.

#### GetTabularDataset

```go
func (ds *DataSource) GetTabularDataset(name string) (*TabularDataset, error)
```

Returns a tabular dataset by name.

#### GetPointDataset

```go
func (ds *DataSource) GetPointDataset(name string) (*PointDataset, error)
```

Returns a point dataset by name.

#### GetLineDataset

```go
func (ds *DataSource) GetLineDataset(name string) (*LineDataset, error)
```

Returns a line dataset by name.

#### GetRegionDataset

```go
func (ds *DataSource) GetRegionDataset(name string) (*RegionDataset, error)
```

Returns a region dataset by name.

### Dataset Creation

#### CreateTabularDataset

```go
func (ds *DataSource) CreateTabularDataset(
    name string,
    fields []*FieldInfo,
) (*TabularDataset, error)
```

Creates a new tabular (non-spatial) dataset.

**Example:**
```go
fields := []*udbx4go.FieldInfo{
    {Name: "code", FieldType: udbx4go.FieldTypeText, Required: true},
    {Name: "value", FieldType: udbx4go.FieldTypeDouble, Nullable: true},
}

tabularDS, err := ds.CreateTabularDataset("attributes", fields)
```

#### CreatePointDataset

```go
func (ds *DataSource) CreatePointDataset(
    name string,
    srid int,
    fields []*FieldInfo,
) (*PointDataset, error)
```

Creates a new 2D point dataset.

**Example:**
```go
fields := []*udbx4go.FieldInfo{
    {Name: "name", FieldType: udbx4go.FieldTypeText},
    {Name: "population", FieldType: udbx4go.FieldTypeInt32},
}

pointDS, err := ds.CreatePointDataset("cities", 4326, fields)
```

#### CreateLineDataset

```go
func (ds *DataSource) CreateLineDataset(
    name string,
    srid int,
    fields []*FieldInfo,
) (*LineDataset, error)
```

Creates a new 2D line dataset.

#### CreateRegionDataset

```go
func (ds *DataSource) CreateRegionDataset(
    name string,
    srid int,
    fields []*FieldInfo,
) (*RegionDataset, error)
```

Creates a new 2D region dataset.

#### CreatePointZDataset

```go
func (ds *DataSource) CreatePointZDataset(
    name string,
    srid int,
    fields []*FieldInfo,
) (*PointZDataset, error)
```

Creates a new 3D point dataset.

#### CreateLineZDataset

```go
func (ds *DataSource) CreateLineZDataset(
    name string,
    srid int,
    fields []*FieldInfo,
) (*LineZDataset, error)
```

Creates a new 3D line dataset.

#### CreateRegionZDataset

```go
func (ds *DataSource) CreateRegionZDataset(
    name string,
    srid int,
    fields []*FieldInfo,
) (*RegionZDataset, error)
```

Creates a new 3D region dataset.

## Dataset Types

### Stable Dataset Semantics

All dataset APIs follow `udbx4spec/docs/08-api-stable-surface.md`:

- `GetByID(id)` returns `nil, err` when the object is missing, and `udbx4go.IsNotFound(err)` must be true.
- `List(opts)` returns rows ordered by `SmID` ascending by default; `QueryOptions.IDs` filters the result set but does not change ordering.
- `Count()` reads the physical table row count, not cached `SmRegister.SmObjectCount`.
- `Update(id, ...)` and `Delete(id)` return a not found error when the target object is missing.
- Unknown update fields return a not found or constraint error; they must not be silently ignored.

### TabularDataset

Non-spatial dataset for attribute-only data.

#### Methods

##### GetByID

```go
func (d *TabularDataset) GetByID(id int) (*TabularRecord, error)
```

Returns a record by ID. Missing records return `nil, err` with `udbx4go.IsNotFound(err) == true`.

##### List

```go
func (d *TabularDataset) List(opts *QueryOptions) ([]*TabularRecord, error)
```

Returns a list of records ordered by `SmID` ascending by default.

##### Insert

```go
func (d *TabularDataset) Insert(record *TabularRecord) error
```

Inserts a new record.

##### InsertMany

```go
func (d *TabularDataset) InsertMany(records []*TabularRecord) error
```

Inserts multiple records in a transaction.

##### Update

```go
func (d *TabularDataset) Update(id int, attributes map[string]interface{}) error
```

Updates a record's attributes. Missing records return a not found error.

##### Delete

```go
func (d *TabularDataset) Delete(id int) error
```

Deletes a record by ID. Missing records return a not found error.

### PointDataset

2D point dataset.

#### Methods

##### GetByID

```go
func (d *PointDataset) GetByID(id int) (*Feature, error)
```

Returns a feature by ID. Missing features return `nil, err` with `udbx4go.IsNotFound(err) == true`.

##### List

```go
func (d *PointDataset) List(opts *QueryOptions) ([]*Feature, error)
```

Returns a list of features ordered by `SmID` ascending by default.

##### Insert

```go
func (d *PointDataset) Insert(feature *Feature) error
```

Inserts a new point feature.

**Example:**
```go
feature := &udbx4go.Feature{
    ID: 1,
    Geometry: &udbx4go.PointGeometry{
        Type:        "Point",
        Coordinates: []float64{116.4, 39.9},
    },
    Attributes: map[string]interface{}{
        "name": "Beijing",
    },
}
err = pointDS.Insert(feature)
```

##### InsertMany

```go
func (d *PointDataset) InsertMany(features []*Feature) error
```

Inserts multiple features in a transaction.

##### Update

```go
func (d *PointDataset) Update(id int, changes *FeatureChanges) error
```

Updates a feature using the public `udbx4go.FeatureChanges` type.

Missing features return a not found error.

**Example:**
```go
err = pointDS.Update(1, &udbx4go.FeatureChanges{
    Geometry: &udbx4go.PointGeometry{
        Type:        "Point",
        Coordinates: []float64{121.5, 31.2},
    },
    Attributes: map[string]interface{}{
        "population": 26320000,
    },
})
```

##### Delete

```go
func (d *PointDataset) Delete(id int) error
```

Deletes a feature by ID. Missing features return a not found error.

### LineDataset

2D line (MultiLineString) dataset. Same methods as PointDataset but expects MultiLineStringGeometry.

### RegionDataset

2D region (MultiPolygon) dataset. Same methods as PointDataset but expects MultiPolygonGeometry.

### PointZDataset, LineZDataset, RegionZDataset

3D variants that support Z coordinates.

## Geometry Types

### PointGeometry

```go
type PointGeometry struct {
    Type        string
    Coordinates []float64  // [x, y] for 2D, [x, y, z] for 3D
    SRID        int        // Optional SRID, 0 if not set
    HasZValue   bool       // Optional explicit Z flag
    BBox        []float64  // Optional [minX, minY, maxX, maxY]
    GeoType     int        // Optional GAIA geoType
}
```

**Methods:**
- `GeometryType() string` - Returns "Point"
- `GetSRID() int` - Returns SRID or 0
- `HasZ() bool` - Returns true if has Z coordinate
- `GetBBox() []float64` - Returns bounding box
- `X() float64` - Returns X coordinate
- `Y() float64` - Returns Y coordinate
- `Z() float64` - Returns Z coordinate (0 if 2D)

### MultiLineStringGeometry

```go
type MultiLineStringGeometry struct {
    Type        string
    Coordinates [][][]float64  // Array of line strings
    SRID        int
    HasZValue   bool
    BBox        []float64
    GeoType     int
}
```

**Methods:**
- `GeometryType() string` - Returns "MultiLineString"
- `GetSRID() int`
- `HasZ() bool`
- `GetBBox() []float64`

### MultiPolygonGeometry

```go
type MultiPolygonGeometry struct {
    Type        string
    Coordinates [][][][]float64  // Array of polygons, each with rings
    SRID        int
    HasZValue   bool
    BBox        []float64
    GeoType     int
}
```

**Methods:**
- `GeometryType() string` - Returns "MultiPolygon"
- `GetSRID() int`
- `HasZ() bool`
- `GetBBox() []float64`

## Feature and Record

### Feature

Spatial feature with geometry and attributes.

```go
type Feature struct {
    ID         int
    Geometry   Geometry
    Attributes map[string]interface{}
}
```

### TabularRecord

Non-spatial record with attributes only.

```go
type TabularRecord struct {
    ID         int
    Attributes map[string]interface{}
}
```

### FeatureChanges

Public change object for vector feature updates.

```go
type FeatureChanges struct {
    Geometry   Geometry
    Attributes map[string]interface{}
}
```

Use `udbx4go.FeatureChanges` with vector dataset `Update` methods. `Geometry` and `Attributes` are both optional; when both are empty, implementations may treat the call as a no-op.

## Query Options

```go
type QueryOptions struct {
    IDs    []int  // Filter by specific IDs
    Limit  int    // Maximum number of results
    Offset int    // Number of results to skip (requires Limit in SQLite)
}
```

**Examples:**

```go
// Get first 10 features
opts := &udbx4go.QueryOptions{Limit: 10}
features, err := dataset.List(opts)

// Get features with IDs 1, 3, 5
opts := &udbx4go.QueryOptions{IDs: []int{1, 3, 5}}
features, err := dataset.List(opts)

// Paginate (skip first 20, get next 10)
opts := &udbx4go.QueryOptions{Limit: 10, Offset: 20}
features, err := dataset.List(opts)
```

## Error Handling

### Error Types

All errors implement the `UdbxError` interface:

```go
type UdbxError interface {
    error
    Code() ErrorCode
}
```

### Error Categories

| Function | Description |
|----------|-------------|
| `IsFormatError(err error)` | Invalid UDBX format |
| `IsNotFound(err error)` | Dataset or feature not found |
| `IsUnsupported(err error)` | Unsupported operation |
| `IsConstraintViolation(err error)` | Data constraint violation |
| `IsIOError(err error)` | File I/O error |

### Sentinel Errors

| Error | Description |
|-------|-------------|
| `ErrNotFound` | Not found error |
| `ErrFormat` | Format error |
| `ErrUnsupported` | Unsupported error |
| `ErrConstraint` | Constraint error |
| `ErrIO` | I/O error |

### Examples

```go
// Check error type
feature, err := dataset.GetByID(999)
if err != nil {
    if udbx4go.IsNotFound(err) {
        fmt.Println("Feature not found")
    } else {
        log.Fatal(err)
    }
}

// Use with errors.Is
if errors.Is(err, udbx4go.ErrNotFound) {
    // Handle not found
}
```
