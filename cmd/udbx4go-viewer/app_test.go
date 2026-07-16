package main

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	udbx4go "github.com/udbx4x/udbx4go"
	"github.com/udbx4x/udbx4go/pkg/types"
)

func sampleDataPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "data", "SampleData.udbx"))
	if err != nil {
		t.Fatalf("resolve sample path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("external fixture not available: %s", path)
		}
		t.Fatalf("stat sample path: %v", err)
	}
	return path
}

func henanDataPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "data", "henan.udbx"))
	if err != nil {
		t.Fatalf("resolve henan path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			t.Skipf("external fixture not available: %s", path)
		}
		t.Fatalf("stat henan path: %v", err)
	}
	return path
}

func TestViewerOpensSampleDataAndPagesDataset(t *testing.T) {
	app := NewApp()
	info, err := app.OpenUDBXFile(sampleDataPath(t))
	if err != nil {
		t.Fatalf("OpenUDBXFile() error = %v", err)
	}
	if info.DatasetCount == 0 {
		t.Fatal("OpenUDBXFile() returned zero datasets")
	}

	datasets, err := app.ListDatasets()
	if err != nil {
		t.Fatalf("ListDatasets() error = %v", err)
	}
	names := map[string]bool{}
	for _, dataset := range datasets {
		names[dataset.Name] = true
	}
	for _, expected := range []string{"BaseMap_P", "County_T", "CADDT"} {
		if !names[expected] {
			t.Fatalf("ListDatasets() missing %s in %v", expected, names)
		}
	}

	page, err := app.LoadDatasetPage("BaseMap_P", 1)
	if err != nil {
		t.Fatalf("LoadDatasetPage() error = %v", err)
	}
	if len(page.Rows) == 0 {
		t.Fatal("LoadDatasetPage() returned no rows")
	}
	if len(page.Columns) < 2 || page.Columns[0] != "SmID" || page.Columns[1] != "Geometry" {
		t.Fatalf("LoadDatasetPage() columns = %v", page.Columns)
	}

	if err := app.CloseUDBXFile(); err != nil {
		t.Fatalf("CloseUDBXFile() error = %v", err)
	}
	if _, err := app.ListDatasets(); err == nil || !strings.Contains(err.Error(), "没有打开的文件") {
		t.Fatalf("ListDatasets() after close error = %v", err)
	}
}

func TestViewerLoadsSampleCADDTAsAttributeTable(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	page, err := app.LoadDatasetPage("CADDT", 1)
	if err != nil {
		t.Fatalf("LoadDatasetPage(CADDT) error = %v", err)
	}
	if page == nil {
		t.Fatal("LoadDatasetPage(CADDT) returned nil page")
	}
	if len(page.Rows) == 0 {
		ds, getErr := app.dataSource.GetDataset("CADDT")
		if getErr != nil {
			t.Fatalf("LoadDatasetPage(CADDT) returned no rows; GetDataset(CADDT) error = %v", getErr)
		}
		vectorDs, ok := ds.(interface {
			List(opts *types.QueryOptions) ([]*types.Feature, error)
		})
		if !ok {
			t.Fatalf("LoadDatasetPage(CADDT) returned no rows; dataset does not implement feature List")
		}
		features, listErr := vectorDs.List(&types.QueryOptions{Limit: pageSize})
		t.Fatalf("LoadDatasetPage(CADDT) returned no rows; direct List returned %d features, error = %v", len(features), listErr)
	}
	if len(page.Columns) < 2 || page.Columns[0] != "SmID" || page.Columns[1] != "Geometry" {
		t.Fatalf("LoadDatasetPage(CADDT) columns = %v", page.Columns)
	}
	for rowIndex, row := range page.Rows {
		if len(row) != len(page.Columns) {
			t.Fatalf("CADDT row %d has %d cells, want %d columns: %q", rowIndex, len(row), len(page.Columns), row)
		}
		if row[0] == "" {
			t.Fatalf("CADDT row %d has empty SmID: %q", rowIndex, row)
		}
	}
}

func TestViewerSampleDataDatasetHandlingMatrix(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	datasets, err := app.ListDatasets()
	if err != nil {
		t.Fatalf("ListDatasets() error = %v", err)
	}
	if len(datasets) == 0 {
		t.Fatal("ListDatasets() returned no datasets")
	}

	expectedUnsupported := map[string]bool{
		"Jingjin_Network":  true,
		"Jingjin_NetworkZ": true,
		"modeldt":          true,
		"modeldt_Texture":  true,
	}
	seen := map[string]bool{}
	for _, dataset := range datasets {
		seen[dataset.Name] = true
		page, pageErr := app.LoadDatasetPage(dataset.Name, 1)
		if expectedUnsupported[dataset.Name] {
			if pageErr == nil {
				t.Fatalf("LoadDatasetPage(%s) error = nil, want unsupported dataset error", dataset.Name)
			}
			if !strings.Contains(pageErr.Error(), "not supported") {
				t.Fatalf("LoadDatasetPage(%s) error = %v, want not supported", dataset.Name, pageErr)
			}
			continue
		}

		if pageErr != nil {
			t.Fatalf("LoadDatasetPage(%s) unexpected error = %v", dataset.Name, pageErr)
		}
		if page == nil {
			t.Fatalf("LoadDatasetPage(%s) returned nil page", dataset.Name)
		}
		if len(page.Columns) == 0 {
			t.Fatalf("LoadDatasetPage(%s) returned no columns", dataset.Name)
		}
		if dataset.ObjectCount > 0 && len(page.Rows) == 0 {
			t.Fatalf("LoadDatasetPage(%s) returned no rows for %d objects", dataset.Name, dataset.ObjectCount)
		}
		for rowIndex, row := range page.Rows {
			if len(row) != len(page.Columns) {
				t.Fatalf("LoadDatasetPage(%s) row %d has %d cells, want %d columns", dataset.Name, rowIndex, len(row), len(page.Columns))
			}
		}
	}

	for name := range expectedUnsupported {
		if !seen[name] {
			t.Fatalf("SampleData.udbx missing expected unsupported dataset %s", name)
		}
	}
}

