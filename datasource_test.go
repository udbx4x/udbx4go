package udbx4go

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func TestCreateAndOpen(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	// Test Create
	ds, err := Create(udbxPath)
	require.NoError(t, err)
	require.NotNil(t, ds)
	defer ds.Close()

	// Verify file was created
	_, err = os.Stat(udbxPath)
	require.NoError(t, err)

	// Close and reopen
	ds.Close()

	// Test Open
	ds2, err := Open(udbxPath)
	require.NoError(t, err)
	require.NotNil(t, ds2)
	defer ds2.Close()
}

func TestOpen_NotUdbxFile(t *testing.T) {
	tempDir := t.TempDir()
	tempFile := filepath.Join(tempDir, "not_udbx.db")

	// Create a plain SQLite file without UDBX tables
	ds, err := Create(tempFile)
	require.NoError(t, err)
	ds.Close()

	// Remove the SmRegister table to make it invalid
	db, err := sql.Open("sqlite3", tempFile)
	require.NoError(t, err)
	_, err = db.Exec("DROP TABLE SmRegister")
	require.NoError(t, err)
	db.Close()

	// Try to open
	ds2, err := Open(tempFile)
	assert.Error(t, err)
	assert.Nil(t, ds2)
	assert.Contains(t, err.Error(), "not a valid UDBX file")
}

func TestDataSource_ListDatasets(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	ds, err := Create(udbxPath)
	require.NoError(t, err)
	defer ds.Close()

	// Initially empty
	datasets, err := ds.ListDatasets()
	require.NoError(t, err)
	assert.Empty(t, datasets)

	// Create some datasets
	_, err = ds.CreatePointDataset("cities", 4326, nil)
	require.NoError(t, err)

	_, err = ds.CreateTabularDataset("countries", nil)
	require.NoError(t, err)

	// List again
	datasets, err = ds.ListDatasets()
	require.NoError(t, err)
	assert.Len(t, datasets, 2)

	// Verify dataset info
	names := make([]string, len(datasets))
	for i, d := range datasets {
		names[i] = d.Name
	}
	assert.Contains(t, names, "cities")
	assert.Contains(t, names, "countries")
}

func TestDataSource_CreateTabularDataset(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	ds, err := Create(udbxPath)
	require.NoError(t, err)
	defer ds.Close()

	fields := []*types.FieldInfo{
		{Name: "name", FieldType: types.FieldTypeText, Nullable: false},
		{Name: "population", FieldType: types.FieldTypeInt32, Nullable: true},
	}

	tabular, err := ds.CreateTabularDataset("countries", fields)
	require.NoError(t, err)
	require.NotNil(t, tabular)

	// Verify dataset info
	assert.Equal(t, "countries", tabular.Info().Name)
	assert.Equal(t, types.DatasetKindTabular, tabular.Info().Kind)

	// Get fields
	retrievedFields, err := tabular.GetFields()
	require.NoError(t, err)
	assert.Len(t, retrievedFields, 2)
}

func TestDataSource_CreatePointDataset(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	ds, err := Create(udbxPath)
	require.NoError(t, err)
	defer ds.Close()

	fields := []*types.FieldInfo{
		{Name: "name", FieldType: types.FieldTypeText, Nullable: true},
	}

	pointDS, err := ds.CreatePointDataset("cities", 4326, fields)
	require.NoError(t, err)
	require.NotNil(t, pointDS)

	// Verify dataset info
	assert.Equal(t, "cities", pointDS.Info().Name)
	assert.Equal(t, types.DatasetKindPoint, pointDS.Info().Kind)
	assert.Equal(t, 4326, pointDS.SRID())
}

func TestDataSource_CreateDuplicateDataset(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	ds, err := Create(udbxPath)
	require.NoError(t, err)
	defer ds.Close()

	_, err = ds.CreatePointDataset("cities", 4326, nil)
	require.NoError(t, err)

	// Try to create again with same name
	_, err = ds.CreatePointDataset("cities", 4326, nil)
	assert.Error(t, err)
	assert.True(t, IsConstraintViolation(err))
}

func TestDataSource_GetDataset(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	ds, err := Create(udbxPath)
	require.NoError(t, err)
	defer ds.Close()

	// Create datasets
	_, err = ds.CreateTabularDataset("countries", nil)
	require.NoError(t, err)

	_, err = ds.CreatePointDataset("cities", 4326, nil)
	require.NoError(t, err)

	_, err = ds.CreateLineDataset("roads", 4326, nil)
	require.NoError(t, err)

	_, err = ds.CreateRegionDataset("regions", 4326, nil)
	require.NoError(t, err)

	// Get each dataset
	tabular, err := ds.GetTabularDataset("countries")
	require.NoError(t, err)
	assert.NotNil(t, tabular)

	point, err := ds.GetPointDataset("cities")
	require.NoError(t, err)
	assert.NotNil(t, point)

	line, err := ds.GetLineDataset("roads")
	require.NoError(t, err)
	assert.NotNil(t, line)

	region, err := ds.GetRegionDataset("regions")
	require.NoError(t, err)
	assert.NotNil(t, region)
}

func TestDataSource_GetDataset_NotFound(t *testing.T) {
	tempDir := t.TempDir()
	udbxPath := filepath.Join(tempDir, "test.udbx")

	ds, err := Create(udbxPath)
	require.NoError(t, err)
	defer ds.Close()

	// Try to get non-existent dataset
	_, err = ds.GetDataset("nonexistent")
	assert.Error(t, err)
	assert.True(t, IsNotFound(err))
}
