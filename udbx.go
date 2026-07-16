// Package udbx4go provides a Go implementation of the UDBX (Universal Spatial Database Extension)
// reader/writer library. UDBX is a spatial data format based on SQLite.
//
// # Basic Usage
//
// Opening an existing UDBX file:
//
//	ds, err := udbx4go.Open("data.udbx")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer ds.Close()
//
// Creating a new UDBX file:
//
//	ds, err := udbx4go.Create("newdata.udbx")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer ds.Close()
//
// # Dataset Types
//
// UDBX supports multiple dataset types:
//   - Point: 2D point features
//   - Line: 2D line features
//   - Region: 2D polygon features
//   - PointZ: 3D point features
//   - LineZ: 3D line features
//   - RegionZ: 3D polygon features
//   - Tabular: Non-spatial tables
//   - Text: Text annotation features
//   - CAD: CAD geometry features
//
// # Geometry Model
//
// The library uses a GeoJSON-like geometry model:
//   - PointGeometry: {"type": "Point", "coordinates": [x, y]}
//   - MultiLineStringGeometry: {"type": "MultiLineString", "coordinates": [...]}
//   - MultiPolygonGeometry: {"type": "MultiPolygon", "coordinates": [...]}
//
// # Error Handling
//
// All errors implement the UdbxError interface with a Code() method:
//   - FormatError: Invalid UDBX format
//   - NotFoundError: Dataset or feature not found
//   - UnsupportedError: Unsupported operation
//   - ConstraintError: Data constraint violation
//   - IOError: File I/O error
//
// Example:
//
//	dataset, err := ds.GetDataset("cities")
//	if err != nil {
//	    if errors.Is(err, udbx4go.ErrNotFound) {
//	        // Handle not found
//	    }
//	}
//
// # Specification Compliance
//
// This library follows the udbx4spec cross-language specification for compatibility
// with Java (udbx4j) and TypeScript (udbx4ts) implementations.
package udbx4go

