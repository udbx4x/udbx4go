package dataset

import (
	"database/sql"

	"github.com/udbx4x/udbx4go/pkg/types"
)

// LineZDataset represents a 3D line dataset.
// It behaves like LineDataset but uses 3D geometries.
type LineZDataset struct {
	*LineDataset
}

// NewLineZDataset creates a new 3D line dataset.
func NewLineZDataset(db *sql.DB, info *types.DatasetInfo) *LineZDataset {
	return &LineZDataset{
		LineDataset: NewLineDataset(db, info),
	}
}

