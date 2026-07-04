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
