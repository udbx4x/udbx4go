package types

import (
	"fmt"
	"math"
	"time"
)

// BoundingBox is an axis-aligned spatial extent.
type BoundingBox struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
}

// Validate checks that all coordinates are finite and ordered.
func (b BoundingBox) Validate() error {
	coordinates := []float64{b.MinX, b.MinY, b.MaxX, b.MaxY}
	for _, coordinate := range coordinates {
		if math.IsNaN(coordinate) || math.IsInf(coordinate, 0) {
			return fmt.Errorf("bounding box coordinates must be finite")
		}
	}
	if b.MinX > b.MaxX || b.MinY > b.MaxY {
		return fmt.Errorf("bounding box coordinates must be ordered")
	}
	return nil
}

// Intersects reports whether two bounding boxes overlap using closed intervals.
func (b BoundingBox) Intersects(other BoundingBox) bool {
	return b.MinX <= other.MaxX && b.MaxX >= other.MinX &&
		b.MinY <= other.MaxY && b.MaxY >= other.MinY
}

// SpatialQueryStrategy identifies how a spatial query was executed.
type SpatialQueryStrategy string

const (
	SpatialQueryStrategyRTree         SpatialQueryStrategy = "rtree"
	SpatialQueryStrategyEnvelopeCache SpatialQueryStrategy = "envelope_cache"
	SpatialQueryStrategyBoundedSample SpatialQueryStrategy = "bounded_sample"
)

// SpatialQueryReason explains why a spatial query failed or degraded.
type SpatialQueryReason string

const (
	SpatialQueryReasonInvalidViewport             SpatialQueryReason = "invalid_viewport"
	SpatialQueryReasonSpatialIndexUnavailable     SpatialQueryReason = "spatial_index_unavailable"
	SpatialQueryReasonEnvelopeCacheBudgetExceeded SpatialQueryReason = "envelope_cache_budget_exceeded"
	SpatialQueryReasonQueryTimeout                SpatialQueryReason = "query_timeout"
	SpatialQueryReasonCorruptGeometry             SpatialQueryReason = "corrupt_geometry"
	SpatialQueryReasonUnsupportedDatasetKind      SpatialQueryReason = "unsupported_dataset_kind"
)

// SpatialQueryOptions provides options for a viewport spatial query.
type SpatialQueryOptions struct {
	Bounds      BoundingBox
	Limit       int
	RequiredIDs []int
}

// Normalize validates the options and returns a copy with unique required IDs.
func (o SpatialQueryOptions) Normalize() (SpatialQueryOptions, error) {
	if err := o.Bounds.Validate(); err != nil {
		return SpatialQueryOptions{}, err
	}
	if o.Limit <= 0 {
		return SpatialQueryOptions{}, fmt.Errorf("spatial query limit must be positive")
	}

	normalized := SpatialQueryOptions{Bounds: o.Bounds, Limit: o.Limit}
	if o.RequiredIDs == nil {
		return normalized, nil
	}

	normalized.RequiredIDs = make([]int, 0, len(o.RequiredIDs))
	seen := make(map[int]struct{}, len(o.RequiredIDs))
	for _, id := range o.RequiredIDs {
		if id <= 0 {
			return SpatialQueryOptions{}, fmt.Errorf("spatial query required IDs must be positive")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized.RequiredIDs = append(normalized.RequiredIDs, id)
	}
	return normalized, nil
}

// SpatialQueryResult contains features and execution facts for a spatial query.
type SpatialQueryResult struct {
	Features       []*Feature
	QueriedBounds  BoundingBox
	Strategy       SpatialQueryStrategy
	HasMore        bool
	DegradedReason SpatialQueryReason
}

// SpatialQueryPolicy limits spatial-query fallback resource usage.
type SpatialQueryPolicy struct {
	// MaxDatasetCacheBytes and MaxTotalCacheBytes are estimated resident memory
	// budgets, not hard thresholds on object counts.
	MaxDatasetCacheBytes int64
	MaxTotalCacheBytes   int64
	BuildTimeout         time.Duration
}

// DefaultSpatialQueryPolicy returns the isolated PoC resource limits.
func DefaultSpatialQueryPolicy() SpatialQueryPolicy {
	return SpatialQueryPolicy{
		MaxDatasetCacheBytes: 32 * 1024 * 1024,
		MaxTotalCacheBytes:   64 * 1024 * 1024,
		BuildTimeout:         500 * time.Millisecond,
	}
}

// Validate checks that a spatial query policy has usable resource limits.
func (p SpatialQueryPolicy) Validate() error {
	if p.MaxDatasetCacheBytes <= 0 {
		return fmt.Errorf("maximum dataset cache bytes must be positive")
	}
	if p.MaxTotalCacheBytes <= 0 {
		return fmt.Errorf("maximum total cache bytes must be positive")
	}
	if p.MaxDatasetCacheBytes > p.MaxTotalCacheBytes {
		return fmt.Errorf("maximum dataset cache bytes must not exceed total cache bytes")
	}
	if p.BuildTimeout <= 0 {
		return fmt.Errorf("spatial query build timeout must be positive")
	}
	return nil
}

// SpatialQueryCapability describes the available spatial-query execution paths.
type SpatialQueryCapability struct {
	Supported         bool
	RTreeAvailable    bool
	FallbackAvailable bool
	DiagnosticReason  SpatialQueryReason
}
