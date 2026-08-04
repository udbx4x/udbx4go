package main

import (
	"context"
	"database/sql"
	stderrors "errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"os"
	"path/filepath"
	"reflect"
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
	if len(preview.Features) == 0 || len(preview.Features) > minSpatialPreviewFeatureLimit {
		t.Fatalf("len(Features) = %d, want full-domain result in [1, %d]", len(preview.Features), minSpatialPreviewFeatureLimit)
	}
	if preview.QueriedBounds != nil {
		t.Fatalf("QueriedBounds = %+v, want nil when caller omitted viewport", preview.QueriedBounds)
	}
	if preview.Strategy != spatialPreviewStrategyBoundedSample {
		t.Fatalf("Strategy = %q, want bounded_sample initial preview", preview.Strategy)
	}
	if preview.DegradedReason != "" {
		t.Fatalf("DegradedReason = %q, want empty", preview.DegradedReason)
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

func TestViewerLoadsRealHenanWeiboDeclaredExtentForInitialViewport(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(henanDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(henan) error = %v", err)
	}
	defer app.CloseUDBXFile()

	summary, err := app.GetDatasetSpatialSummary("weibo")
	if err != nil {
		t.Fatalf("GetDatasetSpatialSummary(weibo) error = %v", err)
	}
	if summary.Extent == nil {
		t.Fatal("weibo extent = nil")
	}

	preview, err := app.LoadSpatialPreview("weibo", SpatialPreviewRequestDTO{
		Viewport:    summary.Extent,
		Limit:       maxSpatialPreviewFeatureLimit,
		MaxVertices: DefaultViewerSettings().SpatialPreview.VertexBudget,
	})
	if err != nil {
		t.Fatalf("LoadSpatialPreview(weibo declared extent) error = %v", err)
	}
	if preview.Strategy != string(types.SpatialQueryStrategyRTree) {
		t.Fatalf("preview strategy = %q, want rtree", preview.Strategy)
	}
	if len(preview.Features) == 0 || len(preview.Features) > maxSpatialPreviewFeatureLimit {
		t.Fatalf("preview feature count = %d, want 1..%d", len(preview.Features), maxSpatialPreviewFeatureLimit)
	}
}

func TestOpenUDBXFileReturnsAuthoritativeFileGeneration(t *testing.T) {
	path, _ := createViewerPointFixture(t, false, 0)
	app := NewApp()

	first, err := app.OpenUDBXFile(path)
	if err != nil {
		t.Fatalf("OpenUDBXFile(first) error = %v", err)
	}
	firstPreview, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{})
	if err != nil {
		t.Fatalf("LoadSpatialPreview(first) error = %v", err)
	}
	if first.FileGeneration != firstPreview.FileGeneration {
		t.Fatalf("first file generation = %d, preview generation = %d", first.FileGeneration, firstPreview.FileGeneration)
	}

	second, err := app.OpenUDBXFile(path)
	if err != nil {
		t.Fatalf("OpenUDBXFile(second) error = %v", err)
	}
	defer app.CloseUDBXFile()
	secondPreview, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{})
	if err != nil {
		t.Fatalf("LoadSpatialPreview(second) error = %v", err)
	}
	if second.FileGeneration != secondPreview.FileGeneration {
		t.Fatalf("second file generation = %d, preview generation = %d", second.FileGeneration, secondPreview.FileGeneration)
	}
	if second.FileGeneration <= first.FileGeneration {
		t.Fatalf("second generation = %d, want greater than first %d", second.FileGeneration, first.FileGeneration)
	}
	current := app.GetCurrentFileInfo()
	if current == nil || current.Path != second.Path || current.FileGeneration != second.FileGeneration {
		t.Fatalf("GetCurrentFileInfo() = %+v, want path %q generation %d", current, second.Path, second.FileGeneration)
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

func TestViewerSpatialPreviewSampleReasonCombinesActualQueryAndVertexLimits(t *testing.T) {
	response := &SpatialPreviewDTO{HasMore: true}
	applySpatialPreviewSampling(response, true)

	if !response.Sampled {
		t.Fatal("Sampled = false, want true")
	}
	if !strings.Contains(response.SampleReason, "要素上限") {
		t.Fatalf("SampleReason = %q, want feature limit reason", response.SampleReason)
	}
	if !strings.Contains(response.SampleReason, "顶点上限") {
		t.Fatalf("SampleReason = %q, want vertex budget reason", response.SampleReason)
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

func TestViewerSpatialPreviewWithoutViewportUsesBoundedListAndActualQueryState(t *testing.T) {
	for _, objectCount := range []int{1, 1000} {
		t.Run(fmt.Sprintf("SmObjectCount-%d", objectCount), func(t *testing.T) {
			path := createViewerSequentialPointFixture(t, 150, objectCount)
			app := NewApp()
			if _, err := app.OpenUDBXFile(path); err != nil {
				t.Fatalf("OpenUDBXFile() error = %v", err)
			}

			preview, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{
				Limit:       100,
				MaxVertices: maxSpatialPreviewVertexBudget,
			})
			if err != nil {
				t.Fatalf("LoadSpatialPreview() error = %v", err)
			}

			wantIDs := make([]int, 100)
			for index := range wantIDs {
				wantIDs[index] = index + 1
			}
			if ids := previewFeatureIDs(preview.Features); !reflect.DeepEqual(ids, wantIDs) {
				t.Fatalf("feature IDs = %v, want ordered IDs %v", ids, wantIDs)
			}
			if preview.QueriedBounds != nil {
				t.Fatalf("QueriedBounds = %+v, want nil when caller omitted viewport", preview.QueriedBounds)
			}
			if !preview.HasMore {
				t.Fatal("HasMore = false, want true from actual limit+1 query")
			}
			if !preview.Sampled || preview.SampleReason != "预览达到要素上限" {
				t.Fatalf("Sampled = %v, SampleReason = %q, want actual query truncation", preview.Sampled, preview.SampleReason)
			}
			if preview.Strategy != spatialPreviewStrategyBoundedSample {
				t.Fatalf("Strategy = %q, want bounded_sample initial preview", preview.Strategy)
			}
			if preview.DegradedReason != "" {
				t.Fatalf("DegradedReason = %q, want empty", preview.DegradedReason)
			}
		})
	}
}

func TestViewerSpatialPreviewWithoutViewportIgnoresUnavailableDeclaredExtent(t *testing.T) {
	tests := []struct {
		name   string
		update string
	}{
		{name: "missing extent", update: "SmLeft = NULL"},
		{name: "invalid extent", update: "SmLeft = 50, SmRight = 40"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, _ := createViewerPointFixture(t, false, 0)
			db := openViewerFixtureDB(t, path)
			if _, err := db.Exec("UPDATE SmRegister SET " + tt.update + " WHERE SmDatasetName = 'viewer_points'"); err != nil {
				t.Fatalf("invalidate declared extent: %v", err)
			}
			if err := db.Close(); err != nil {
				t.Fatalf("close fixture database: %v", err)
			}

			app := NewApp()
			if _, err := app.OpenUDBXFile(path); err != nil {
				t.Fatalf("OpenUDBXFile() error = %v", err)
			}
			preview, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{Limit: 100})
			if err != nil {
				t.Fatalf("LoadSpatialPreview() error = %v", err)
			}
			if ids := previewFeatureIDs(preview.Features); !reflect.DeepEqual(ids, []int{1, 2, 3}) {
				t.Fatalf("feature IDs = %v, want [1 2 3]", ids)
			}
			if preview.QueriedBounds != nil {
				t.Fatalf("QueriedBounds = %+v, want nil when caller omitted viewport", preview.QueriedBounds)
			}
			if preview.HasMore || preview.Sampled || preview.SampleReason != "" {
				t.Fatalf("HasMore = %v, Sampled = %v, SampleReason = %q, want complete result", preview.HasMore, preview.Sampled, preview.SampleReason)
			}
			if preview.Strategy != spatialPreviewStrategyBoundedSample || preview.DegradedReason != "" {
				t.Fatalf("Strategy = %q, DegradedReason = %q, want non-degraded bounded_sample", preview.Strategy, preview.DegradedReason)
			}
		})
	}
}

func TestViewerSpatialPreviewWithoutViewportIncludesCoordinatesBeyondFloat32Range(t *testing.T) {
	for _, withRTree := range []bool{false, true} {
		t.Run(fmt.Sprintf("rtree-%t", withRTree), func(t *testing.T) {
			path := createViewerExtremeCoordinatePointFixture(t, withRTree)
			app := NewApp()
			if _, err := app.OpenUDBXFile(path); err != nil {
				t.Fatalf("OpenUDBXFile() error = %v", err)
			}

			preview, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{
				Limit:       100,
				MaxVertices: maxSpatialPreviewVertexBudget,
			})
			if err != nil {
				t.Fatalf("LoadSpatialPreview() error = %v", err)
			}

			if ids := previewFeatureIDs(preview.Features); !reflect.DeepEqual(ids, []int{1, 2, 3}) {
				t.Fatalf("feature IDs = %v, want all IDs beyond and within float32 range", ids)
			}
			if preview.HasMore || preview.Sampled || preview.SampleReason != "" {
				t.Fatalf("HasMore = %v, Sampled = %v, SampleReason = %q, want complete initial sample", preview.HasMore, preview.Sampled, preview.SampleReason)
			}
			if preview.Strategy != spatialPreviewStrategyBoundedSample || preview.QueriedBounds != nil || preview.DegradedReason != "" {
				t.Fatalf("Strategy = %q, QueriedBounds = %+v, DegradedReason = %q, want non-degraded unbounded-request sample", preview.Strategy, preview.QueriedBounds, preview.DegradedReason)
			}
		})
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
		viewport := &BoundingBoxDTO{MinX: 100, MinY: 100, MaxX: 101, MaxY: 101}
		preview, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{
			Viewport:    viewport,
			Limit:       100,
			RequiredIDs: []int{3},
		})
		if err != nil {
			t.Fatalf("LoadSpatialPreview() error = %v", err)
		}
		if ids := previewFeatureIDs(preview.Features); !reflect.DeepEqual(ids, []int{1, 2, 3}) {
			t.Fatalf("feature IDs = %v, want [1 2 3]", ids)
		}
		if preview.Strategy != "bounded_sample" {
			t.Fatalf("Strategy = %q, want bounded_sample", preview.Strategy)
		}
		if preview.DegradedReason != string(types.SpatialQueryReasonEnvelopeCacheBudgetExceeded) {
			t.Fatalf("DegradedReason = %q, want envelope_cache_budget_exceeded", preview.DegradedReason)
		}
		if preview.HasMore {
			t.Fatal("HasMore = true, want false for three rows with limit 100")
		}
		if preview.QueriedBounds == nil || *preview.QueriedBounds != *viewport {
			t.Fatalf("QueriedBounds = %+v, want %+v", preview.QueriedBounds, viewport)
		}
	})
}

