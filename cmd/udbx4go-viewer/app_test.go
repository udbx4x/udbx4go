package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func sampleDataPath(t *testing.T) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "..", "data", "SampleData.udbx"))
	if err != nil {
		t.Fatalf("resolve sample path: %v", err)
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
	if len(preview.Features) != 3 {
		t.Fatalf("len(Features) = %d, want 3", len(preview.Features))
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