func TestViewerDatasetIconsRecognizeTextAndCAD(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	datasets, err := app.ListDatasets()
	if err != nil {
		t.Fatalf("ListDatasets() error = %v", err)
	}
	icons := make(map[string]string, len(datasets))
	for _, dataset := range datasets {
		icons[dataset.Name] = dataset.IconType
	}
	if icons["County_T"] != "text" {
		t.Fatalf("County_T iconType = %q, want text", icons["County_T"])
	}
	if icons["CADDT"] != "cad" {
		t.Fatalf("CADDT iconType = %q, want cad", icons["CADDT"])
	}
}

func TestViewerHandlesOpenErrorsAndPageBounds(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	if _, err := app.OpenUDBXFile(filepath.Join(t.TempDir(), "missing.udbx")); err == nil {
		t.Fatal("OpenUDBXFile(missing) expected error")
	}
	if _, err := app.ListDatasets(); err == nil || !strings.Contains(err.Error(), "没有打开的文件") {
		t.Fatalf("ListDatasets() after failed open error = %v", err)
	}

	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample again) error = %v", err)
	}
	firstPage, err := app.LoadDatasetPage("BaseMap_P", -10)
	if err != nil {
		t.Fatalf("LoadDatasetPage(negative) error = %v", err)
	}
	if firstPage.CurrentPage != 1 {
		t.Fatalf("LoadDatasetPage(negative) CurrentPage = %d, want 1", firstPage.CurrentPage)
	}

	lastPage, err := app.LoadDatasetPage("BaseMap_P", 999)
	if err != nil {
		t.Fatalf("LoadDatasetPage(large) error = %v", err)
	}
	if lastPage.CurrentPage != lastPage.TotalPages {
		t.Fatalf("LoadDatasetPage(large) CurrentPage = %d, want %d", lastPage.CurrentPage, lastPage.TotalPages)
	}
}

func TestViewerSpatialSummaryAndPreviewUseRendererNeutralContract(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	summary, err := app.GetDatasetSpatialSummary("BaseMap_P")
	if err != nil {
		t.Fatalf("GetDatasetSpatialSummary() error = %v", err)
	}
	if !summary.PreviewSupported {
		t.Fatalf("PreviewSupported = false, reason = %q", summary.UnsupportedReason)
	}
	if summary.Kind != "point" {
		t.Fatalf("Kind = %q, want point", summary.Kind)
	}
	if summary.ObjectCount <= 0 {
		t.Fatalf("ObjectCount = %d, want positive", summary.ObjectCount)
	}
	if summary.Extent == nil {
		t.Fatal("Extent = nil, want dataset extent")
	}

	preview, err := app.LoadSpatialPreview("BaseMap_P", SpatialPreviewRequestDTO{Limit: 3, MaxVertices: 10})
	if err != nil {
		t.Fatalf("LoadSpatialPreview() error = %v", err)
	}
	if preview.Sampled {
		t.Fatal("Sampled = true for small limit preview")
	}
	wantFeatureCount := summary.ObjectCount
	if wantFeatureCount > minSpatialPreviewFeatureLimit {
		wantFeatureCount = minSpatialPreviewFeatureLimit
	}
	if len(preview.Features) != wantFeatureCount {
		t.Fatalf("len(Features) = %d, want clamped limit count %d", len(preview.Features), wantFeatureCount)
	}
	first := preview.Features[0]
	if first.ID == 0 {
		t.Fatal("first feature ID = 0, want SmID")
	}
	if first.Geometry.Type != "Point" {
		t.Fatalf("first geometry type = %q, want Point", first.Geometry.Type)
	}
	if len(first.Geometry.Coordinates) == 0 {
		t.Fatal("first geometry coordinates are empty")
	}
}

func TestViewerSpatialPreviewReportsUnsupportedTabularDataset(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	summary, err := app.GetDatasetSpatialSummary("TabularDT")
	if err != nil {
		t.Fatalf("GetDatasetSpatialSummary(tabular) error = %v", err)
	}
	if summary.PreviewSupported {
		t.Fatal("PreviewSupported = true for tabular dataset")
	}
	if !strings.Contains(summary.UnsupportedReason, "非空间数据集") {
		t.Fatalf("UnsupportedReason = %q, want 非空间数据集", summary.UnsupportedReason)
	}

	if _, err := app.LoadSpatialPreview("TabularDT", SpatialPreviewRequestDTO{Limit: 3}); err == nil || !strings.Contains(err.Error(), "不支持空间预览") {
		t.Fatalf("LoadSpatialPreview(tabular) error = %v", err)
	}
}

func TestViewerSpatialPreviewSupportsPointLineAndRegionDatasets(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	cases := []struct {
		name         string
		wantGeomType string
		maxVertices  int
	}{
		{name: "BaseMap_P", wantGeomType: "Point", maxVertices: 10},
		{name: "BaseMap_L", wantGeomType: "MultiLineString", maxVertices: 100},
		{name: "BaseMap_R", wantGeomType: "MultiPolygon", maxVertices: 200},
	}

	for _, tc := range cases {
		preview, err := app.LoadSpatialPreview(tc.name, SpatialPreviewRequestDTO{Limit: 2, MaxVertices: tc.maxVertices})
		if err != nil {
			t.Fatalf("LoadSpatialPreview(%s) error = %v", tc.name, err)
		}
		if len(preview.Features) == 0 {
			t.Fatalf("LoadSpatialPreview(%s) returned no features", tc.name)
		}
		if preview.Features[0].Geometry.Type != tc.wantGeomType {
			t.Fatalf("%s geometry type = %q, want %q", tc.name, preview.Features[0].Geometry.Type, tc.wantGeomType)
		}
	}
}

