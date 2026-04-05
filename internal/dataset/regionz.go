package dataset

import (
	"database/sql"

	"github.com/udbx4x/udbx4go/pkg/types"
)

// RegionZDataset represents a 3D region dataset.
// It behaves like RegionDataset but uses 3D geometries.
type RegionZDataset struct {
	*RegionDataset
}

// NewRegionZDataset creates a new 3D region dataset.
func NewRegionZDataset(db *sql.DB, info *types.DatasetInfo) *RegionZDataset {
	return &RegionZDataset{
		RegionDataset: NewRegionDataset(db, info),
	}
}

