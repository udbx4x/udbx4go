package system

import (
	"context"
	"database/sql"
	stderrors "errors"
	"math"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/internal/schema"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func setupTestDB(t *testing.T) *sql.DB {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := sql.Open("sqlite3", dbPath)
	require.NoError(t, err)

	initializer := schema.NewInitializer(db)
	err = initializer.Initialize()
	require.NoError(t, err)

	return db
}

func TestSmRegisterDao_Insert(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmRegisterDao(db)

	record := &SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindPoint),
		SmDatasetName: "cities",
		SmTableName:   "cities",
		SmObjectCount: 0,
	}

	err := dao.Insert(record)
	require.NoError(t, err)
	assert.Greater(t, record.SmDatasetID, 0)
}

func TestSmRegisterDao_GetByName(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmRegisterDao(db)

	// Insert a record
	record := &SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindPoint),
		SmDatasetName: "cities",
		SmTableName:   "cities",
		SmObjectCount: 10,
	}
	err := dao.Insert(record)
	require.NoError(t, err)

	// Get by name
	retrieved, err := dao.GetByName("cities")
	require.NoError(t, err)
	assert.Equal(t, "cities", retrieved.SmDatasetName)
	assert.Equal(t, 10, retrieved.SmObjectCount)
}

func TestSmRegisterDao_GetByName_NotFound(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmRegisterDao(db)

	_, err := dao.GetByName("nonexistent")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestSmRegisterDao_GetByNameContextHonorsCancellation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewSmRegisterDao(db).GetByNameContext(ctx, "cities")
	require.Error(t, err)
	assert.True(t, stderrors.Is(err, context.Canceled))
}

func TestSmRegisterDao_ListAll(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmRegisterDao(db)

	// Insert multiple records
	records := []*SmRegisterRecord{
		{SmDatasetType: int(types.DatasetKindPoint), SmDatasetName: "cities", SmTableName: "cities"},
		{SmDatasetType: int(types.DatasetKindLine), SmDatasetName: "roads", SmTableName: "roads"},
		{SmDatasetType: int(types.DatasetKindTabular), SmDatasetName: "countries", SmTableName: "countries"},
	}

	for _, r := range records {
		err := dao.Insert(r)
		require.NoError(t, err)
	}

	// List all
	all, err := dao.ListAll()
	require.NoError(t, err)
	assert.Len(t, all, 3)
}

func TestSmRegisterDao_Exists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmRegisterDao(db)

	// Insert a record
	record := &SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindPoint),
		SmDatasetName: "cities",
		SmTableName:   "cities",
	}
	err := dao.Insert(record)
	require.NoError(t, err)

	// Check existence
	exists, err := dao.Exists("cities")
	require.NoError(t, err)
	assert.True(t, exists)

	exists, err = dao.Exists("nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestSmRegisterDao_UpdateObjectCount(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmRegisterDao(db)

	record := &SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindPoint),
		SmDatasetName: "cities",
		SmTableName:   "cities",
		SmObjectCount: 0,
	}
	err := dao.Insert(record)
	require.NoError(t, err)

	// Update count
	err = dao.UpdateObjectCount(record.SmDatasetID, 100)
	require.NoError(t, err)

	// Verify
	retrieved, err := dao.GetByName("cities")
	require.NoError(t, err)
	assert.Equal(t, 100, retrieved.SmObjectCount)
}

func TestSmRegisterDao_UpdateBounds(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmRegisterDao(db)

	record := &SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindPoint),
		SmDatasetName: "cities",
		SmTableName:   "cities",
	}
	err := dao.Insert(record)
	require.NoError(t, err)

	// Update bounds (minX, minY, maxX, maxY)
	err = dao.UpdateBounds(record.SmDatasetID, 116.0, 39.0, 117.0, 40.0)
	require.NoError(t, err)

	// Verify - Java UDBX uses SmLeft, SmRight, SmTop, SmBottom
	retrieved, err := dao.GetByName("cities")
	require.NoError(t, err)
	assert.InDelta(t, 116.0, retrieved.SmLeft.Float64, 0.0001)
	assert.InDelta(t, 39.0, retrieved.SmBottom.Float64, 0.0001)
	assert.InDelta(t, 117.0, retrieved.SmRight.Float64, 0.0001)
	assert.InDelta(t, 40.0, retrieved.SmTop.Float64, 0.0001)
}