func TestViewerSpatialPreviewKeepsLineAndRegionFeatureExtents(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	cases := []string{"BaseMap_L", "BaseMap_R"}
	for _, datasetName := range cases {
		preview, err := app.LoadSpatialPreview(datasetName, SpatialPreviewRequestDTO{
			Limit:       1,
			MaxVertices: maxSpatialPreviewVertexBudget,
		})
		if err != nil {
			t.Fatalf("LoadSpatialPreview(%s) error = %v", datasetName, err)
		}
		if len(preview.Features) == 0 {
			t.Fatalf("LoadSpatialPreview(%s) returned no features", datasetName)
		}
		bbox := preview.Features[0].BBox
		if bbox == nil {
			t.Fatalf("LoadSpatialPreview(%s) feature bbox = nil", datasetName)
		}
		if bbox.MinX == bbox.MaxX && bbox.MinY == bbox.MaxY {
			t.Fatalf("LoadSpatialPreview(%s) feature bbox collapsed to point: %+v", datasetName, bbox)
		}
	}
}

func TestViewerSpatialPreviewUsesSavedSettingsWhenRequestOmitsLimits(t *testing.T) {
	app := NewApp()
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")

	settings := DefaultViewerSettings()
	settings.SpatialPreview.FeatureLimit = minSpatialPreviewFeatureLimit
	settings.SpatialPreview.VertexBudget = minSpatialPreviewVertexBudget
	if _, err := app.SaveViewerSettings(settings); err != nil {
		t.Fatalf("SaveViewerSettings() error = %v", err)
	}
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	preview, err := app.LoadSpatialPreview("Jingjin_Network_Node", SpatialPreviewRequestDTO{})
	if err != nil {
		t.Fatalf("LoadSpatialPreview() error = %v", err)
	}
	if len(preview.Features) != minSpatialPreviewFeatureLimit {
		t.Fatalf("len(preview.Features) = %d, want settings feature limit %d", len(preview.Features), minSpatialPreviewFeatureLimit)
	}
}

func TestViewerSpatialPreviewUsesExplicitRequestLimitsBeforeSettings(t *testing.T) {
	app := NewApp()
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")

	settings := DefaultViewerSettings()
	settings.SpatialPreview.FeatureLimit = 1000
	settings.SpatialPreview.VertexBudget = maxSpatialPreviewVertexBudget
	if _, err := app.SaveViewerSettings(settings); err != nil {
		t.Fatalf("SaveViewerSettings() error = %v", err)
	}
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	const requestLimit = 200
	preview, err := app.LoadSpatialPreview("Jingjin_Network_Node", SpatialPreviewRequestDTO{
		Limit:       requestLimit,
		MaxVertices: minSpatialPreviewVertexBudget,
	})
	if err != nil {
		t.Fatalf("LoadSpatialPreview() error = %v", err)
	}
	if len(preview.Features) != requestLimit {
		t.Fatalf("len(preview.Features) = %d, want explicit request limit %d", len(preview.Features), requestLimit)
	}
}

func TestViewerSpatialPreviewMarksSampledWhenFeatureLimitIsReached(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	preview, err := app.LoadSpatialPreview("Jingjin_Network_Node", SpatialPreviewRequestDTO{
		Limit:       minSpatialPreviewFeatureLimit,
		MaxVertices: maxSpatialPreviewVertexBudget,
	})
	if err != nil {
		t.Fatalf("LoadSpatialPreview() error = %v", err)
	}
	if !preview.Sampled {
		t.Fatal("Sampled = false, want true when preview reaches feature limit")
	}
	if !strings.Contains(preview.SampleReason, "要素上限") {
		t.Fatalf("SampleReason = %q, want feature limit reason", preview.SampleReason)
	}
}

func TestViewerSpatialPreviewClampsExplicitRequestLimits(t *testing.T) {
	app := NewApp()
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")

	settings := DefaultViewerSettings()
	settings.SpatialPreview.FeatureLimit = 1000
	settings.SpatialPreview.VertexBudget = maxSpatialPreviewVertexBudget
	if _, err := app.SaveViewerSettings(settings); err != nil {
		t.Fatalf("SaveViewerSettings() error = %v", err)
	}
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	preview, err := app.LoadSpatialPreview("Jingjin_Network_Node", SpatialPreviewRequestDTO{
		Limit:       1,
		MaxVertices: 1,
	})
	if err != nil {
		t.Fatalf("LoadSpatialPreview() error = %v", err)
	}
	if len(preview.Features) != minSpatialPreviewFeatureLimit {
		t.Fatalf("len(preview.Features) = %d, want clamped request limit %d", len(preview.Features), minSpatialPreviewFeatureLimit)
	}
}

func TestViewerSpatialPreviewSampleReasonCombinesFeatureAndVertexLimits(t *testing.T) {
	sampled, reason := spatialPreviewSampleReason(200, 100, 100, true)

	if !sampled {
		t.Fatal("sampled = false, want true")
	}
	if !strings.Contains(reason, "要素上限") {
		t.Fatalf("reason = %q, want feature limit reason", reason)
	}
	if !strings.Contains(reason, "顶点上限") {
		t.Fatalf("reason = %q, want vertex budget reason", reason)
	}
}

func TestFormatPreviewFeaturesReportsVertexBudgetReached(t *testing.T) {
	app := NewApp()
	features := []*types.Feature{
		{
			ID: 1,
			Geometry: &types.MultiLineStringGeometry{
				Coordinates: [][][]float64{
					{
						{0, 0},
						{1, 1},
					},
				},
			},
		},
	}

	previewFeatures, vertexBudgetReached := app.formatPreviewFeatures(features, nil, 1, nil)

	if len(previewFeatures) != 0 {
		t.Fatalf("len(previewFeatures) = %d, want 0", len(previewFeatures))
	}
	if !vertexBudgetReached {
		t.Fatal("vertexBudgetReached = false, want true")
	}
}

