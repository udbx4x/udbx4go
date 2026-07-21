package types

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoundingBoxValidate(t *testing.T) {
	tests := []struct {
		name    string
		bounds  BoundingBox
		wantErr bool
	}{
		{name: "normal", bounds: BoundingBox{MinX: -1, MinY: -2, MaxX: 3, MaxY: 4}},
		{name: "zero width", bounds: BoundingBox{MinX: 1, MinY: 2, MaxX: 1, MaxY: 3}},
		{name: "zero area", bounds: BoundingBox{MinX: 1, MinY: 2, MaxX: 1, MaxY: 2}},
		{name: "nan min x", bounds: BoundingBox{MinX: math.NaN(), MinY: 0, MaxX: 1, MaxY: 1}, wantErr: true},
		{name: "positive infinity min y", bounds: BoundingBox{MinX: 0, MinY: math.Inf(1), MaxX: 1, MaxY: 1}, wantErr: true},
		{name: "negative infinity max x", bounds: BoundingBox{MinX: 0, MinY: 0, MaxX: math.Inf(-1), MaxY: 1}, wantErr: true},
		{name: "nan max y", bounds: BoundingBox{MinX: 0, MinY: 0, MaxX: 1, MaxY: math.NaN()}, wantErr: true},
		{name: "inverted x", bounds: BoundingBox{MinX: 2, MinY: 0, MaxX: 1, MaxY: 1}, wantErr: true},
		{name: "inverted y", bounds: BoundingBox{MinX: 0, MinY: 2, MaxX: 1, MaxY: 1}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.bounds.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestBoundingBoxIntersectsUsesClosedIntervals(t *testing.T) {
	base := BoundingBox{MinX: 0, MinY: 0, MaxX: 10, MaxY: 10}
	tests := []struct {
		name  string
		other BoundingBox
		want  bool
	}{
		{name: "overlap", other: BoundingBox{MinX: 2, MinY: 3, MaxX: 4, MaxY: 5}, want: true},
		{name: "touch right edge", other: BoundingBox{MinX: 10, MinY: 3, MaxX: 20, MaxY: 5}, want: true},
		{name: "touch corner", other: BoundingBox{MinX: 10, MinY: 10, MaxX: 10, MaxY: 10}, want: true},
		{name: "separate on x", other: BoundingBox{MinX: 11, MinY: 3, MaxX: 20, MaxY: 5}},
		{name: "separate on y", other: BoundingBox{MinX: 3, MinY: -2, MaxX: 5, MaxY: -1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, base.Intersects(tt.other))
		})
	}
}

func TestSpatialQueryConstantValues(t *testing.T) {
	strategies := []struct {
		value SpatialQueryStrategy
		want  string
	}{
		{SpatialQueryStrategyRTree, "rtree"},
		{SpatialQueryStrategyEnvelopeCache, "envelope_cache"},
	}
	for _, tt := range strategies {
		assert.Equal(t, tt.want, string(tt.value))
	}

	reasons := []struct {
		value SpatialQueryReason
		want  string
	}{
		{SpatialQueryReasonInvalidViewport, "invalid_viewport"},
		{SpatialQueryReasonSpatialIndexUnavailable, "spatial_index_unavailable"},
		{SpatialQueryReasonEnvelopeCacheBudgetExceeded, "envelope_cache_budget_exceeded"},
		{SpatialQueryReasonQueryTimeout, "query_timeout"},
		{SpatialQueryReasonCorruptGeometry, "corrupt_geometry"},
		{SpatialQueryReasonUnsupportedDatasetKind, "unsupported_dataset_kind"},
	}
	for _, tt := range reasons {
		assert.Equal(t, tt.want, string(tt.value))
		assert.True(t, tt.value.Valid())
	}
	assert.False(t, SpatialQueryReason("").Valid())
	assert.False(t, SpatialQueryReason("arbitrary_reason").Valid())
}

func TestSpatialQueryOptionsNormalize(t *testing.T) {
	inputIDs := []int{7, 3, 7, 9, 3}
	options := SpatialQueryOptions{
		Bounds:      BoundingBox{MinX: 1, MinY: 2, MaxX: 3, MaxY: 4},
		Limit:       25,
		RequiredIDs: inputIDs,
	}

	normalized, err := options.Normalize()
	require.NoError(t, err)
	assert.Equal(t, options.Bounds, normalized.Bounds)
	assert.Equal(t, options.Limit, normalized.Limit)
	assert.Equal(t, []int{7, 3, 9}, normalized.RequiredIDs)
	assert.Equal(t, []int{7, 3, 7, 9, 3}, inputIDs, "Normalize must not modify the caller's slice")

	normalized.RequiredIDs[0] = 100
	assert.Equal(t, 7, inputIDs[0], "normalized RequiredIDs must not share backing storage")
}

func TestSpatialQueryOptionsNormalizeRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name    string
		options SpatialQueryOptions
	}{
		{
			name: "invalid bounds",
			options: SpatialQueryOptions{
				Bounds: BoundingBox{MinX: 2, MinY: 0, MaxX: 1, MaxY: 1},
				Limit:  1,
			},
		},
		{name: "zero limit", options: SpatialQueryOptions{Bounds: BoundingBox{}, Limit: 0}},
		{name: "negative limit", options: SpatialQueryOptions{Bounds: BoundingBox{}, Limit: -1}},
		{name: "limit cannot allow limit plus one", options: SpatialQueryOptions{Bounds: BoundingBox{}, Limit: math.MaxInt}},
		{name: "zero required id", options: SpatialQueryOptions{Bounds: BoundingBox{}, Limit: 1, RequiredIDs: []int{1, 0}}},
		{name: "negative required id", options: SpatialQueryOptions{Bounds: BoundingBox{}, Limit: 1, RequiredIDs: []int{-1}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.options.Normalize()
			require.Error(t, err)
			if tt.options.Limit == math.MaxInt {
				assert.Contains(t, err.Error(), "limit must allow limit + 1")
			}
		})
	}
}

