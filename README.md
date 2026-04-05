# udbx4go

[![Go Reference](https://pkg.go.dev/badge/github.com/udbx4x/udbx4go.svg)](https://pkg.go.dev/github.com/udbx4x/udbx4go)
[![Go Report Card](https://goreportcard.com/badge/github.com/udbx4x/udbx4go)](https://goreportcard.com/report/github.com/udbx4x/udbx4go)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](./)
[![Coverage](https://img.shields.io/badge/coverage-76.7%25-yellowgreen)](./)

A Go implementation of the UDBX (Universal Spatial Database Extension) reader/writer library. UDBX is a spatial data format based on SQLite, supporting vector (Point, Line, Region, CAD) and tabular datasets.

[中文](./README.zh.md)

## Features

- ✅ Full UDBX format support (read/write)
- ✅ All dataset types: Point, Line, Region, PointZ, LineZ, RegionZ, Tabular, Text, CAD
- ✅ 14 field types with proper type mapping
- ✅ GeoJSON-like geometry model
- ✅ Streaming and batch operations
- ✅ Cross-language compatibility (udbx4j, udbx4ts)
- ✅ Comprehensive error handling
- ✅ TDD development with 76%+ test coverage

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
| `CAD` | CAD dataset | Custom GeoHeader |

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
changes := &udbx4go.FeatureChanges{
    Attributes: map[string]interface{}{
        "population": 27000000,
    },
}
err = pointDS.Update(2, changes)

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
├── cmd/                    # Example applications
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