import (
	"github.com/udbx4x/udbx4go/internal/dataset"
	"github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

// Re-export types for convenience.
type (
	// DatasetKind represents the type of dataset.
	DatasetKind = types.DatasetKind

	// FieldType represents the type of a field.
	FieldType = types.FieldType

	// Geometry is the interface for all geometry types.
	Geometry = types.Geometry

	// PointGeometry represents a Point geometry.
	PointGeometry = types.PointGeometry

	// MultiLineStringGeometry represents a MultiLineString geometry.
	MultiLineStringGeometry = types.MultiLineStringGeometry

	// MultiPolygonGeometry represents a MultiPolygon geometry.
	MultiPolygonGeometry = types.MultiPolygonGeometry

	// CadStyle represents a CAD GeoHeader style payload.
	CadStyle = types.CadStyle

	// CadMarkerStyle represents a CAD marker style.
	CadMarkerStyle = types.CadMarkerStyle

	// CadLineStyle represents a CAD line style.
	CadLineStyle = types.CadLineStyle

	// CadFillStyle represents a CAD fill style.
	CadFillStyle = types.CadFillStyle

	// CadGeometry represents a minimal CAD GeoHeader geometry.
	CadGeometry = types.CadGeometry

	// Color represents a UDBX Color value in ABGR byte order.
	Color = types.Color

	// TextStyle represents GeoText style.
	TextStyle = types.TextStyle

	// TextSubText represents a GeoText sub-text object.
	TextSubText = types.TextSubText

	// TextGeometry represents a UDBX GeoText geometry.
	TextGeometry = types.TextGeometry

	// CadPointGeometry represents a CAD point geometry.
	CadPointGeometry = types.CadPointGeometry

	// CadLineGeometry represents a CAD line geometry.
	CadLineGeometry = types.CadLineGeometry

	// CadRegionGeometry represents a CAD region geometry.
	CadRegionGeometry = types.CadRegionGeometry

	// Feature represents a spatial feature.
	Feature = types.Feature

	// TabularRecord represents a non-spatial record.
	TabularRecord = types.TabularRecord

	// DatasetInfo holds metadata about a dataset.
	DatasetInfo = types.DatasetInfo

	// FieldInfo holds metadata about a field.
	FieldInfo = types.FieldInfo

	// QueryOptions provides options for querying features.
	QueryOptions = types.QueryOptions

	// BoundingBox is an axis-aligned spatial extent.
	BoundingBox = types.BoundingBox

	// SpatialQueryStrategy identifies how a spatial query was executed.
	SpatialQueryStrategy = types.SpatialQueryStrategy

	// SpatialQueryReason explains why a spatial query failed or degraded.
	SpatialQueryReason = types.SpatialQueryReason

	// SpatialQueryOptions provides options for a viewport spatial query.
	SpatialQueryOptions = types.SpatialQueryOptions

	// SpatialQueryResult contains features and execution facts for a spatial query.
	SpatialQueryResult = types.SpatialQueryResult

	// SpatialQueryPolicy limits spatial-query fallback resource usage.
	SpatialQueryPolicy = types.SpatialQueryPolicy

	// SpatialQueryCapability describes the available spatial-query execution paths.
	SpatialQueryCapability = types.SpatialQueryCapability

	// FeatureChanges holds geometry and attribute changes for vector dataset updates.
	FeatureChanges = dataset.FeatureChanges
)

// DatasetKind constants.
const (
	DatasetKindTabular = types.DatasetKindTabular
	DatasetKindPoint   = types.DatasetKindPoint
	DatasetKindLine    = types.DatasetKindLine
	DatasetKindRegion  = types.DatasetKindRegion
	DatasetKindText    = types.DatasetKindText
	DatasetKindPointZ  = types.DatasetKindPointZ
	DatasetKindLineZ   = types.DatasetKindLineZ
	DatasetKindRegionZ = types.DatasetKindRegionZ
	DatasetKindCAD     = types.DatasetKindCAD
)

// SpatialQueryStrategy constants.
const (
	SpatialQueryStrategyRTree         = types.SpatialQueryStrategyRTree
	SpatialQueryStrategyEnvelopeCache = types.SpatialQueryStrategyEnvelopeCache
	SpatialQueryStrategyBoundedSample = types.SpatialQueryStrategyBoundedSample
)

// SpatialQueryReason constants.
const (
	SpatialQueryReasonInvalidViewport             = types.SpatialQueryReasonInvalidViewport
	SpatialQueryReasonSpatialIndexUnavailable     = types.SpatialQueryReasonSpatialIndexUnavailable
	SpatialQueryReasonEnvelopeCacheBudgetExceeded = types.SpatialQueryReasonEnvelopeCacheBudgetExceeded
	SpatialQueryReasonQueryTimeout                = types.SpatialQueryReasonQueryTimeout
	SpatialQueryReasonCorruptGeometry             = types.SpatialQueryReasonCorruptGeometry
	SpatialQueryReasonUnsupportedDatasetKind      = types.SpatialQueryReasonUnsupportedDatasetKind
)

// FieldType constants.
const (
	FieldTypeBoolean  = types.FieldTypeBoolean
	FieldTypeByte     = types.FieldTypeByte
	FieldTypeInt16    = types.FieldTypeInt16
	FieldTypeInt32    = types.FieldTypeInt32
	FieldTypeInt64    = types.FieldTypeInt64
	FieldTypeSingle   = types.FieldTypeSingle
	FieldTypeDouble   = types.FieldTypeDouble
	FieldTypeDate     = types.FieldTypeDate
	FieldTypeBinary   = types.FieldTypeBinary
	FieldTypeGeometry = types.FieldTypeGeometry
	FieldTypeChar     = types.FieldTypeChar
	FieldTypeNText    = types.FieldTypeNText
	FieldTypeText     = types.FieldTypeText
	FieldTypeTime     = types.FieldTypeTime
)

// Error types and functions.
type (
	// UdbxError is the interface for all UDBX errors.
	UdbxError = errors.UdbxError

	// SpatialQueryError associates a spatial-query failure reason with its cause.
	SpatialQueryError = errors.SpatialQueryError
)

// Error checking functions.
var (
	IsFormatError             = errors.IsFormatError
	IsNotFound                = errors.IsNotFound
	IsUnsupported             = errors.IsUnsupported
	IsConstraintViolation     = errors.IsConstraintViolation
	IsIOError                 = errors.IsIOError
	IsUdbxError               = errors.IsUdbxError
	DefaultSpatialQueryPolicy = types.DefaultSpatialQueryPolicy
	NewSpatialQueryError      = errors.NewSpatialQueryError
	SpatialQueryReasonOf      = errors.SpatialQueryReasonOf
)

// Sentinel errors.
var (
	ErrNotFound    = errors.ErrNotFound
	ErrFormat      = errors.ErrFormat
	ErrUnsupported = errors.ErrUnsupported
	ErrConstraint  = errors.ErrConstraint
	ErrIO          = errors.ErrIO
)
