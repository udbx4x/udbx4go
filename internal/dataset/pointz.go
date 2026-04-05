package dataset

import (
	"database/sql"

	"github.com/udbx4x/udbx4go/pkg/types"
)

// PointZDataset represents a 3D point dataset.
// It behaves like PointDataset but uses 3D geometries.
type PointZDataset struct {
	*PointDataset
}

// NewPointZDataset creates a new 3D point dataset.
func NewPointZDataset(db *sql.DB, info *types.DatasetInfo) *PointZDataset {
	return &PointZDataset{
		PointDataset: NewPointDataset(db, info),
	}
}