func TestViewerSpatialViewportPassesBoundsToSDKAndKeepsRequiredID(t *testing.T) {
	path, _ := createViewerPointFixture(t, true, 0)
	app := NewApp()
	if _, err := app.OpenUDBXFile(path); err != nil {
		t.Fatalf("OpenUDBXFile() error = %v", err)
	}

	viewport := &BoundingBoxDTO{MinX: 0, MinY: 0, MaxX: 5, MaxY: 5}
	preview, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{
		Viewport:    viewport,
		Limit:       100,
		MaxVertices: minSpatialPreviewVertexBudget,
		RequiredIDs: []int{2},
	})
	if err != nil {
		t.Fatalf("LoadSpatialPreview() error = %v", err)
	}

	if preview.QueriedBounds == nil || *preview.QueriedBounds != *viewport {
		t.Fatalf("QueriedBounds = %+v, want %+v", preview.QueriedBounds, viewport)
	}
	if preview.Strategy != string(types.SpatialQueryStrategyRTree) {
		t.Fatalf("Strategy = %q, want rtree", preview.Strategy)
	}
	if preview.HasMore {
		t.Fatal("HasMore = true, want false")
	}
	if preview.DegradedReason != "" {
		t.Fatalf("DegradedReason = %q, want empty", preview.DegradedReason)
	}
	if preview.QueryDurationMS < 0 {
		t.Fatalf("QueryDurationMS = %f, want non-negative", preview.QueryDurationMS)
	}
	if preview.FileGeneration == 0 {
		t.Fatal("FileGeneration = 0, want current file generation")
	}

	ids := make(map[int]bool, len(preview.Features))
	for _, feature := range preview.Features {
		ids[feature.ID] = true
	}
	if !ids[1] || !ids[2] {
		t.Fatalf("feature IDs = %v, want viewport ID 1 and required outside ID 2", ids)
	}
	if ids[3] {
		t.Fatalf("feature IDs = %v, outside non-required ID 3 must be filtered", ids)
	}
}

func TestViewerSpatialViewportRejectsInvalidBoundsWithStableReason(t *testing.T) {
	path, _ := createViewerPointFixture(t, false, 0)
	app := NewApp()
	if _, err := app.OpenUDBXFile(path); err != nil {
		t.Fatalf("OpenUDBXFile() error = %v", err)
	}

	_, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{
		Viewport: &BoundingBoxDTO{MinX: 10, MinY: 0, MaxX: 1, MaxY: 5},
		Limit:    100,
	})
	if err == nil || !strings.Contains(err.Error(), string(types.SpatialQueryReasonInvalidViewport)) {
		t.Fatalf("LoadSpatialPreview(invalid viewport) error = %v, want invalid_viewport", err)
	}
}

func TestViewerSpatialDeclaredExtentComesFromSmRegisterWithoutReadingGeometry(t *testing.T) {
	path, tableName := createViewerPointFixture(t, false, 0)
	db := openViewerFixtureDB(t, path)
	if _, err := db.Exec(fmt.Sprintf(`UPDATE %q SET SmGeometry = x'00'`, tableName)); err != nil {
		t.Fatalf("corrupt fixture geometry: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture database: %v", err)
	}

	app := NewApp()
	if _, err := app.OpenUDBXFile(path); err != nil {
		t.Fatalf("OpenUDBXFile() error = %v", err)
	}
	summary, err := app.GetDatasetSpatialSummary("viewer_points")
	if err != nil {
		t.Fatalf("GetDatasetSpatialSummary() error = %v", err)
	}
	want := BoundingBoxDTO{MinX: -10, MinY: -20, MaxX: 30, MaxY: 40}
	if summary.Extent == nil || *summary.Extent != want {
		t.Fatalf("Extent = %+v, want declared %+v", summary.Extent, want)
	}
	if summary.EstimatedVertexCount != 0 {
		t.Fatalf("EstimatedVertexCount = %d, want 0 without geometry sampling", summary.EstimatedVertexCount)
	}
}

func TestViewerSpatialReasonMapsRTreeCacheAndBoundedSampleStates(t *testing.T) {
	t.Run("rtree capability", func(t *testing.T) {
		path, _ := createViewerPointFixture(t, true, 0)
		app := NewApp()
		if _, err := app.OpenUDBXFile(path); err != nil {
			t.Fatalf("OpenUDBXFile() error = %v", err)
		}
		summary, err := app.GetDatasetSpatialSummary("viewer_points")
		if err != nil {
			t.Fatalf("GetDatasetSpatialSummary() error = %v", err)
		}
		if !summary.ViewportQuerySupported || !summary.RTreeAvailable || summary.QueryDiagnosticReason != "" {
			t.Fatalf("summary query capability = %+v", summary)
		}
	})

	t.Run("envelope cache", func(t *testing.T) {
		path, _ := createViewerPointFixture(t, false, 0)
		app := NewApp()
		if _, err := app.OpenUDBXFile(path); err != nil {
			t.Fatalf("OpenUDBXFile() error = %v", err)
		}
		summary, err := app.GetDatasetSpatialSummary("viewer_points")
		if err != nil {
			t.Fatalf("GetDatasetSpatialSummary() error = %v", err)
		}
		if !summary.ViewportQuerySupported || summary.RTreeAvailable {
			t.Fatalf("summary query capability = %+v", summary)
		}
		if summary.QueryDiagnosticReason != string(types.SpatialQueryReasonSpatialIndexUnavailable) {
			t.Fatalf("QueryDiagnosticReason = %q, want spatial_index_unavailable", summary.QueryDiagnosticReason)
		}

		preview, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{
			Viewport: &BoundingBoxDTO{MinX: 0, MinY: 0, MaxX: 5, MaxY: 5},
			Limit:    100,
		})
		if err != nil {
			t.Fatalf("LoadSpatialPreview() error = %v", err)
		}
		if preview.Strategy != string(types.SpatialQueryStrategyEnvelopeCache) || preview.DegradedReason != "" {
			t.Fatalf("preview strategy = %q, reason = %q", preview.Strategy, preview.DegradedReason)
		}
	})

	t.Run("bounded sample", func(t *testing.T) {
		path, _ := createViewerPointFixture(t, false, 1_000_000)
		app := NewApp()
		if _, err := app.OpenUDBXFile(path); err != nil {
			t.Fatalf("OpenUDBXFile() error = %v", err)
		}
		preview, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{
			Viewport: &BoundingBoxDTO{MinX: 0, MinY: 0, MaxX: 5, MaxY: 5},
			Limit:    100,
		})
		if err != nil {
			t.Fatalf("LoadSpatialPreview() error = %v", err)
		}
		if preview.Strategy != string(types.SpatialQueryStrategyBoundedSample) {
			t.Fatalf("Strategy = %q, want bounded_sample", preview.Strategy)
		}
		if preview.DegradedReason != string(types.SpatialQueryReasonEnvelopeCacheBudgetExceeded) {
			t.Fatalf("DegradedReason = %q, want envelope_cache_budget_exceeded", preview.DegradedReason)
		}
		if !preview.HasMore {
			t.Fatal("HasMore = false, want SDK bounded sample result")
		}
	})
}

