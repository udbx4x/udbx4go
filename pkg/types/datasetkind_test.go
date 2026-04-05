package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDatasetKind_String(t *testing.T) {
	tests := []struct {
		kind     DatasetKind
		expected string
	}{
		{DatasetKindTabular, "tabular"},
		{DatasetKindPoint, "point"},
		{DatasetKindLine, "line"},
		{DatasetKindRegion, "region"},
		{DatasetKindText, "text"},
		{DatasetKindPointZ, "pointZ"},
		{DatasetKindLineZ, "lineZ"},
		{DatasetKindRegionZ, "regionZ"},
		{DatasetKindCAD, "cad"},
		{DatasetKind(999), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.kind.String())
		})
	}
}

func TestFromDatasetKindString(t *testing.T) {
	tests := []struct {
		input    string
		expected DatasetKind
		ok       bool
	}{
		{"tabular", DatasetKindTabular, true},
		{"point", DatasetKindPoint, true},
		{"line", DatasetKindLine, true},
		{"region", DatasetKindRegion, true},
		{"text", DatasetKindText, true},
		{"pointZ", DatasetKindPointZ, true},
		{"lineZ", DatasetKindLineZ, true},
		{"regionZ", DatasetKindRegionZ, true},
		{"cad", DatasetKindCAD, true},
		{"unknown", DatasetKindTabular, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			kind, ok := FromDatasetKindString(tt.input)
			assert.Equal(t, tt.ok, ok)
			if tt.ok {
				assert.Equal(t, tt.expected, kind)
			}
		})
	}
}

func TestDatasetKind_IsSpatial(t *testing.T) {
	tests := []struct {
		kind     DatasetKind
		expected bool
	}{
		{DatasetKindTabular, false},
		{DatasetKindPoint, true},
		{DatasetKindLine, true},
		{DatasetKindRegion, true},
		{DatasetKindText, true},
		{DatasetKindPointZ, true},
		{DatasetKindLineZ, true},
		{DatasetKindRegionZ, true},
		{DatasetKindCAD, true},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.kind.IsSpatial())
		})
	}
}

func TestDatasetKind_Is3D(t *testing.T) {
	tests := []struct {
		kind     DatasetKind
		expected bool
	}{
		{DatasetKindTabular, false},
		{DatasetKindPoint, false},
		{DatasetKindLine, false},
		{DatasetKindRegion, false},
		{DatasetKindPointZ, true},
		{DatasetKindLineZ, true},
		{DatasetKindRegionZ, true},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.kind.Is3D())
		})
	}
}

func TestDatasetKind_GeometryType(t *testing.T) {
	tests := []struct {
		kind     DatasetKind
		expected int
	}{
		{DatasetKindTabular, 0},
		{DatasetKindPoint, 1},
		{DatasetKindLine, 5},
		{DatasetKindRegion, 6},
		{DatasetKindPointZ, 1001},
		{DatasetKindLineZ, 1005},
		{DatasetKindRegionZ, 1006},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.kind.GeometryType())
		})
	}
}

func TestDatasetKind_CoordDimension(t *testing.T) {
	tests := []struct {
		kind     DatasetKind
		expected int
	}{
		{DatasetKindTabular, 2},
		{DatasetKindPoint, 2},
		{DatasetKindPointZ, 3},
		{DatasetKindLineZ, 3},
		{DatasetKindRegionZ, 3},
	}

	for _, tt := range tests {
		t.Run(tt.kind.String(), func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.kind.CoordDimension())
		})
	}
}