func TestViewerSpatialRoutingDoesNotDefineDatasetKindWhitelist(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "app.go", nil, 0)
	if err != nil {
		t.Fatalf("parse app.go: %v", err)
	}
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if ok && function.Name.Name == "supportsViewportSpatialQuery" {
			t.Fatal("Viewer must use DatasetKind.IsSpatial and SDK capability instead of a local viewport kind whitelist")
		}
	}
}

func TestViewerSpatialBoundedPreviewPreservesMinimumFeatureLimit(t *testing.T) {
	path, _ := createViewerPointFixture(t, false, 1_000_000)
	app := NewApp()
	if _, err := app.OpenUDBXFile(path); err != nil {
		t.Fatalf("OpenUDBXFile() error = %v", err)
	}

	preview, err := app.LoadSpatialPreview("viewer_points", SpatialPreviewRequestDTO{
		Viewport:    &BoundingBoxDTO{MinX: 100, MinY: 100, MaxX: 101, MaxY: 101},
		Limit:       2,
		RequiredIDs: []int{3},
	})
	if err != nil {
		t.Fatalf("LoadSpatialPreview() error = %v", err)
	}
	if ids := previewFeatureIDs(preview.Features); !reflect.DeepEqual(ids, []int{1, 2, 3}) {
		t.Fatalf("feature IDs = %v, want [1 2 3]", ids)
	}
	if preview.HasMore {
		t.Fatal("HasMore = true, want false after clamping viewport limit to 100")
	}
}

