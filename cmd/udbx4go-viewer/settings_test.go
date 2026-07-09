package main

import "testing"

func TestDefaultViewerSettings(t *testing.T) {
	settings := DefaultViewerSettings()

	if settings.SpatialPreview.FeatureLimit != 1000 {
		t.Fatalf("FeatureLimit = %d, want 1000", settings.SpatialPreview.FeatureLimit)
	}
	if settings.SpatialPreview.VertexBudget != 1000000 {
		t.Fatalf("VertexBudget = %d, want 1000000", settings.SpatialPreview.VertexBudget)
	}
	if !settings.SpatialPreview.AutoFitOnLayerChange {
		t.Fatal("AutoFitOnLayerChange = false, want true")
	}
	if !settings.MapInteraction.ZoomToSelectedFeature {
		t.Fatal("ZoomToSelectedFeature = false, want true")
	}
	if !settings.Table.DefaultOpen {
		t.Fatal("Table.DefaultOpen = false, want true")
	}
	if settings.Advanced.ShowPreviewStats {
		t.Fatal("ShowPreviewStats = true, want false")
	}
}
