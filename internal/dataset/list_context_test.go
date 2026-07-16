package dataset

import (
	"context"
	"database/sql"
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/codec"
	udbxerrors "github.com/udbx4x/udbx4go/pkg/errors"
	"github.com/udbx4x/udbx4go/pkg/types"
)

type contextFeatureLister interface {
	ListContext(context.Context, *types.QueryOptions) ([]*types.Feature, error)
}

func TestSpatialDatasetListContextMapsCancellationToQueryTimeout(t *testing.T) {
	tests := []struct {
		name   string
		create func(*testing.T, *sql.DB) contextFeatureLister
	}{
		{name: "point", create: func(t *testing.T, db *sql.DB) contextFeatureLister {
			dataset, _ := createPointDataset(t, db)
			return dataset
		}},
		{name: "line", create: func(t *testing.T, db *sql.DB) contextFeatureLister {
			dataset, _ := createLineDataset(t, db)
			return dataset
		}},
		{name: "region", create: func(t *testing.T, db *sql.DB) contextFeatureLister {
			dataset, _ := createRegionDataset(t, db)
			return dataset
		}},
		{name: "point-z", create: func(t *testing.T, db *sql.DB) contextFeatureLister {
			dataset, _ := createPointDataset(t, db)
			return NewPointZDataset(db, dataset.Info())
		}},
		{name: "line-z", create: func(t *testing.T, db *sql.DB) contextFeatureLister {
			dataset, _ := createLineDataset(t, db)
			return NewLineZDataset(db, dataset.Info())
		}},
		{name: "region-z", create: func(t *testing.T, db *sql.DB) contextFeatureLister {
			dataset, _ := createRegionDataset(t, db)
			return NewRegionZDataset(db, dataset.Info())
		}},
		{name: "text", create: func(t *testing.T, db *sql.DB) contextFeatureLister {
			return createContextTextDataset(t, db)
		}},
		{name: "cad", create: func(t *testing.T, db *sql.DB) contextFeatureLister {
			dataset, _ := createCadDataset(t, db)
			return dataset
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupTestDB(t)
			defer db.Close()
			dataset := test.create(t, db)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			_, err := dataset.ListContext(ctx, &types.QueryOptions{Limit: 10})

			assertListContextSpatialError(t, err, types.SpatialQueryReasonQueryTimeout, udbxerrors.CodeUdbxError)
			assert.ErrorIs(t, err, context.Canceled)
		})
	}
}

func TestSpatialDatasetListContextMapsMissingGeometryToCorruptGeometry(t *testing.T) {
	tests := []struct {
		name   string
		create func(*testing.T, *sql.DB) contextFeatureLister
		insert func(*testing.T, *sql.DB)
	}{
		{
			name: "point",
			create: func(t *testing.T, db *sql.DB) contextFeatureLister {
				dataset, _ := createPointDataset(t, db)
				return dataset
			},
			insert: func(t *testing.T, db *sql.DB) {
				_, err := db.Exec(`INSERT INTO cities (SmID, SmGeometry) VALUES (1, NULL)`)
				require.NoError(t, err)
			},
		},
		{
			name: "text",
			create: func(t *testing.T, db *sql.DB) contextFeatureLister {
				return createContextTextDataset(t, db)
			},
			insert: func(t *testing.T, db *sql.DB) {
				_, err := db.Exec(`INSERT INTO labels (SmID, SmGeometry) VALUES (1, NULL)`)
				require.NoError(t, err)
			},
		},
		{
			name: "cad",
			create: func(t *testing.T, db *sql.DB) contextFeatureLister {
				dataset, _ := createCadDataset(t, db)
				return dataset
			},
			insert: func(t *testing.T, db *sql.DB) {
				_, err := db.Exec(`INSERT INTO cad_layers (SmID, SmGeometry, name) VALUES (1, NULL, 'broken')`)
				require.NoError(t, err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			db := setupTestDB(t)
			defer db.Close()
			dataset := test.create(t, db)
			test.insert(t, db)

			_, err := dataset.ListContext(context.Background(), &types.QueryOptions{Limit: 10})

			assertListContextSpatialError(t, err, types.SpatialQueryReasonCorruptGeometry, udbxerrors.CodeFormatError)
		})
	}
}

func TestSpatialDatasetListContextPreservesOrdinaryIOError(t *testing.T) {
	db := setupTestDB(t)
	dataset, _ := createPointDataset(t, db)
	require.NoError(t, db.Close())

	_, err := dataset.ListContext(context.Background(), &types.QueryOptions{Limit: 10})

	require.Error(t, err)
	assert.True(t, udbxerrors.IsIOError(err))
	_, hasReason := udbxerrors.SpatialQueryReasonOf(err)
	assert.False(t, hasReason)
}

func TestSpatialDatasetListContextPreservesNonGeometryFormatError(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	_, err := db.Exec(`CREATE TABLE malformed_features (SmID TEXT, SmGeometry BLOB)`)
	require.NoError(t, err)
	geometry, err := codec.NewGaiaGeometryCodec().Encode(&types.PointGeometry{
		Type:        "Point",
		Coordinates: []float64{1, 1},
	}, 4326)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO malformed_features VALUES ('bad-id', ?)`, geometry)
	require.NoError(t, err)
	dataset := NewPointDataset(db, &types.DatasetInfo{
		Name:      "malformed_features",
		TableName: "malformed_features",
		Kind:      types.DatasetKindPoint,
	})

	_, err = dataset.ListContext(context.Background(), &types.QueryOptions{Limit: 10})

	require.Error(t, err)
	assert.True(t, udbxerrors.IsFormatError(err))
	_, hasReason := udbxerrors.SpatialQueryReasonOf(err)
	assert.False(t, hasReason)
}

func createContextTextDataset(t *testing.T, db *sql.DB) *TextDataset {
	t.Helper()
	_, err := db.Exec(`CREATE TABLE labels (
		SmID INTEGER PRIMARY KEY,
		SmGeometry BLOB
	)`)
	require.NoError(t, err)
	return NewTextDataset(db, &types.DatasetInfo{
		Name:      "labels",
		TableName: "labels",
		Kind:      types.DatasetKindText,
	})
}

func assertListContextSpatialError(
	t *testing.T,
	err error,
	wantReason types.SpatialQueryReason,
	wantCode string,
) {
	t.Helper()
	require.Error(t, err)
	reason, ok := udbxerrors.SpatialQueryReasonOf(err)
	require.True(t, ok)
	assert.Equal(t, wantReason, reason)
	var udbxErr udbxerrors.UdbxError
	require.True(t, stderrors.As(err, &udbxErr))
	assert.Equal(t, wantCode, udbxErr.Code())
}
