package udbx4go_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/udbx4x/udbx4go"
)

func TestPublicFeatureChangesCanUpdatePointDataset(t *testing.T) {
	path := filepath.Join(t.TempDir(), "public-api.udbx")

	ds, err := udbx4go.Create(path)
	require.NoError(t, err)
	defer ds.Close()

	points, err := ds.CreatePointDataset("cities", 4326, []*udbx4go.FieldInfo{
		{Name: "name", FieldType: udbx4go.FieldTypeText, Nullable: true},
		{Name: "population", FieldType: udbx4go.FieldTypeInt32, Nullable: true},
	})
	require.NoError(t, err)

	err = points.Insert(&udbx4go.Feature{
		ID: 1,
		Geometry: &udbx4go.PointGeometry{
			Type:        "Point",
			Coordinates: []float64{116.4, 39.9},
		},
		Attributes: map[string]interface{}{
			"name":       "Beijing",
			"population": 21540000,
		},
	})
	require.NoError(t, err)

	err = points.Update(1, &udbx4go.FeatureChanges{
		Geometry: &udbx4go.PointGeometry{
			Type:        "Point",
			Coordinates: []float64{121.5, 31.2},
		},
		Attributes: map[string]interface{}{
			"population": 26320000,
		},
	})
	require.NoError(t, err)

	feature, err := points.GetByID(1)
	require.NoError(t, err)
	require.Equal(t, int64(26320000), feature.Attributes["population"])

	point, ok := feature.Geometry.(*udbx4go.PointGeometry)
	require.True(t, ok)
	require.InDelta(t, 121.5, point.Coordinates[0], 0.000001)
	require.InDelta(t, 31.2, point.Coordinates[1], 0.000001)
}