func TestViewerSpatialReasonTextAndCADIgnoreViewportQuery(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	invalidViewport := &BoundingBoxDTO{MinX: 10, MinY: 10, MaxX: -10, MaxY: -10}
	for _, datasetName := range []string{"County_T", "CADDT"} {
		t.Run(datasetName, func(t *testing.T) {
			summary, err := app.GetDatasetSpatialSummary(datasetName)
			if err != nil {
				t.Fatalf("GetDatasetSpatialSummary(%s) error = %v", datasetName, err)
			}
			if !summary.PreviewSupported || summary.ViewportQuerySupported || summary.RTreeAvailable {
				t.Fatalf("summary(%s) = %+v", datasetName, summary)
			}
			if summary.QueryDiagnosticReason != string(types.SpatialQueryReasonUnsupportedDatasetKind) {
				t.Fatalf("QueryDiagnosticReason = %q, want unsupported_dataset_kind", summary.QueryDiagnosticReason)
			}

			preview, err := app.LoadSpatialPreview(datasetName, SpatialPreviewRequestDTO{
				Viewport: invalidViewport,
				Limit:    100,
			})
			if err != nil {
				t.Fatalf("LoadSpatialPreview(%s) error = %v", datasetName, err)
			}
			if preview.QueriedBounds != nil {
				t.Fatalf("QueriedBounds = %+v, want nil for ignored viewport", preview.QueriedBounds)
			}
			if preview.Strategy != string(types.SpatialQueryStrategyBoundedSample) {
				t.Fatalf("Strategy = %q, want bounded_sample", preview.Strategy)
			}
			if preview.DegradedReason != string(types.SpatialQueryReasonUnsupportedDatasetKind) {
				t.Fatalf("DegradedReason = %q, want unsupported_dataset_kind", preview.DegradedReason)
			}
		})
	}
}

func TestViewerSpatialRequiredIDBypassesOrdinaryVertexBudget(t *testing.T) {
	app := NewApp()
	features := []*types.Feature{
		{ID: 1, Geometry: &types.PointGeometry{Coordinates: []float64{0, 0}, BBox: []float64{0, 0, 0, 0}}},
		{ID: 2, Geometry: &types.MultiLineStringGeometry{Coordinates: [][][]float64{{{0, 0}, {1, 1}}}, BBox: []float64{0, 0, 1, 1}}},
		{ID: 99, Geometry: &types.MultiLineStringGeometry{Coordinates: [][][]float64{{{2, 2}, {3, 3}}}, BBox: []float64{2, 2, 3, 3}}},
	}
	required := map[int]struct{}{99: {}}

	previewFeatures, vertexBudgetReached := app.formatPreviewFeatures(features, nil, 1, required)

	if !vertexBudgetReached {
		t.Fatal("vertexBudgetReached = false, want true")
	}
	if len(previewFeatures) != 2 || previewFeatures[0].ID != 1 || previewFeatures[1].ID != 99 {
		t.Fatalf("preview feature IDs = %v, want ordinary ID 1 plus required ID 99", previewFeatureIDs(previewFeatures))
	}
}

func TestViewerSpatialRequiredIDVertexBudgetDoesNotChangeSDKHasMore(t *testing.T) {
	response := &SpatialPreviewDTO{HasMore: false}

	applySpatialPreviewSampling(response, true)

	if response.HasMore {
		t.Fatal("HasMore changed to true after viewer vertex-budget truncation")
	}
	if !response.Sampled || !strings.Contains(response.SampleReason, "顶点上限") {
		t.Fatalf("sampling state = sampled %v, reason %q", response.Sampled, response.SampleReason)
	}
}

func TestViewerSpatialRequiredIDsAllowOneNormalizedSelectionAndRejectMultiple(t *testing.T) {
	path, _ := createViewerPointFixture(t, true, 0)
	app := NewApp()
	if _, err := app.OpenUDBXFile(path); err != nil {
		t.Fatalf("OpenUDBXFile() error = %v", err)
	}
	viewport := &BoundingBoxDTO{MinX: 0, MinY: 0, MaxX: 5, MaxY: 5}

	preview, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{
		Viewport:    viewport,
		RequiredIDs: []int{2, 2},
	})
	if err != nil {
		t.Fatalf("LoadSpatialPreview(duplicate required ID) error = %v", err)
	}
	if ids := previewFeatureIDs(preview.Features); !containsPreviewFeatureID(ids, 2) {
		t.Fatalf("preview feature IDs = %v, want required ID 2", ids)
	}

	_, err = app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{
		Viewport:    viewport,
		RequiredIDs: []int{2, 3, 2},
	})
	if err == nil {
		t.Fatal("LoadSpatialPreview(multiple required IDs) error = nil")
	}
	reason, ok := udbx4go.SpatialQueryReasonOf(err)
	if !ok || reason != types.SpatialQueryReasonInvalidViewport {
		t.Fatalf("spatial query reason = %q, present %v, want invalid_viewport", reason, ok)
	}
	if !udbx4go.IsConstraintViolation(err) {
		t.Fatalf("error = %v, want constraint classification", err)
	}
}