func TestSmRegisterDao_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	dao := NewSmRegisterDao(db)

	record := &SmRegisterRecord{
		SmDatasetType: int(types.DatasetKindPoint),
		SmDatasetName: "cities",
		SmTableName:   "cities",
	}
	err := dao.Insert(record)
	require.NoError(t, err)

	// Delete
	err = dao.Delete(record.SmDatasetID)
	require.NoError(t, err)

	// Verify deletion
	_, err = dao.GetByName("cities")
	require.Error(t, err)
}

func TestSmRegisterRecord_ToDatasetInfo(t *testing.T) {
	record := &SmRegisterRecord{
		SmDatasetID:   1,
		SmDatasetType: int(types.DatasetKindPoint),
		SmDatasetName: "cities",
		SmTableName:   "cities_table",
		SmObjectCount: 10,
		SmSRID:        sql.NullInt32{Int32: 4326, Valid: true},
	}

	info := record.ToDatasetInfo()

	assert.Equal(t, 1, info.ID)
	assert.Equal(t, "cities", info.Name)
	assert.Equal(t, "cities_table", info.TableName)
	assert.Equal(t, types.DatasetKindPoint, info.Kind)
	assert.Equal(t, 10, info.ObjectCount)
	require.NotNil(t, info.SRID)
	assert.Equal(t, 4326, *info.SRID)
	require.NotNil(t, info.GeometryType)
	assert.Equal(t, 1, *info.GeometryType)
}

func TestSmRegisterRecord_ToDatasetInfo_NoSRID(t *testing.T) {
	record := &SmRegisterRecord{
		SmDatasetID:   1,
		SmDatasetType: int(types.DatasetKindTabular),
		SmDatasetName: "countries",
		SmTableName:   "countries_table",
		SmObjectCount: 5,
		SmSRID:        sql.NullInt32{},
	}

	info := record.ToDatasetInfo()

	assert.Nil(t, info.SRID)
	assert.Nil(t, info.GeometryType) // Tabular has no geometry
}

func TestDatasetInfoExtentMapsOnlyValidDeclaredBounds(t *testing.T) {
	valid := func(value float64) sql.NullFloat64 {
		return sql.NullFloat64{Float64: value, Valid: true}
	}

	tests := []struct {
		name   string
		kind   types.DatasetKind
		left   sql.NullFloat64
		bottom sql.NullFloat64
		right  sql.NullFloat64
		top    sql.NullFloat64
		want   *types.BoundingBox
	}{
		{
			name:   "normal extent",
			kind:   types.DatasetKindPoint,
			left:   valid(-10),
			bottom: valid(-20),
			right:  valid(30),
			top:    valid(40),
			want:   &types.BoundingBox{MinX: -10, MinY: -20, MaxX: 30, MaxY: 40},
		},
		{
			name:   "zero area extent",
			kind:   types.DatasetKindPoint,
			left:   valid(5),
			bottom: valid(6),
			right:  valid(5),
			top:    valid(6),
			want:   &types.BoundingBox{MinX: 5, MinY: 6, MaxX: 5, MaxY: 6},
		},
		{name: "tabular declared zero extent", kind: types.DatasetKindTabular, left: valid(0), bottom: valid(0), right: valid(0), top: valid(0)},
		{name: "null left", kind: types.DatasetKindPoint, bottom: valid(0), right: valid(1), top: valid(1)},
		{name: "null bottom", kind: types.DatasetKindPoint, left: valid(0), right: valid(1), top: valid(1)},
		{name: "null right", kind: types.DatasetKindPoint, left: valid(0), bottom: valid(0), top: valid(1)},
		{name: "null top", kind: types.DatasetKindPoint, left: valid(0), bottom: valid(0), right: valid(1)},
		{name: "nan", kind: types.DatasetKindPoint, left: valid(math.NaN()), bottom: valid(0), right: valid(1), top: valid(1)},
		{name: "positive infinity", kind: types.DatasetKindPoint, left: valid(0), bottom: valid(math.Inf(1)), right: valid(1), top: valid(1)},
		{name: "negative infinity", kind: types.DatasetKindPoint, left: valid(0), bottom: valid(0), right: valid(math.Inf(-1)), top: valid(1)},
		{name: "inverted x", kind: types.DatasetKindPoint, left: valid(2), bottom: valid(0), right: valid(1), top: valid(1)},
		{name: "inverted y", kind: types.DatasetKindPoint, left: valid(0), bottom: valid(2), right: valid(1), top: valid(1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := &SmRegisterRecord{
				SmDatasetType: int(tt.kind),
				SmLeft:        tt.left,
				SmBottom:      tt.bottom,
				SmRight:       tt.right,
				SmTop:         tt.top,
			}

			info := record.ToDatasetInfo()
			assert.Equal(t, tt.want, info.Extent)
		})
	}
}