func TestViewerSpatialReasonTextAndCADUseViewportQueryCapability(t *testing.T) {
	app := NewApp()
	if _, err := app.OpenUDBXFile(sampleDataPath(t)); err != nil {
		t.Fatalf("OpenUDBXFile(sample) error = %v", err)
	}

	tests := []struct {
		datasetName string
		targetKind  string
	}{
		{datasetName: "County_T", targetKind: "text"},
		{datasetName: "CADDT", targetKind: "cad-line"},
		{datasetName: "CADDT", targetKind: "cad-region"},
	}
	for _, test := range tests {
		t.Run(test.datasetName+"/"+test.targetKind, func(t *testing.T) {
			summary, err := app.GetDatasetSpatialSummary(test.datasetName)
			if err != nil {
				t.Fatalf("GetDatasetSpatialSummary(%s) error = %v", test.datasetName, err)
			}
			if !summary.PreviewSupported || !summary.ViewportQuerySupported || summary.Extent == nil {
				t.Fatalf("summary(%s) = %+v", test.datasetName, summary)
			}

			dataSource, _, queryContext, release, err := app.acquireDataSource()
			if err != nil {
				t.Fatalf("acquireDataSource() error = %v", err)
			}
			defer release()
			capability, err := dataSource.GetSpatialQueryCapability(queryContext, test.datasetName)
			if err != nil {
				t.Fatalf("GetSpatialQueryCapability(%s) error = %v", test.datasetName, err)
			}
			wantViewportSupport := capability.RTreeAvailable || capability.FallbackAvailable
			if summary.ViewportQuerySupported != wantViewportSupport {
				t.Fatalf("ViewportQuerySupported = %v, want SDK capability %v", summary.ViewportQuerySupported, wantViewportSupport)
			}
			if summary.RTreeAvailable != capability.RTreeAvailable {
				t.Fatalf("RTreeAvailable = %v, want SDK capability %v", summary.RTreeAvailable, capability.RTreeAvailable)
			}
			if summary.QueryDiagnosticReason != string(capability.DiagnosticReason) {
				t.Fatalf("QueryDiagnosticReason = %q, want SDK reason %q", summary.QueryDiagnosticReason, capability.DiagnosticReason)
			}
			viewport, authoritative := selectViewerSpatialAuthority(
				t,
				dataSource,
				queryContext,
				test.datasetName,
				*summary.Extent,
				test.targetKind,
			)
			if authoritative.Strategy != types.SpatialQueryStrategyEnvelopeCache {
				t.Fatalf("SDK strategy = %q, want envelope_cache", authoritative.Strategy)
			}

			preview, err := app.LoadSpatialPreview(test.datasetName, SpatialPreviewRequestDTO{
				Viewport:    viewport,
				Limit:       100,
				MaxVertices: maxSpatialPreviewVertexBudget,
			})
			if err != nil {
				t.Fatalf("LoadSpatialPreview(%s) error = %v", test.datasetName, err)
			}
			if len(preview.Features) != len(authoritative.Features) {
				t.Fatalf("len(preview.Features) = %d, want SDK result %d", len(preview.Features), len(authoritative.Features))
			}
			if preview.QueriedBounds == nil || *preview.QueriedBounds != *viewport {
				t.Fatalf("QueriedBounds = %+v, want selective viewport %+v", preview.QueriedBounds, viewport)
			}
			if preview.Strategy != string(types.SpatialQueryStrategyEnvelopeCache) {
				t.Fatalf("Strategy = %q, want envelope_cache", preview.Strategy)
			}
			if preview.DegradedReason != "" {
				t.Fatalf("DegradedReason = %q, want empty", preview.DegradedReason)
			}
			if preview.HasMore != authoritative.HasMore {
				t.Fatalf("HasMore = %v, want SDK result %v", preview.HasMore, authoritative.HasMore)
			}
			if !reflect.DeepEqual(preview.SRID, summary.SRID) {
				t.Fatalf("SRID = %v, want summary %v", preview.SRID, summary.SRID)
			}
			for index, feature := range authoritative.Features {
				assertViewerPreviewMatchesSDKFeature(t, preview.Features[index], feature, preview.SRID)
			}
		})
	}
}