func TestViewerSpatialLifecycleFileSwitchCancelsAndWaitsForRealPreview(t *testing.T) {
	pointPath, _ := createViewerPointFixture(t, false, 0)
	samplePath := sampleDataPath(t)
	tests := []struct {
		name        string
		path        string
		datasetName string
		request     SpatialPreviewRequestDTO
	}{
		{name: "point bounded preview", path: pointPath, datasetName: "viewer_points"},
		{
			name:        "point viewport preview",
			path:        pointPath,
			datasetName: "viewer_points",
			request: SpatialPreviewRequestDTO{
				Viewport: &BoundingBoxDTO{MinX: 0, MinY: 0, MaxX: 5, MaxY: 5},
			},
		},
		{name: "text bounded preview", path: samplePath, datasetName: "County_T"},
		{
			name:        "cad bounded preview",
			path:        samplePath,
			datasetName: "CADDT",
			request: SpatialPreviewRequestDTO{
				Viewport: &BoundingBoxDTO{MinX: 10, MinY: 10, MaxX: -10, MaxY: -10},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := NewApp()
			if _, err := app.OpenUDBXFile(test.path); err != nil {
				t.Fatalf("OpenUDBXFile() error = %v", err)
			}
			initial, err := app.LoadSpatialPreview(test.datasetName, test.request)
			if err != nil {
				t.Fatalf("initial LoadSpatialPreview() error = %v", err)
			}

			entered := make(chan struct{})
			canceled := make(chan error, 1)
			forceRelease := make(chan struct{})
			app.previewQueryHook = func(ctx context.Context) error {
				close(entered)
				select {
				case <-ctx.Done():
					canceled <- ctx.Err()
					return ctx.Err()
				case <-forceRelease:
					return fmt.Errorf("forced preview release")
				}
			}

			type previewResult struct {
				preview *SpatialPreviewDTO
				err     error
			}
			previewDone := make(chan previewResult, 1)
			go func() {
				preview, loadErr := app.LoadSpatialPreview(test.datasetName, test.request)
				previewDone <- previewResult{preview: preview, err: loadErr}
			}()
			<-entered

			openDone := make(chan error, 1)
			go func() {
				_, openErr := app.OpenUDBXFile(test.path)
				openDone <- openErr
			}()

			select {
			case cancelErr := <-canceled:
				if !stderrors.Is(cancelErr, context.Canceled) {
					close(forceRelease)
					t.Fatalf("preview lifecycle cancellation = %v, want context.Canceled", cancelErr)
				}
			case <-time.After(time.Second):
				close(forceRelease)
				<-openDone
				t.Fatal("file switch did not cancel active LoadSpatialPreview")
			}

			old := <-previewDone
			if old.preview != nil {
				t.Fatalf("old preview = %+v, want nil after file switch", old.preview)
			}
			assertViewerSpatialReason(t, old.err, types.SpatialQueryReasonQueryTimeout)
			if !stderrors.Is(old.err, context.Canceled) {
				t.Fatalf("old preview error = %v, want context.Canceled", old.err)
			}
			if err := <-openDone; err != nil {
				t.Fatalf("OpenUDBXFile(switch) error = %v", err)
			}

			app.previewQueryHook = nil
			current, err := app.LoadSpatialPreview(test.datasetName, test.request)
			if err != nil {
				t.Fatalf("current LoadSpatialPreview() error = %v", err)
			}
			if current.FileGeneration <= initial.FileGeneration {
				t.Fatalf("current generation = %d, want greater than old %d", current.FileGeneration, initial.FileGeneration)
			}
		})
	}
}

func TestViewerSpatialLifecycleDropsCompletedQueryWhenFileSwitchStartsBeforeReturn(t *testing.T) {
	path, _ := createViewerPointFixture(t, false, 0)
	app := NewApp()
	if _, err := app.OpenUDBXFile(path); err != nil {
		t.Fatalf("OpenUDBXFile() error = %v", err)
	}

	resultReady := make(chan struct{})
	app.previewResultHook = func(ctx context.Context) {
		close(resultReady)
		<-ctx.Done()
	}
	type previewResult struct {
		preview *SpatialPreviewDTO
		err     error
	}
	previewDone := make(chan previewResult, 1)
	go func() {
		preview, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{})
		previewDone <- previewResult{preview: preview, err: err}
	}()
	<-resultReady

	openDone := make(chan error, 1)
	go func() {
		_, err := app.OpenUDBXFile(path)
		openDone <- err
	}()

	old := <-previewDone
	if old.preview != nil {
		t.Fatalf("old preview = %+v, want nil after switch starts", old.preview)
	}
	assertViewerSpatialReason(t, old.err, types.SpatialQueryReasonQueryTimeout)
	if !stderrors.Is(old.err, context.Canceled) {
		t.Fatalf("old preview error = %v, want context.Canceled", old.err)
	}
	if err := <-openDone; err != nil {
		t.Fatalf("OpenUDBXFile(switch) error = %v", err)
	}
}

func TestViewerSpatialLifecycleCloseCancelsAndWaitsForRealPreview(t *testing.T) {
	path, _ := createViewerPointFixture(t, false, 0)
	app := NewApp()
	if _, err := app.OpenUDBXFile(path); err != nil {
		t.Fatalf("OpenUDBXFile() error = %v", err)
	}
	entered := make(chan struct{})
	app.previewQueryHook = func(ctx context.Context) error {
		close(entered)
		<-ctx.Done()
		return ctx.Err()
	}

	previewDone := make(chan error, 1)
	go func() {
		_, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{})
		previewDone <- err
	}()
	<-entered
	closeDone := make(chan error, 1)
	go func() { closeDone <- app.CloseUDBXFile() }()

	err := <-previewDone
	assertViewerSpatialReason(t, err, types.SpatialQueryReasonQueryTimeout)
	if !stderrors.Is(err, context.Canceled) {
		t.Fatalf("preview error = %v, want context.Canceled", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("CloseUDBXFile() error = %v", err)
	}
	if current := app.GetCurrentFile(); current != "" {
		t.Fatalf("GetCurrentFile() = %q, want closed", current)
	}
}

