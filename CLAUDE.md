# CLAUDE.md

This file provides guidance to Claude Code when working with the udbx4go project.

## Project Overview

`udbx4go` is a Go implementation of the UDBX (Universal Spatial Database Extension) reader/writer library. It follows the `udbx4spec` cross-language specification for compatibility with `udbx4j` (Java) and `udbx4ts` (TypeScript) implementations.

## Project Structure

```
udbx4go/
├── cmd/                          # Command-line applications
│   └── udbx4go-example/          # Example usage
├── internal/                     # Internal implementation (not exported)
│   ├── codec/                    # Binary codecs (GAIA, CAD)
│   │   ├── point.go              # Point GAIA codec
│   │   ├── line.go               # Line GAIA codec
│   │   ├── polygon.go            # Polygon GAIA codec
│   │   └── gaia.go               # Common GAIA utilities
│   ├── dataset/                  # Dataset implementations
│   │   ├── dataset.go            # Base dataset interface
│   │   ├── vector.go             # Vector dataset (base for spatial)
│   │   ├── point.go              # 2D Point dataset
│   │   ├── line.go               # 2D Line dataset
│   │   ├── region.go             # 2D Region dataset
│   │   ├── pointz.go             # 3D Point dataset
│   │   ├── linez.go              # 3D Line dataset
│   │   ├── regionz.go            # 3D Region dataset
│   │   └── tabular.go            # Non-spatial tabular dataset
│   ├── schema/                   # Schema initialization
│   │   └── initializer.go        # Creates system tables and data tables
│   └── system/                   # System table DAOs
│       ├── smregister.go         # SmRegister table operations
│       ├── smfieldinfo.go        # SmFieldInfo table operations
│       ├── geometrycolumns.go    # geometry_columns table operations
│       └── smdatasourceinfo.go   # SmDataSourceInfo table operations
├── pkg/                          # Public API
│   ├── errors/                   # Error types
│   │   └── errors.go             # UdbxError interface and implementations
│   └── types/                    # Core types
│       ├── datasetkind.go        # DatasetKind enum
│       ├── fieldtype.go          # FieldType enum
│       ├── geometry.go           # Geometry interfaces and implementations
│       ├── feature.go            # Feature and TabularRecord types
│       └── datasetinfo.go        # DatasetInfo and FieldInfo types
├── udbx.go                       # Main package documentation and re-exports
├── datasource.go                 # DataSource implementation
├── datasource_test.go            # DataSource tests
├── go.mod
├── go.sum
└── README.md
```

## Test Coverage (as of latest run)

| Package | Coverage |
|---------|----------|
| pkg/types | 98.5% |
| pkg/errors | 92.3% |
| internal/schema | 88.9% |
| internal/dataset | 83.2% |
| internal/system | 83.0% |
| internal/codec | 73.2% |
| **Total** | **76.7%** |

## Development Commands

### Using Make (Recommended)

```bash
# Format, lint, build, and test
make all

# Build the library
make build

# Run all tests
make test

# Run unit tests with coverage
make test-unit

# Run integration tests
make test-integration

# Generate coverage report
make coverage

# Format code
make fmt

# Run linter
make lint

# Clean build artifacts
make clean
```

### Using Go Directly

```bash
# Build
go build ./...

# Run all tests
go test ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Format code
go fmt ./...

# Run linter
go vet ./...
```

## Key Types

### DatasetKind

| Kind | Value | Geometry Type |
|------|-------|---------------|
| Tabular | 0 | None |
| Point | 1 | GAIAPoint (geoType=1) |
| Line | 3 | GAIAMultiLineString (geoType=5) |
| Region | 5 | GAIAMultiPolygon (geoType=6) |
| PointZ | 101 | GAIAPointZ (geoType=1001) |
| LineZ | 103 | GAIAMultiLineStringZ (geoType=1005) |
| RegionZ | 105 | GAIAMultiPolygonZ (geoType=1006) |
| Text | 7 | GeoText |
| CAD | 149 | GeoHeader |

### FieldType

14 canonical field types: `boolean`, `byte`, `int16`, `int32`, `int64`, `single`, `double`, `date`, `binary`, `geometry`, `char`, `ntext`, `text`, `time`.

## Error Handling

All errors implement the `UdbxError` interface with a `Code()` method:

- `UdbxFormatError` - Format errors
- `UdbxNotFoundError` - Not found errors
- `UdbxUnsupportedError` - Unsupported operations
- `UdbxConstraintError` - Constraint violations
- `UdbxIOError` - I/O errors

## Binary Format

### GAIA Geometry (Little-Endian)

```
0x00 | byteOrder(0x01) | srid(int32) | MBR(4×double) | 0x7c | geoType(int32) | coords... | 0xFE
```

Header length: 43 bytes.

## Testing Strategy

- **Unit Tests**: Test individual components in isolation
  - Use table-driven tests with `t.Run()` for subtests
  - Use `testify/assert` and `testify/require` for assertions
  - Each test should be independent and use `t.TempDir()` for file operations

- **Test File Naming**: `xxx_test.go` alongside the implementation file

- **Test Utilities**: 
  - `internal/dataset/testutil_test.go` contains `setupTestDB()` helper

### Example Test Pattern

```go
func TestComponent_DoSomething(t *testing.T) {
    db := setupTestDB(t)
    defer db.Close()
    
    // Test case
    result, err := component.DoSomething()
    
    require.NoError(t, err)
    assert.Equal(t, expected, result)
}

func TestComponent_DoSomethingTableDriven(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected int
        wantErr  bool
    }{
        {"valid", "test", 4, false},
        {"empty", "", 0, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := DoSomething(tt.input)
            if tt.wantErr {
                require.Error(t, err)
                return
            }
            require.NoError(t, err)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

## Dependencies

- `github.com/mattn/go-sqlite3` - SQLite driver (requires CGO)
- `github.com/stretchr/testify` - Test assertions

## API Design Patterns

### Type Re-exports

Public types are re-exported in `udbx.go` for convenient access:

```go
import "github.com/udbx4x/udbx4go"

// Use re-exported types
ds, err := udbx4go.Open("file.udbx")
feature := &udbx4go.Feature{...}
```

### Dataset Interface Hierarchy

```
Dataset (interface)
├── BaseDataset (struct)
│   └── TabularDataset
└── VectorDataset (struct)
    ├── PointDataset
    ├── LineDataset
    ├── RegionDataset
    ├── PointZDataset
    ├── LineZDataset
    └── RegionZDataset
```

### Error Handling Pattern

Always wrap errors with context:

```go
if err != nil {
    return errors.IOError("failed to query features", err)
}
```

Check specific error types:

```go
if udbx4go.IsNotFound(err) {
    // Handle not found
}
if udbx4go.IsFormatError(err) {
    // Handle format error
}
```

## Spec Compliance

This project strictly follows `udbx4spec`:

- API naming matches the specification
- Geometry model uses GeoJSON-like structures
- DatasetKind and FieldType values are synchronized
- Error taxonomy follows the spec exactly

Any API changes must first be defined in udbx4spec before implementation here.