func TestViewerSpatialReasonLegacyCADUsesBoundedFallbackWhenIndexUnavailable(t *testing.T) {
	path := createViewerLegacyCADFixture(t, 150)
	app := NewApp()
	if _, err := app.OpenUDBXFile(path); err != nil {
		t.Fatalf("OpenUDBXFile() error = %v", err)
	}

	summary, err := app.GetDatasetSpatialSummary("CADDT")
	if err != nil {
		t.Fatalf("GetDatasetSpatialSummary(CADDT) error = %v", err)
	}
	if summary.Extent == nil || summary.ViewportQuerySupported || summary.RTreeAvailable {
		t.Fatalf("summary(CADDT) = %+v", summary)
	}
	if summary.QueryDiagnosticReason != string(types.SpatialQueryReasonSpatialIndexUnavailable) {
		t.Fatalf("QueryDiagnosticReason = %q, want spatial_index_unavailable", summary.QueryDiagnosticReason)
	}

	preview, err := app.LoadSpatialPreview("CADDT", SpatialPreviewRequestDTO{
		Viewport: summary.Extent,
		Limit:    100,
	})
	if err != nil {
		t.Fatalf("LoadSpatialPreview(CADDT) error = %v", err)
	}
	if len(preview.Features) == 0 || len(preview.Features) > 100 {
		t.Fatalf("len(Features) = %d, want bounded result in [1, 100]", len(preview.Features))
	}
	if !preview.HasMore {
		t.Fatal("HasMore = false, want true for fixture larger than the fallback limit")
	}
	if preview.Strategy != spatialPreviewStrategyBoundedSample {
		t.Fatalf("Strategy = %q, want bounded_sample", preview.Strategy)
	}
	if preview.DegradedReason != string(types.SpatialQueryReasonSpatialIndexUnavailable) {
		t.Fatalf("DegradedReason = %q, want spatial_index_unavailable", preview.DegradedReason)
	}
	if preview.QueriedBounds == nil || *preview.QueriedBounds != *summary.Extent {
		t.Fatalf("QueriedBounds = %+v, want summary extent %+v", preview.QueriedBounds, summary.Extent)
	}
}