func TestViewerSpatialPreviewAllPathsUseTask1Timeout(t *testing.T) {
	pointPath, _ := createViewerPointFixture(t, false, 0)
	samplePath := sampleDataPath(t)
	tests := []struct {
		name        string
		path        string
		datasetName string
		request     SpatialPreviewRequestDTO
	}{
		{name: "point bounded preview", path: pointPath, datasetName: "viewer_points"},
		{name: "line bounded preview", path: samplePath, datasetName: "BaseMap_L"},
		{name: "region bounded preview", path: samplePath, datasetName: "BaseMap_R"},
		{name: "point-z bounded preview", path: samplePath, datasetName: "BaseMap_PZ"},
		{name: "line-z bounded preview", path: samplePath, datasetName: "BaseMap_LZ"},
		{name: "region-z bounded preview", path: samplePath, datasetName: "BaseMap_RZ"},
		{
			name:        "point viewport preview",
			path:        pointPath,
			datasetName: "viewer_points",
			request: SpatialPreviewRequestDTO{
				Viewport: &BoundingBoxDTO{MinX: 0, MinY: 0, MaxX: 5, MaxY: 5},
			},
		},
		{name: "text bounded preview", path: samplePath, datasetName: "County_T"},
		{name: "cad bounded preview", path: samplePath, datasetName: "CADDT"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := NewApp()
			if _, err := app.OpenUDBXFile(test.path); err != nil {
				t.Fatalf("OpenUDBXFile() error = %v", err)
			}
			app.previewQueryHook = func(ctx context.Context) error {
				deadline, ok := ctx.Deadline()
				if !ok {
					return fmt.Errorf("preview query context has no deadline")
				}
				remaining := time.Until(deadline)
				if remaining <= 0 || remaining > spatialQueryPolicy().BuildTimeout {
					return fmt.Errorf("preview query deadline remaining %s is outside Task1 timeout", remaining)
				}
				return nil
			}

			if _, err := app.LoadSpatialPreview(test.datasetName, test.request); err != nil {
				t.Fatalf("LoadSpatialPreview() error = %v", err)
			}
		})
	}
}

func TestViewerSpatialBoundedPreviewMapsMissingGeometryToCorruptGeometry(t *testing.T) {
	tests := []struct {
		name        string
		fixture     func(*testing.T) string
		datasetName string
	}{
		{
			name: "point",
			fixture: func(t *testing.T) string {
				path, _ := createViewerPointFixture(t, false, 0)
				return path
			},
			datasetName: "viewer_points",
		},
		{name: "text", fixture: copySampleDataFixture, datasetName: "County_T"},
		{name: "cad", fixture: copySampleDataFixture, datasetName: "CADDT"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := test.fixture(t)
			clearViewerDatasetGeometry(t, path, test.datasetName)
			app := NewApp()
			if _, err := app.OpenUDBXFile(path); err != nil {
				t.Fatalf("OpenUDBXFile() error = %v", err)
			}

			_, err := app.LoadSpatialPreview(test.datasetName, SpatialPreviewRequestDTO{})

			assertViewerSpatialReason(t, err, types.SpatialQueryReasonCorruptGeometry)
			if !udbx4go.IsFormatError(err) {
				t.Fatalf("error = %v, want format classification", err)
			}
		})
	}
}

func TestViewerSpatialBoundedPreviewUsesListContext(t *testing.T) {
	lister := &contextOnlyPreviewLister{}
	app := NewApp()

	features, err := app.listPreviewFeatures(context.Background(), lister, &types.QueryOptions{Limit: 1})

	if err != nil {
		t.Fatalf("listPreviewFeatures() error = %v", err)
	}
	if len(features) != 1 || features[0].ID != 1 {
		t.Fatalf("features = %+v, want context-aware result", features)
	}
	if !lister.contextCalled || lister.legacyCalled {
		t.Fatalf("ListContext called = %v, List called = %v", lister.contextCalled, lister.legacyCalled)
	}
}

type contextOnlyPreviewLister struct {
	contextCalled bool
	legacyCalled  bool
}

func (l *contextOnlyPreviewLister) List(*types.QueryOptions) ([]*types.Feature, error) {
	l.legacyCalled = true
	return nil, fmt.Errorf("legacy List must not be called")
}

func (l *contextOnlyPreviewLister) ListContext(context.Context, *types.QueryOptions) ([]*types.Feature, error) {
	l.contextCalled = true
	return []*types.Feature{{ID: 1}}, nil
}

func assertViewerSpatialReason(t *testing.T, err error, want types.SpatialQueryReason) {
	t.Helper()
	requireError := err != nil
	if !requireError {
		t.Fatal("spatial preview error = nil")
	}
	reason, ok := udbx4go.SpatialQueryReasonOf(err)
	if !ok || reason != want {
		t.Fatalf("spatial query reason = %q, present %v, want %q", reason, ok, want)
	}
}