func TestSpatialQueryOptionsNormalizeAllowsLargestSafeLimit(t *testing.T) {
	options := SpatialQueryOptions{Bounds: BoundingBox{}, Limit: math.MaxInt - 1}

	normalized, err := options.Normalize()
	require.NoError(t, err)
	assert.Equal(t, math.MaxInt-1, normalized.Limit)
}

func TestSpatialQueryPolicyDefaultAndValidate(t *testing.T) {
	policy := DefaultSpatialQueryPolicy()
	assert.Equal(t, int64(32*1024*1024), policy.MaxDatasetCacheBytes)
	assert.Equal(t, int64(64*1024*1024), policy.MaxTotalCacheBytes)
	assert.Equal(t, 500*time.Millisecond, policy.BuildTimeout)
	assert.NoError(t, policy.Validate())

	tests := []struct {
		name   string
		policy SpatialQueryPolicy
	}{
		{name: "zero dataset budget", policy: SpatialQueryPolicy{MaxTotalCacheBytes: 1, BuildTimeout: time.Second}},
		{name: "zero total budget", policy: SpatialQueryPolicy{MaxDatasetCacheBytes: 1, BuildTimeout: time.Second}},
		{name: "dataset exceeds total", policy: SpatialQueryPolicy{MaxDatasetCacheBytes: 2, MaxTotalCacheBytes: 1, BuildTimeout: time.Second}},
		{name: "zero timeout", policy: SpatialQueryPolicy{MaxDatasetCacheBytes: 1, MaxTotalCacheBytes: 1}},
		{name: "negative timeout", policy: SpatialQueryPolicy{MaxDatasetCacheBytes: 1, MaxTotalCacheBytes: 1, BuildTimeout: -time.Second}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.policy.Validate())
		})
	}
}

func TestSpatialQueryResultAndCapabilityFields(t *testing.T) {
	feature := &Feature{ID: 1}
	result := SpatialQueryResult{
		Features:      []*Feature{feature},
		QueriedBounds: BoundingBox{MinX: 1, MinY: 2, MaxX: 3, MaxY: 4},
		Strategy:      SpatialQueryStrategyEnvelopeCache,
		HasMore:       true,
	}
	assert.Same(t, feature, result.Features[0])
	assert.True(t, result.HasMore)

	capability := SpatialQueryCapability{
		Supported:         true,
		RTreeAvailable:    false,
		FallbackAvailable: true,
		DiagnosticReason:  SpatialQueryReasonSpatialIndexUnavailable,
	}
	assert.True(t, capability.Supported)
	assert.False(t, capability.RTreeAvailable)
	assert.True(t, capability.FallbackAvailable)
	assert.Equal(t, SpatialQueryReasonSpatialIndexUnavailable, capability.DiagnosticReason)
}
