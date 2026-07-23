package dataset

import (
	"database/sql"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/system"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestBaseDatasetMutationHookCanReenterAndBeReplacedConcurrently(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	base := NewBaseDataset(db, &types.DatasetInfo{Name: "test", TableName: "test"})
	done := make(chan struct{})
	base.SetSpatialMutationHook(func() {
		base.SetSpatialMutationHook(nil)
		close(done)
	})
	go base.notifySpatialMutation()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("mutation hook deadlocked while replacing itself")
	}

	var calls atomic.Int64
	var writers sync.WaitGroup
	for range 4 {
		writers.Add(1)
		go func() {
			defer writers.Done()
			for range 100 {
				base.SetSpatialMutationHook(func() { calls.Add(1) })
				base.notifySpatialMutation()
			}
		}()
	}
	writers.Wait()
	assert.Positive(t, calls.Load())
}

func TestTextDatasetMutationHookFiresOnlyAfterAffectedWrites(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset := createMutationTextDataset(t, db)
	var calls int
	dataset.SetSpatialMutationHook(func() { calls++ })

	require.NoError(t, dataset.Insert(textMutationFeature(1, 1, 1)))
	assert.Equal(t, 1, calls)
	require.NoError(t, dataset.Update(1, &FeatureChanges{Geometry: textMutationGeometry(2, 2)}))
	assert.Equal(t, 2, calls)
	require.NoError(t, dataset.Delete(1))
	assert.Equal(t, 3, calls)

	assert.Error(t, dataset.Insert(&types.Feature{ID: 2, Geometry: &types.PointGeometry{}}))
	assert.NoError(t, dataset.Update(999, &FeatureChanges{}))
	assert.Error(t, dataset.Update(999, &FeatureChanges{Attributes: map[string]interface{}{"name": "missing"}}))
	assert.Error(t, dataset.Delete(999))
	assert.Equal(t, 3, calls)

	require.NoError(t, dataset.Insert(textMutationFeature(2, 3, 3)))
	assert.Error(t, dataset.Insert(textMutationFeature(2, 4, 4)))
	assert.Equal(t, 4, calls)
}

func TestCadDatasetMutationHookFiresOnlyAfterAffectedWrites(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dataset, _ := createCadDataset(t, db)
	var calls int
	dataset.SetSpatialMutationHook(func() { calls++ })

	require.NoError(t, dataset.Insert(cadMutationFeature(1, 1, 1)))
	assert.Equal(t, 1, calls)
	require.NoError(t, dataset.Update(1, &FeatureChanges{Geometry: &types.CadPointGeometry{XCoord: 2, YCoord: 2}}))
	assert.Equal(t, 2, calls)
	require.NoError(t, dataset.Delete(1))
	assert.Equal(t, 3, calls)

	assert.Error(t, dataset.Insert(&types.Feature{ID: 2, Geometry: &types.PointGeometry{}}))
	assert.NoError(t, dataset.Update(999, &FeatureChanges{}))
	assert.Error(t, dataset.Update(999, &FeatureChanges{Attributes: map[string]interface{}{"name": "missing"}}))
	assert.Error(t, dataset.Delete(999))
	assert.Equal(t, 3, calls)

	require.NoError(t, dataset.Insert(cadMutationFeature(2, 3, 3)))
	assert.Error(t, dataset.Insert(cadMutationFeature(2, 4, 4)))
	assert.Equal(t, 4, calls)
}

func TestSpatialDatasetInsertManyNotifiesOncePerSuccessfulRow(t *testing.T) {
	tests := []struct {
		name     string
		new      func(*testing.T, *sql.DB) mutationInsertManyDataset
		features []*types.Feature
	}{
		{
			name: "Text",
			new: func(t *testing.T, db *sql.DB) mutationInsertManyDataset {
				return createMutationTextDataset(t, db)
			},
			features: []*types.Feature{textMutationFeature(1, 1, 1), textMutationFeature(2, 2, 2)},
		},
		{
			name: "CAD",
			new: func(t *testing.T, db *sql.DB) mutationInsertManyDataset {
				dataset, _ := createCadDataset(t, db)
				return dataset
			},
			features: []*types.Feature{cadMutationFeature(1, 1, 1), cadMutationFeature(2, 2, 2)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupTestDB(t)
			defer db.Close()
			dataset := tt.new(t, db)
			var calls int
			dataset.SetSpatialMutationHook(func() { calls++ })

			require.NoError(t, dataset.InsertMany(tt.features))
			assert.Equal(t, len(tt.features), calls)
		})
	}
}

type mutationInsertManyDataset interface {
	InsertMany([]*types.Feature) error
	SetSpatialMutationHook(func())
}

func createMutationTextDataset(t *testing.T, db *sql.DB) *TextDataset {
	t.Helper()
	_, err := db.Exec(`CREATE TABLE labels (
		SmID INTEGER PRIMARY KEY,
		SmUserID INTEGER DEFAULT 0 NOT NULL,
		SmGeometry BLOB,
		SmIndexKey POLYGON,
		name TEXT
	)`)
	require.NoError(t, err)

	record := &system.SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindText),
		SmDatasetName: "labels",
		SmTableName:   "labels",
		SmObjectCount: 0,
		SmGeoColName:  sql.NullString{String: "SmGeometry", Valid: true},
	}
	require.NoError(t, system.NewSmRegisterDao(db).Insert(record))
	require.NoError(t, system.NewSmFieldInfoDao(db).Insert(&system.SmFieldInfoRecord{
		SmDatasetID: record.SmDatasetID,
		SmFieldName: "name",
		SmFieldType: int(types.FieldTypeText),
	}))
	return NewTextDataset(db, record.ToDatasetInfo())
}

func textMutationFeature(id int, x, y float64) *types.Feature {
	return &types.Feature{
		ID:         id,
		Geometry:   textMutationGeometry(x, y),
		Attributes: map[string]interface{}{"name": "label"},
	}
}

func textMutationGeometry(x, y float64) *types.TextGeometry {
	return &types.TextGeometry{Text: "label", Anchor: []float64{x, y}}
}

func cadMutationFeature(id int, x, y float64) *types.Feature {
	return &types.Feature{
		ID:         id,
		Geometry:   &types.CadPointGeometry{XCoord: x, YCoord: y},
		Attributes: map[string]interface{}{"name": "cad"},
	}
}