func createViewerPointFixture(t *testing.T, withRTree bool, objectCountOverride int) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "viewer-spatial.udbx")
	source, err := udbx4go.Create(path)
	if err != nil {
		t.Fatalf("create fixture: %v", err)
	}
	points, err := source.CreatePointDataset("viewer_points", 4326, nil)
	if err != nil {
		t.Fatalf("create point dataset: %v", err)
	}
	tableName := points.Info().TableName
	for _, feature := range []*types.Feature{
		{ID: 1, Geometry: &types.PointGeometry{Type: "Point", Coordinates: []float64{1, 1}, SRID: 4326, BBox: []float64{1, 1, 1, 1}}},
		{ID: 2, Geometry: &types.PointGeometry{Type: "Point", Coordinates: []float64{20, 20}, SRID: 4326, BBox: []float64{20, 20, 20, 20}}},
		{ID: 3, Geometry: &types.PointGeometry{Type: "Point", Coordinates: []float64{30, 30}, SRID: 4326, BBox: []float64{30, 30, 30, 30}}},
	} {
		if err := points.Insert(feature); err != nil {
			t.Fatalf("insert point %d: %v", feature.ID, err)
		}
	}
	if err := source.Close(); err != nil {
		t.Fatalf("close fixture source: %v", err)
	}

	db := openViewerFixtureDB(t, path)
	objectCount := 3
	if objectCountOverride > 0 {
		objectCount = objectCountOverride
	}
	if _, err := db.Exec(`
		UPDATE SmRegister
		SET SmObjectCount = ?, SmLeft = -10, SmBottom = -20, SmRight = 30, SmTop = 40,
		    SmIDColName = 'SmID', SmGeoColName = 'SmGeometry'
		WHERE SmDatasetName = 'viewer_points'`, objectCount); err != nil {
		t.Fatalf("update fixture metadata: %v", err)
	}
	if withRTree {
		if _, err := db.Exec(`UPDATE geometry_columns SET spatial_index_enabled = 1 WHERE f_table_name = ?`, tableName); err != nil {
			t.Fatalf("enable fixture RTree metadata: %v", err)
		}
		rtreeName := "idx_" + tableName + "_SmGeometry"
		if _, err := db.Exec(fmt.Sprintf(`CREATE VIRTUAL TABLE %q USING rtree(pkid, xmin, xmax, ymin, ymax)`, rtreeName)); err != nil {
			t.Fatalf("create fixture RTree: %v", err)
		}
		for _, row := range []struct {
			id int
			x  float64
		}{{1, 1}, {2, 20}, {3, 30}} {
			if _, err := db.Exec(fmt.Sprintf(`INSERT INTO %q VALUES (?, ?, ?, ?, ?)`, rtreeName), row.id, row.x, row.x, row.x, row.x); err != nil {
				t.Fatalf("insert fixture RTree row %d: %v", row.id, err)
			}
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close fixture database: %v", err)
	}
	return path, tableName
}

func openViewerFixtureDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open fixture database: %v", err)
	}
	return db
}

func copySampleDataFixture(t *testing.T) string {
	t.Helper()
	content, err := os.ReadFile(sampleDataPath(t))
	if err != nil {
		t.Fatalf("read sample fixture: %v", err)
	}
	path := filepath.Join(t.TempDir(), "SampleData.udbx")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("copy sample fixture: %v", err)
	}
	return path
}

func clearViewerDatasetGeometry(t *testing.T, path string, datasetName string) {
	t.Helper()
	db := openViewerFixtureDB(t, path)
	defer db.Close()
	var tableName string
	if err := db.QueryRow(`SELECT SmTableName FROM SmRegister WHERE SmDatasetName = ?`, datasetName).Scan(&tableName); err != nil {
		t.Fatalf("find dataset table: %v", err)
	}
	query := fmt.Sprintf(
		`UPDATE %q SET SmGeometry = NULL WHERE SmID = (SELECT MIN(SmID) FROM %q)`,
		tableName,
		tableName,
	)
	if _, err := db.Exec(query); err != nil {
		t.Fatalf("clear dataset geometry: %v", err)
	}
}

func previewFeatureIDs(features []PreviewFeatureDTO) []int {
	ids := make([]int, len(features))
	for i, feature := range features {
		ids[i] = feature.ID
	}
	return ids
}

func containsPreviewFeatureID(ids []int, want int) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

func TestViewerSpatialPreviewLoadsAllHenanCountyRegionsForTableSelection(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(henanDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(henan) error = %v", err)
	}

	const datasetName = "县级行政区划"
	preview, err := app.LoadSpatialPreview(datasetName, SpatialPreviewRequestDTO{Limit: 1000})
	if err != nil {
		t.Fatalf("LoadSpatialPreview(%s) error = %v", datasetName, err)
	}
	if len(preview.Features) != 164 {
		t.Fatalf("len(preview.Features) = %d, want all 164 county regions", len(preview.Features))
	}

	secondPage, err := app.LoadDatasetPage(datasetName, 2)
	if err != nil {
		t.Fatalf("LoadDatasetPage(%s, 2) error = %v", datasetName, err)
	}
	if len(secondPage.Rows) == 0 {
		t.Fatal("LoadDatasetPage second page returned no rows")
	}
	secondPageFeatureID := secondPage.Rows[0][0]
	for _, feature := range preview.Features {
		if strconv.Itoa(feature.ID) == secondPageFeatureID {
			return
		}
	}
	t.Fatalf("preview does not contain second-page SmID %s, so table selection cannot highlight it on the map", secondPageFeatureID)
}

func TestViewerGetFeatureAttributesUsesDatasetNameAndSmID(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	preview, err := app.LoadSpatialPreview("BaseMap_P", SpatialPreviewRequestDTO{Limit: 1, MaxVertices: 10})
	if err != nil {
		t.Fatalf("LoadSpatialPreview(BaseMap_P) error = %v", err)
	}
	if len(preview.Features) == 0 {
		t.Fatal("LoadSpatialPreview(BaseMap_P) returned no features")
	}

	attributes, err := app.GetFeatureAttributes("BaseMap_P", preview.Features[0].ID)
	if err != nil {
		t.Fatalf("GetFeatureAttributes() error = %v", err)
	}
	if attributes.DatasetName != "BaseMap_P" {
		t.Fatalf("DatasetName = %q, want BaseMap_P", attributes.DatasetName)
	}
	if attributes.ID != preview.Features[0].ID {
		t.Fatalf("ID = %d, want %d", attributes.ID, preview.Features[0].ID)
	}
	if attributes.GeometryType != "Point" {
		t.Fatalf("GeometryType = %q, want Point", attributes.GeometryType)
	}
	if attributes.Properties == nil {
		t.Fatal("Properties = nil")
	}
}

func TestViewerGetFeatureAttributesRejectsMissingFeature(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	_, err := app.GetFeatureAttributes("BaseMap_P", -999999)
	if err == nil || !strings.Contains(err.Error(), "要素不存在") {
		t.Fatalf("GetFeatureAttributes(missing) error = %v", err)
	}
}
