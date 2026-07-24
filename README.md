# udbx4go

[![Go Reference](https://pkg.go.dev/badge/github.com/udbx4x/udbx4go.svg)](https://pkg.go.dev/github.com/udbx4x/udbx4go)
[![Go Report Card](https://goreportcard.com/badge/github.com/udbx4x/udbx4go)](https://goreportcard.com/report/github.com/udbx4x/udbx4go)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](./)
[![Coverage](https://img.shields.io/badge/coverage-76.7%25-yellowgreen)](./)

A Go implementation of the UDBX (Universal Spatial Database Extension) reader/writer library. UDBX is a spatial data format based on SQLite, supporting tabular datasets and vector datasets such as Point, Line, Region, PointZ, LineZ, RegionZ, Text, and CAD.

[中文](./README.zh.md)

## Features

- ✅ Core UDBX read/write support, including Text / GeoText and CAD minimal GeoHeader baselines.
- ✅ Dataset types: Point, Line, Region, PointZ, LineZ, RegionZ, Tabular, Text, CAD
- ✅ `TextDataset` supports minimal GeoText read/write CRUD and `CadDataset` supports minimal GeoHeader `GeoPoint` / `GeoLine` / `GeoRegion`.
- ✅ Context-aware viewport MBR queries for Point, Line, Region, their Z variants, Text, and CAD, with verified RTree and envelope-cache strategies.
- ✅ 14 field types with proper type mapping
- ✅ GeoJSON-like geometry model
- ✅ Streaming and batch operations
- ✅ Cross-language compatibility (udbx4j, udbx4ts)
- ✅ Comprehensive error handling
- ✅ TDD development with 76%+ test coverage
- ✅ GUI viewer for visualizing UDBX files (Wails-based, React + TypeScript frontend)

## Installation

```bash
go get github.com/udbx4x/udbx4go
```

**Note**: This package requires CGO because it uses `github.com/mattn/go-sqlite3`. Make sure you have a C compiler installed.

## Quick Start

### Opening an Existing UDBX File

```go
package main

import (
    "log"
    "github.com/udbx4x/udbx4go"
)

func main() {
    // Open an existing UDBX file
    ds, err := udbx4go.Open("data.udbx")
    if err != nil {
        log.Fatal(err)
    }
    defer ds.Close()

    // List all datasets
    datasets, err := ds.ListDatasets()
    if err != nil {
        log.Fatal(err)
    }
    for _, info := range datasets {
        log.Printf("Dataset: %s (kind: %s)", info.Name, info.Kind)
    }

    // Get a point dataset
    pointDataset, err := ds.GetPointDataset("cities")
    if err != nil {
        log.Fatal(err)
    }

    // Query features
    features, err := pointDataset.List(&udbx4go.QueryOptions{Limit: 10})
    if err != nil {
        log.Fatal(err)
    }
    for _, f := range features {
        log.Printf("Feature %d: %v", f.ID, f.Attributes["name"])
    }
}
```

### Viewport Spatial Query

`DataSource.QuerySpatial` returns a stable, bounded set of features for a map viewport:

```go
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()

result, err := ds.QuerySpatial(ctx, "weibo", udbx4go.SpatialQueryOptions{
    Bounds: udbx4go.BoundingBox{MinX: 113.5, MinY: 34.5, MaxX: 114.0, MaxY: 35.0},
    Limit: 1000,
    RequiredIDs: []int{12345},
})
```

MBR intersection uses closed intervals. Every ordinary feature intersects the requested bounds MBR. `HasMore` applies only to ordinary viewport matches; required IDs are deduplicated, do not consume `Limit`, and are the only features that may remain outside the viewport. A successful SDK result uses only `rtree` or `envelope_cache`, and `SpatialQueryResult` has no `DegradedReason` field. Envelope caches live only for the open `DataSource` and are released by `Close`.

The current defaults of 32 MiB per dataset and 64 MiB per `DataSource` are measured cache resource policies. They charge a stable-RSS model of roughly 4 MiB fixed per dataset plus 80 bytes per capacity entry, not an object-count or UDBX format limit. If a complete cache exceeds that policy, the SDK returns an `envelope_cache_budget_exceeded` error. Text and CAD use `SmIndexKey` for envelope candidate filtering and decode matched business objects from `SmGeometry`. This requires the envelope column and valid spatial metadata: legacy CAD datasets without `SmIndexKey` or a valid `geometry_columns` registration report `spatial_index_unavailable` instead of succeeding through a non-spatial path. When no viewport is supplied, the Viewer creates its private ID-ordered `bounded_sample` initial preview through `ListContext`; `QueriedBounds` and `degradedReason` remain unset, and the declared extent is only an auto-fit hint. Sampling diagnostics use the bounded read's actual `HasMore` state and vertex-budget truncation, not `SmObjectCount`. When a viewport is supplied, the Viewer calls `QuerySpatial`: normal results use `rtree` or `envelope_cache`, while only `envelope_cache_budget_exceeded` or `spatial_index_unavailable` triggers a private bounded fallback carrying that `degradedReason`. `bounded_sample` and `degradedReason` are Viewer DTO concepts, not SDK success strategies or `SpatialQueryResult` fields. See [API.md](./API.md#viewport-spatial-queries) for the runnable program, all six reason codes, and `ListContext` cancellation semantics.

## GUI Viewer

udbx4go includes a graphical viewer application for visualizing UDBX files. Built with [Wails](https://wails.io/) v2 (Go backend + React/TypeScript frontend).

### Features

- Open and browse UDBX files
- Display dataset list with type icons
- View data records in paginated table with MUI X-DataGrid
- Column sorting, resizing, and reordering
- Support implemented dataset types: Point, Line, Region, PointZ, LineZ, RegionZ, Tabular, Text, CAD
- Geometry preview in GeoJSON format
- Cross-platform: macOS, Windows, Linux

### Prerequisites

- Go 1.21 or later
- Node.js 18 or later
- Wails CLI: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Installation

```bash
cd cmd/udbx4go-viewer

# Install frontend dependencies
cd frontend && npm install && cd ..

# Build for current platform
wails build

# Or build for specific platform
wails build -platform darwin/universal
wails build -platform windows/amd64
wails build -platform linux/amd64
```

### Development

```bash
cd cmd/udbx4go-viewer

# Run in development mode (with hot reload)
wails dev

# Build with debug info
wails build -debug
```

### Usage

```bash
# Run the built app
./build/bin/udbx4go-viewer.app/Contents/MacOS/udbx4go-viewer

# Or open the .app bundle directly
open ./build/bin/udbx4go-viewer.app
```

Click "选择文件" to open a `.udbx` file. The dataset list appears on the left, click any dataset to view its records in the table on the right.

## Creating a New UDBX File

```go
package main

import (
    "log"
    "github.com/udbx4x/udbx4go"
)

func main() {
    // Create a new UDBX file
    ds, err := udbx4go.Create("newdata.udbx")
    if err != nil {
        log.Fatal(err)
    }
    defer ds.Close()

    // Create a point dataset with custom fields
    fields := []*udbx4go.FieldInfo{
        {Name: "name", FieldType: udbx4go.FieldTypeText, Nullable: true},
        {Name: "population", FieldType: udbx4go.FieldTypeInt32, Nullable: true},
    }

    pointDS, err := ds.CreatePointDataset("cities", 4326, fields)
    if err != nil {
        log.Fatal(err)
    }

    // Insert a feature
    feature := &udbx4go.Feature{
        ID: 1,
        Geometry: &udbx4go.PointGeometry{
            Type:        "Point",
            Coordinates: []float64{116.4, 39.9},
        },
        Attributes: map[string]interface{}{
            "name":       "Beijing",
            "population": 21540000,
        },
    }

    if err := pointDS.Insert(feature); err != nil {
        log.Fatal(err)
    }
}
```

## Dataset Types

| Dataset Type | Description | Geometry |
|--------------|-------------|----------|
| `Tabular` | Attribute-only table | None |
| `Point` | 2D Point dataset | Point |
| `Line` | 2D Line dataset | MultiLineString |
| `Region` | 2D Region dataset | MultiPolygon |
| `PointZ` | 3D Point dataset | Point (with Z) |
| `LineZ` | 3D Line dataset | MultiLineString (with Z) |
| `RegionZ` | 3D Region dataset | MultiPolygon (with Z) |
| `Text` | Text annotation dataset | GeoText |
| `CAD` | CAD dataset | Custom GeoHeader (`GeoPoint` / `GeoLine` / `GeoRegion`) |

## Field Types

| Field Type | Go Type | SQLite Type |
|------------|---------|-------------|
| `Boolean` | `bool` | INTEGER |
| `Byte` | `int8` | INTEGER |
| `Int16` | `int16` | INTEGER |
| `Int32` | `int32` | INTEGER |
| `Int64` | `int64` | INTEGER |
| `Single` | `float32` | REAL |
| `Double` | `float64` | REAL |
| `Date` | `string` | TEXT |
| `Time` | `string` | TEXT |
| `Char` | `string` | TEXT |
| `Text` | `string` | TEXT |
| `NText` | `string` | TEXT |
| `Binary` | `[]byte` | BLOB |
| `Geometry` | `[]byte` | BLOB |

## CRUD Operations

### Point Dataset

```go
// Get by ID
feature, err := pointDS.GetByID(1)
if err != nil {
    if udbx4go.IsNotFound(err) {
        log.Println("Feature not found")
    } else {
        log.Fatal(err)
    }
}

// Insert
newFeature := &udbx4go.Feature{
    ID: 2,
    Geometry: &udbx4go.PointGeometry{
        Type:        "Point",
        Coordinates: []float64{121.5, 31.2},
    },
    Attributes: map[string]interface{}{
        "name":       "Shanghai",
        "population": 26320000,
    },
}
err = pointDS.Insert(newFeature)

// Update
err = pointDS.Update(2, &udbx4go.FeatureChanges{
    Geometry: &udbx4go.PointGeometry{
        Type:        "Point",
        Coordinates: []float64{121.6, 31.3},
    },
    Attributes: map[string]interface{}{
        "population": 26400000,
    },
})

// Delete
err = pointDS.Delete(2)
```

### Line Dataset

```go
lineDS, err := ds.GetLineDataset("roads")

// Insert a line feature
lineFeature := &udbx4go.Feature{
    ID: 1,
    Geometry: &udbx4go.MultiLineStringGeometry{
        Type: "MultiLineString",
        Coordinates: [][][]float64{
            {{116.4, 39.9}, {116.5, 39.8}, {116.6, 39.85}},
        },
    },
    Attributes: map[string]interface{}{
        "name":   "Highway 1",
        "length": 15.5,
    },
}
err = lineDS.Insert(lineFeature)
```

### Region Dataset

```go
regionDS, err := ds.GetRegionDataset("districts")

// Insert a polygon feature
regionFeature := &udbx4go.Feature{
    ID: 1,
    Geometry: &udbx4go.MultiPolygonGeometry{
        Type: "MultiPolygon",
        Coordinates: [][][][]float64{
            {
                {{0, 0}, {10, 0}, {10, 10}, {0, 10}, {0, 0}},
            },
        },
    },
    Attributes: map[string]interface{}{
        "name": "District A",
        "area": 100.0,
    },
}
err = regionDS.Insert(regionFeature)
```

### Tabular Dataset

```go
tabularDS, err := ds.GetTabularDataset("attributes")

// Insert a record
record := &udbx4go.TabularRecord{
    ID: 1,
    Attributes: map[string]interface{}{
        "code":  "ATTR001",
        "value": 99.9,
    },
}
err = tabularDS.Insert(record)

// Update
err = tabularDS.Update(1, map[string]interface{}{
    "value": 100.0,
})
```

## Stable Public API Semantics

`udbx4go` follows `udbx4spec/docs/08-api-stable-surface.md`. Go APIs use synchronous calls and `(value, error)` returns, while preserving the same cross-language behavior as `udbx4j` and `udbx4ts`.

| Meaning | Go API | Stable behavior |
|---|---|---|
| Open data source | `Open(path)` | Open an existing UDBX file |
| Create data source | `Create(path)` | Create UDBX and initialize system tables |
| List datasets | `ListDatasets()` | Return a `DatasetInfo` list |
| Get dataset by name | `GetDataset(name)` / typed getters | Return not found error when missing |
| Get object by ID | `GetByID(id)` | Return `nil, err`; `udbx4go.IsNotFound(err)` must be true when missing |
| List objects | `List(options)` | Return objects ordered by `SmID` ascending by default |
| Count | `Count()` | Read the physical table row count, not cached `SmRegister.SmObjectCount` |
| Write | `Insert(...)` / `InsertMany(...)` | Write objects and synchronize object count |
| Update | `Update(id, ...)` | Return not found when the target is missing; unknown fields return not found or constraint errors |
| Delete | `Delete(id)` | Return not found when the target is missing; synchronize object count after success |

Error handling example:

```go
feature, err := pointDS.GetByID(42)
if err != nil {
    if udbx4go.IsNotFound(err) {
        // Dataset, field, record, or feature not found.
        return
    }
    log.Fatal(err)
}
log.Println(feature.ID)
```

## Error Handling

udbx4go provides specific error types for different failure scenarios:

```go
dataset, err := ds.GetDataset("nonexistent")
if err != nil {
    if errors.Is(err, udbx4go.ErrNotFound) {
        // Handle not found
    } else if udbxErr, ok := err.(udbx4go.UdbxError); ok {
        log.Printf("UDBX error [%s]: %v", udbxErr.Code(), err)
    }
}
```

## Specification

This library follows the [udbx4spec](https://github.com/udbx4x/udbx4spec) cross-language specification for compatibility with:

- [udbx4j](https://github.com/udbx4x/udbx4j) - Java implementation
- [udbx4ts](https://github.com/udbx4x/udbx4ts) - TypeScript implementation

## Development

### Prerequisites

- Go 1.21 or later
- C compiler (for SQLite CGO bindings)

### Setup

```bash
# Clone the repository
git clone https://github.com/udbx4x/udbx4go.git
cd udbx4go

# Install dependencies
go mod download
```

### Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### Code Quality

```bash
# Format code
go fmt ./...

# Run linter
go vet ./...

# Run with race detection
go test -race ./...
```

## Project Structure

```
udbx4go/
├── pkg/                    # Public API
│   ├── types/              # Core types (DatasetKind, FieldType, Geometry, etc.)
│   └── errors/             # Error types and handling
├── internal/               # Internal implementation
│   ├── codec/              # Binary codecs (GAIA, CAD)
│   ├── dataset/            # Dataset implementations (Point, Line, Region, Tabular)
│   ├── schema/             # Schema initialization
│   └── system/             # System table DAOs (SmRegister, SmFieldInfo, etc.)
├── cmd/                    # Applications
│   ├── udbx4go-example/    # Example usage
│   └── udbx4go-viewer/     # GUI viewer (Wails-based)
│       ├── main.go         # Entry point (Wails)
│       ├── app.go          # Go backend bindings
│       ├── models.go       # DTO types
│       └── frontend/       # React + TypeScript frontend
│           ├── src/
│           │   ├── App.tsx         # Main app component
│           │   ├── DatasetTree.tsx # Dataset list sidebar
│           │   ├── DataTable.tsx   # MUI X-DataGrid table
│           │   └── main.tsx        # Entry point
│           └── package.json
├── udbx.go                 # Main package with re-exports
└── datasource.go           # DataSource implementation
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass (`go test ./...`)
2. Code coverage is maintained (currently 76%+)
3. Follow Go best practices (`go fmt`, `go vet`)
4. Run tests with race detector (`go test -race ./...`)
5. Add tests for new features
6. Update documentation as needed
7. Before a public release, follow the release gates in the workspace root `RELEASE.md`

## Architecture

### Core Design Principles

- **Immutable data**: Prefer creating new objects over modifying existing ones
- **Interface-based design**: Small, focused interfaces for extensibility
- **Error wrapping**: All errors include context for debugging
- **TDD approach**: Tests are written before implementation

### Binary Format

UDBX uses SQLite as the underlying storage with GAIA geometry encoding:

```
GAIA Point Header (43 bytes):
0x00 | byteOrder(0x01) | srid(int32) | MBR(4×double) | 0x7c | geoType(int32)
```

### System Tables

| Table | Purpose |
|-------|---------|
| `SmRegister` | Dataset metadata (name, kind, bounds, count) |
| `SmFieldInfo` | Field metadata (name, type, alias, nullable) |
| `geometry_columns` | Geometry column registration |
| `SmDataSourceInfo` | File-level metadata |

## Related Projects

- [udbx4spec](https://github.com/udbx4x/udbx4spec) - Cross-language specification
- [udbx4j](https://github.com/udbx4x/udbx4j) - Java implementation
- [udbx4ts](https://github.com/udbx4x/udbx4ts) - TypeScript implementation