func TestViewerSpatialReasonBoundedFallbackWhitelist(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "envelope cache budget exceeded",
			err: newViewerSpatialError(
				types.SpatialQueryReasonEnvelopeCacheBudgetExceeded,
				stderrors.New("cache budget exceeded"),
			),
			want: true,
		},
		{
			name: "spatial index unavailable",
			err: newViewerSpatialError(
				types.SpatialQueryReasonSpatialIndexUnavailable,
				stderrors.New("index unavailable"),
			),
			want: true,
		},
		{
			name: "corrupt geometry",
			err: newViewerSpatialError(
				types.SpatialQueryReasonCorruptGeometry,
				stderrors.New("corrupt geometry"),
			),
		},
		{
			name: "query timeout",
			err: newViewerSpatialError(
				types.SpatialQueryReasonQueryTimeout,
				context.DeadlineExceeded,
			),
		},
		{name: "context cancellation", err: context.Canceled},
		{name: "unclassified error", err: stderrors.New("query failed")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := viewerCanUseBoundedFallback(test.err); got != test.want {
				t.Fatalf("viewerCanUseBoundedFallback() = %v, want %v", got, test.want)
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
			name:        "cad viewport preview",
			path:        samplePath,
			datasetName: "CADDT",
			request: SpatialPreviewRequestDTO{
				Viewport: &BoundingBoxDTO{MinX: -180, MinY: -90, MaxX: 180, MaxY: 90},
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

func TestViewerSpatialPreviewAllPathsUseViewerRequestTimeout(t *testing.T) {
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
				if remaining <= spatialQueryPolicy().BuildTimeout || remaining > viewerSpatialQueryTimeout {
					return fmt.Errorf("preview query deadline remaining %s does not preserve the SDK build budget plus Viewer overhead", remaining)
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

func TestViewerSpatialBoundedPreviewLoadsLimitPlusOneAndRequiredIDs(t *testing.T) {
	path, _ := createViewerPointFixture(t, false, 0)
	app := NewApp()
	if _, err := app.OpenUDBXFile(path); err != nil {
		t.Fatalf("OpenUDBXFile() error = %v", err)
	}

	dataSource, _, dataSourceContext, release, err := app.acquireDataSource()
	if err != nil {
		t.Fatalf("acquireDataSource() error = %v", err)
	}
	defer release()
	_, ds, err := app.getDatasetForPreview(dataSource, "viewer_points")
	if err != nil {
		t.Fatalf("getDatasetForPreview() error = %v", err)
	}

	features, hasMore, err := app.loadBoundedPreviewFeatures(dataSourceContext, ds, 2, []int{3})
	if err != nil {
		t.Fatalf("loadBoundedPreviewFeatures() error = %v", err)
	}
	ids := make([]int, len(features))
	for i, feature := range features {
		ids[i] = feature.ID
	}
	if !reflect.DeepEqual(ids, []int{1, 2, 3}) {
		t.Fatalf("feature IDs = %v, want ordinary [1 2] plus required [3]", ids)
	}
	if !hasMore {
		t.Fatal("hasMore = false, want true for three ordinary rows with limit two")
	}
}

func TestViewerSpatialBoundedPreviewFormatsRequiredIDs(t *testing.T) {
	path, _ := createViewerPointFixture(t, false, 0)
	app := NewApp()
	if _, err := app.OpenUDBXFile(path); err != nil {
		t.Fatalf("OpenUDBXFile() error = %v", err)
	}

	dataSource, _, dataSourceContext, release, err := app.acquireDataSource()
	if err != nil {
		t.Fatalf("acquireDataSource() error = %v", err)
	}
	defer release()
	_, ds, err := app.getDatasetForPreview(dataSource, "viewer_points")
	if err != nil {
		t.Fatalf("getDatasetForPreview() error = %v", err)
	}

	tests := []struct {
		name          string
		requiredIDs   []int
		wantLoadedIDs []int
		wantFinalIDs  []int
	}{
		{
			name:          "overlapping required ID is deduplicated",
			requiredIDs:   []int{2},
			wantLoadedIDs: []int{1, 2, 2},
			wantFinalIDs:  []int{1, 2},
		},
		{
			name:          "overlapping required ID moves to the end",
			requiredIDs:   []int{1},
			wantLoadedIDs: []int{1, 2, 1},
			wantFinalIDs:  []int{2, 1},
		},
		{
			name:          "missing required ID adds no placeholder",
			requiredIDs:   []int{999},
			wantLoadedIDs: []int{1, 2},
			wantFinalIDs:  []int{1, 2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features, hasMore, err := app.loadBoundedPreviewFeatures(dataSourceContext, ds, 2, tt.requiredIDs)
			if err != nil {
				t.Fatalf("loadBoundedPreviewFeatures() error = %v", err)
			}
			loadedIDs := make([]int, len(features))
			for i, feature := range features {
				loadedIDs[i] = feature.ID
			}
			if !reflect.DeepEqual(loadedIDs, tt.wantLoadedIDs) {
				t.Fatalf("loaded feature IDs = %v, want %v", loadedIDs, tt.wantLoadedIDs)
			}
			if !hasMore {
				t.Fatal("hasMore = false, want true from the ordinary limit plus one query")
			}

			formatted, vertexBudgetReached := app.formatPreviewFeatures(
				features,
				nil,
				maxSpatialPreviewVertexBudget,
				requiredIDSet(tt.requiredIDs),
			)
			if vertexBudgetReached {
				t.Fatal("vertexBudgetReached = true, want false")
			}
			if ids := previewFeatureIDs(formatted); !reflect.DeepEqual(ids, tt.wantFinalIDs) {
				t.Fatalf("formatted feature IDs = %v, want %v", ids, tt.wantFinalIDs)
			}
		})
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

func createViewerSequentialPointFixture(t *testing.T, featureCount int, objectCount int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "viewer-sequential-points.udbx")
	source, err := udbx4go.Create(path)
	if err != nil {
		t.Fatalf("create sequential point fixture: %v", err)
	}
	points, err := source.CreatePointDataset("viewer_points", 4326, nil)
	if err != nil {
		t.Fatalf("create sequential point dataset: %v", err)
	}
	tableName := points.Info().TableName
	for id := 1; id <= featureCount; id++ {
		coordinate := float64(id)
		if err := points.Insert(&types.Feature{
			ID: id,
			Geometry: &types.PointGeometry{
				Type:        "Point",
				Coordinates: []float64{coordinate, coordinate},
				SRID:        4326,
				BBox:        []float64{coordinate, coordinate, coordinate, coordinate},
			},
		}); err != nil {
			t.Fatalf("insert sequential point %d: %v", id, err)
		}
	}
	if err := source.Close(); err != nil {
		t.Fatalf("close sequential point fixture: %v", err)
	}

	db := openViewerFixtureDB(t, path)
	if _, err := db.Exec(`
		UPDATE SmRegister
		SET SmObjectCount = ?, SmLeft = 1, SmBottom = 1, SmRight = 10, SmTop = 10,
		    SmIDColName = 'SmID', SmGeoColName = 'SmGeometry'
		WHERE SmDatasetName = 'viewer_points'`, objectCount); err != nil {
		t.Fatalf("shrink sequential point metadata: %v", err)
	}
	if _, err := db.Exec(`UPDATE geometry_columns SET spatial_index_enabled = 1 WHERE f_table_name = ?`, tableName); err != nil {
		t.Fatalf("enable sequential point RTree metadata: %v", err)
	}
	rtreeName := "idx_" + tableName + "_SmGeometry"
	if _, err := db.Exec(fmt.Sprintf(`CREATE VIRTUAL TABLE %q USING rtree(pkid, xmin, xmax, ymin, ymax)`, rtreeName)); err != nil {
		t.Fatalf("create sequential point RTree: %v", err)
	}
	for id := 1; id <= featureCount; id++ {
		coordinate := float64(id)
		if _, err := db.Exec(
			fmt.Sprintf(`INSERT INTO %q VALUES (?, ?, ?, ?, ?)`, rtreeName),
			id, coordinate, coordinate, coordinate, coordinate,
		); err != nil {
			t.Fatalf("insert sequential point RTree row %d: %v", id, err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close sequential point fixture database: %v", err)
	}
	return path
}

func createViewerExtremeCoordinatePointFixture(t *testing.T, withRTree bool) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "viewer-extreme-coordinate-points.udbx")
	source, err := udbx4go.Create(path)
	if err != nil {
		t.Fatalf("create extreme-coordinate point fixture: %v", err)
	}
	points, err := source.CreatePointDataset("viewer_points", 4326, nil)
	if err != nil {
		t.Fatalf("create extreme-coordinate point dataset: %v", err)
	}
	tableName := points.Info().TableName
	coordinates := []float64{-1e39, 0, 1e39}
	for index, coordinate := range coordinates {
		id := index + 1
		if err := points.Insert(&types.Feature{
			ID: id,
			Geometry: &types.PointGeometry{
				Type:        "Point",
				Coordinates: []float64{coordinate, coordinate},
				SRID:        4326,
				BBox:        []float64{coordinate, coordinate, coordinate, coordinate},
			},
		}); err != nil {
			t.Fatalf("insert extreme-coordinate point %d: %v", id, err)
		}
	}
	if err := source.Close(); err != nil {
		t.Fatalf("close extreme-coordinate point fixture: %v", err)
	}

	db := openViewerFixtureDB(t, path)
	if _, err := db.Exec(`
		UPDATE SmRegister
		SET SmObjectCount = 3, SmLeft = -1, SmBottom = -1, SmRight = 1, SmTop = 1,
		    SmIDColName = 'SmID', SmGeoColName = 'SmGeometry'
		WHERE SmDatasetName = 'viewer_points'`); err != nil {
		t.Fatalf("set stale extreme-coordinate metadata: %v", err)
	}
	if withRTree {
		if _, err := db.Exec(`UPDATE geometry_columns SET spatial_index_enabled = 1 WHERE f_table_name = ?`, tableName); err != nil {
			t.Fatalf("enable extreme-coordinate RTree metadata: %v", err)
		}
		rtreeName := "idx_" + tableName + "_SmGeometry"
		if _, err := db.Exec(fmt.Sprintf(`CREATE VIRTUAL TABLE %q USING rtree(pkid, xmin, xmax, ymin, ymax)`, rtreeName)); err != nil {
			t.Fatalf("create extreme-coordinate RTree: %v", err)
		}
		for index, coordinate := range coordinates {
			id := index + 1
			if _, err := db.Exec(
				fmt.Sprintf(`INSERT INTO %q VALUES (?, ?, ?, ?, ?)`, rtreeName),
				id, coordinate, coordinate, coordinate, coordinate,
			); err != nil {
				t.Fatalf("insert extreme-coordinate RTree row %d: %v", id, err)
			}
		}
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close extreme-coordinate point fixture database: %v", err)
	}
	return path
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

func createViewerLegacyCADFixture(t *testing.T, duplicateCount int) string {
	t.Helper()
	path := copySampleDataFixture(t)
	db := openViewerFixtureDB(t, path)
	defer db.Close()

	rows, err := db.Query(`SELECT name FROM sqlite_master WHERE type = 'trigger' AND tbl_name = 'CADDT'`)
	if err != nil {
		t.Fatalf("list CAD fixture triggers: %v", err)
	}
	var triggerNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			t.Fatalf("scan CAD fixture trigger: %v", err)
		}
		triggerNames = append(triggerNames, name)
	}
	if err := rows.Close(); err != nil {
		t.Fatalf("close CAD fixture trigger rows: %v", err)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate CAD fixture triggers: %v", err)
	}
	for _, name := range triggerNames {
		if _, err := db.Exec(fmt.Sprintf(`DROP TRIGGER %q`, name)); err != nil {
			t.Fatalf("drop CAD fixture trigger %s: %v", name, err)
		}
	}
	if _, err := db.Exec(`ALTER TABLE CADDT DROP COLUMN SmIndexKey`); err != nil {
		t.Fatalf("drop CAD fixture SmIndexKey column: %v", err)
	}
	var smGeoTypeColumns, smIndexKeyColumns int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('CADDT') WHERE name = 'SmGeoType' COLLATE NOCASE`).Scan(&smGeoTypeColumns); err != nil {
		t.Fatalf("inspect CAD fixture SmGeoType column: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('CADDT') WHERE name = 'SmIndexKey' COLLATE NOCASE`).Scan(&smIndexKeyColumns); err != nil {
		t.Fatalf("inspect CAD fixture SmIndexKey column: %v", err)
	}
	if smGeoTypeColumns != 1 || smIndexKeyColumns != 0 {
		t.Fatalf("legacy CAD columns: SmGeoType=%d SmIndexKey=%d, want 1 and 0", smGeoTypeColumns, smIndexKeyColumns)
	}

	if duplicateCount > 0 {
		var maxID int
		if err := db.QueryRow(`SELECT MAX(SmID) FROM CADDT`).Scan(&maxID); err != nil {
			t.Fatalf("read CAD fixture max ID: %v", err)
		}
		if _, err := db.Exec(`
			WITH RECURSIVE seq(n) AS (
				VALUES(1)
				UNION ALL
				SELECT n + 1 FROM seq WHERE n < ?
			), seed AS (
				SELECT SmUserID, SmGeoType, SmGeometry FROM CADDT ORDER BY SmID LIMIT 1
			)
			INSERT INTO CADDT (SmID, SmUserID, SmGeoType, SmGeometry)
			SELECT ? + seq.n, seed.SmUserID, seed.SmGeoType, seed.SmGeometry FROM seq CROSS JOIN seed`, duplicateCount, maxID); err != nil {
			t.Fatalf("expand legacy CAD fixture: %v", err)
		}
	}
	if _, err := db.Exec(`
		UPDATE SmRegister
		SET SmObjectCount = (SELECT COUNT(*) FROM CADDT)
		WHERE SmDatasetName = 'CADDT'`); err != nil {
		t.Fatalf("update legacy CAD fixture object count: %v", err)
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

func selectViewerSpatialAuthority(
	t *testing.T,
	dataSource *udbx4go.DataSource,
	ctx context.Context,
	datasetName string,
	extent BoundingBoxDTO,
	targetKind string,
) (*BoundingBoxDTO, *types.SpatialQueryResult) {
	t.Helper()
	all, err := dataSource.QuerySpatial(ctx, datasetName, types.SpatialQueryOptions{
		Bounds: extent.spatialBoundingBox(),
		Limit:  maxSpatialPreviewFeatureLimit,
	})
	if err != nil {
		t.Fatalf("QuerySpatial(%s full extent) error = %v", datasetName, err)
	}
	if len(all.Features) < 2 {
		t.Fatalf("QuerySpatial(%s full extent) returned %d features, need at least two for selective viewport", datasetName, len(all.Features))
	}

	for _, feature := range all.Features {
		if !sdkGeometryMatchesTarget(feature.Geometry, targetKind) {
			continue
		}
		bbox := feature.Geometry.GetBBox()
		if len(bbox) < 4 {
			continue
		}
		centerX := (bbox[0] + bbox[2]) / 2
		centerY := (bbox[1] + bbox[3]) / 2
		viewport := &BoundingBoxDTO{MinX: centerX, MinY: centerY, MaxX: centerX, MaxY: centerY}
		result, err := dataSource.QuerySpatial(ctx, datasetName, types.SpatialQueryOptions{
			Bounds: viewport.spatialBoundingBox(),
			Limit:  100,
		})
		if err != nil {
			t.Fatalf("QuerySpatial(%s selective viewport) error = %v", datasetName, err)
		}
		if containsSDKFeatureID(result.Features, feature.ID) && len(result.Features) < len(all.Features) {
			return viewport, result
		}
	}
	t.Fatalf("could not find a selective %s viewport for %s from %d SDK features", targetKind, datasetName, len(all.Features))
	return nil, nil
}

func sdkGeometryMatchesTarget(geometry types.Geometry, targetKind string) bool {
	switch targetKind {
	case "text":
		_, ok := geometry.(*types.TextGeometry)
		return ok
	case "cad-line":
		_, ok := geometry.(*types.CadLineGeometry)
		return ok
	case "cad-region":
		_, ok := geometry.(*types.CadRegionGeometry)
		return ok
	default:
		return false
	}
}

func containsSDKFeatureID(features []*types.Feature, want int) bool {
	for _, feature := range features {
		if feature.ID == want {
			return true
		}
	}
	return false
}

func assertViewerPreviewMatchesSDKFeature(
	t *testing.T,
	preview PreviewFeatureDTO,
	feature *types.Feature,
	datasetSRID *int,
) {
	t.Helper()
	if preview.ID != feature.ID {
		t.Fatalf("preview ID = %d, want SDK ID %d", preview.ID, feature.ID)
	}
	wantGeometry, ok := sdkExpectedPreviewGeometry(feature.Geometry)
	if !ok {
		t.Fatalf("unsupported SDK geometry for preview comparison: %T", feature.Geometry)
	}
	if preview.Geometry.Type != wantGeometry.Type || preview.Geometry.HasZ != wantGeometry.HasZ {
		t.Fatalf("preview geometry = {%s hasZ=%v}, want {%s hasZ=%v}", preview.Geometry.Type, preview.Geometry.HasZ, wantGeometry.Type, wantGeometry.HasZ)
	}
	assertViewerCoordinateStructure(t, preview.Geometry.Coordinates, wantGeometry.Coordinates, "coordinates")
	assertViewerBBoxMatchesSDK(t, preview.BBox, feature.Geometry.GetBBox())
	if datasetSRID != nil && feature.Geometry.GetSRID() != *datasetSRID {
		t.Fatalf("SDK feature SRID = %d, want dataset SRID %d", feature.Geometry.GetSRID(), *datasetSRID)
	}
}

func sdkExpectedPreviewGeometry(geometry types.Geometry) (PreviewGeometryDTO, bool) {
	switch typed := geometry.(type) {
	case *types.TextGeometry:
		return PreviewGeometryDTO{Type: "Text", Coordinates: sdkFloatCoordinates(typed.Anchor), HasZ: typed.HasZ()}, true
	case *types.CadPointGeometry:
		return PreviewGeometryDTO{Type: "Point", Coordinates: []interface{}{typed.XCoord, typed.YCoord}}, true
	case *types.CadLineGeometry:
		return PreviewGeometryDTO{Type: "MultiLineString", Coordinates: sdkCADSegments(typed.Coordinates, typed.SubPointCounts)}, true
	case *types.CadRegionGeometry:
		return PreviewGeometryDTO{Type: "MultiPolygon", Coordinates: []interface{}{sdkCADSegments(typed.Coordinates, typed.SubPointCounts)}}, true
	case *types.CadTextGeometry:
		return PreviewGeometryDTO{Type: "Text", Coordinates: sdkFloatCoordinates(typed.Anchor)}, true
	}
	return PreviewGeometryDTO{}, false
}

func sdkFloatCoordinates(values []float64) []interface{} {
	coordinates := make([]interface{}, len(values))
	for index, value := range values {
		coordinates[index] = value
	}
	return coordinates
}

func sdkCADSegments(coordinates [][2]float64, subPointCounts []int) []interface{} {
	if len(subPointCounts) == 0 {
		subPointCounts = []int{len(coordinates)}
	}
	segments := make([]interface{}, 0, len(subPointCounts))
	offset := 0
	for _, count := range subPointCounts {
		if count <= 0 || offset >= len(coordinates) {
			continue
		}
		end := offset + count
		if end > len(coordinates) {
			end = len(coordinates)
		}
		segment := make([]interface{}, 0, end-offset)
		for _, coordinate := range coordinates[offset:end] {
			segment = append(segment, []interface{}{coordinate[0], coordinate[1]})
		}
		segments = append(segments, segment)
		offset = end
	}
	return segments
}

func assertViewerCoordinateStructure(t *testing.T, got interface{}, want interface{}, path string) {
	t.Helper()
	switch typedWant := want.(type) {
	case []interface{}:
		typedGot, ok := got.([]interface{})
		if !ok {
			t.Fatalf("%s type = %T, want []interface{}", path, got)
		}
		if len(typedGot) != len(typedWant) {
			t.Fatalf("%s length = %d, want %d", path, len(typedGot), len(typedWant))
		}
		for index := range typedWant {
			assertViewerCoordinateStructure(t, typedGot[index], typedWant[index], fmt.Sprintf("%s[%d]", path, index))
		}
	case float64:
		typedGot, ok := got.(float64)
		if !ok || math.Abs(typedGot-typedWant) > 1e-9 {
			t.Fatalf("%s = %v (%T), want %v", path, got, got, typedWant)
		}
	default:
		t.Fatalf("unsupported expected coordinate type at %s: %T", path, want)
	}
}

func assertViewerBBoxMatchesSDK(t *testing.T, preview *BoundingBoxDTO, bbox []float64) {
	t.Helper()
	if len(bbox) < 4 {
		if preview != nil {
			t.Fatalf("preview BBox = %+v, want nil", preview)
		}
		return
	}
	if preview == nil {
		t.Fatalf("preview BBox = nil, want %v", bbox[:4])
	}
	got := []float64{preview.MinX, preview.MinY, preview.MaxX, preview.MaxY}
	for index := range got {
		if math.Abs(got[index]-bbox[index]) > 1e-9 {
			t.Fatalf("preview BBox = %v, want SDK BBox %v", got, bbox[:4])
		}
	}
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
