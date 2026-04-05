// Package types provides core type definitions for udbx4go.
//
// This package defines the fundamental types used throughout the library:
//   - DatasetKind: Enumeration of dataset types (Point, Line, Region, etc.)
//   - FieldType: Enumeration of field types (Int32, Text, Double, etc.)
//   - Geometry types: PointGeometry, MultiLineStringGeometry, MultiPolygonGeometry
//   - Feature: Spatial feature with geometry and attributes
//   - TabularRecord: Non-spatial record with attributes only
//   - DatasetInfo: Metadata about a dataset
//   - FieldInfo: Metadata about a field
//   - QueryOptions: Options for querying features
//
// All types follow the udbx4spec cross-language specification for compatibility
// with udbx4j (Java) and udbx4ts (TypeScript) implementations.
package types
