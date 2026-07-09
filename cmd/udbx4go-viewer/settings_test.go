package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultViewerSettings(t *testing.T) {
	settings := DefaultViewerSettings()

	if settings.SpatialPreview.FeatureLimit != defaultSpatialPreviewLimit {
		t.Fatalf("FeatureLimit = %d, want %d", settings.SpatialPreview.FeatureLimit, defaultSpatialPreviewLimit)
	}
	if settings.SpatialPreview.VertexBudget != defaultSpatialVertexBudget {
		t.Fatalf("VertexBudget = %d, want %d", settings.SpatialPreview.VertexBudget, defaultSpatialVertexBudget)
	}
	if settings.SpatialPreview.FeatureLimit < minSpatialPreviewFeatureLimit ||
		settings.SpatialPreview.FeatureLimit > maxSpatialPreviewFeatureLimit {
		t.Fatalf(
			"FeatureLimit = %d, want within [%d, %d]",
			settings.SpatialPreview.FeatureLimit,
			minSpatialPreviewFeatureLimit,
			maxSpatialPreviewFeatureLimit,
		)
	}
	if settings.SpatialPreview.VertexBudget < minSpatialPreviewVertexBudget ||
		settings.SpatialPreview.VertexBudget > maxSpatialPreviewVertexBudget {
		t.Fatalf(
			"VertexBudget = %d, want within [%d, %d]",
			settings.SpatialPreview.VertexBudget,
			minSpatialPreviewVertexBudget,
			maxSpatialPreviewVertexBudget,
		)
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

func TestViewerSettingsJSONContract(t *testing.T) {
	data, err := json.Marshal(DefaultViewerSettings())
	if err != nil {
		t.Fatalf("json.Marshal(DefaultViewerSettings()) error = %v", err)
	}

	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		t.Fatalf("json.Unmarshal(DefaultViewerSettings()) error = %v", err)
	}

	spatialPreview := requireObjectField(t, settings, "spatialPreview")
	requireNumberField(t, spatialPreview, "featureLimit")
	requireNumberField(t, spatialPreview, "vertexBudget")
	requireBoolField(t, spatialPreview, "autoFitOnLayerChange")

	mapInteraction := requireObjectField(t, settings, "mapInteraction")
	requireBoolField(t, mapInteraction, "zoomToSelectedFeature")

	table := requireObjectField(t, settings, "table")
	requireBoolField(t, table, "defaultOpen")

	advanced := requireObjectField(t, settings, "advanced")
	requireBoolField(t, advanced, "showPreviewStats")
}

func TestNormalizeViewerSettingsClampsRanges(t *testing.T) {
	settings := DefaultViewerSettings()
	settings.SpatialPreview.FeatureLimit = 1
	settings.SpatialPreview.VertexBudget = 1

	normalized := NormalizeViewerSettings(settings)

	if normalized.SpatialPreview.FeatureLimit != minSpatialPreviewFeatureLimit {
		t.Fatalf("FeatureLimit = %d, want %d", normalized.SpatialPreview.FeatureLimit, minSpatialPreviewFeatureLimit)
	}
	if normalized.SpatialPreview.VertexBudget != minSpatialPreviewVertexBudget {
		t.Fatalf("VertexBudget = %d, want %d", normalized.SpatialPreview.VertexBudget, minSpatialPreviewVertexBudget)
	}
}

func TestViewerSettingsPersistence(t *testing.T) {
	app := NewApp()
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")

	settings := DefaultViewerSettings()
	settings.SpatialPreview.FeatureLimit = 2000
	settings.Table.DefaultOpen = false

	saved, err := app.SaveViewerSettings(settings)
	if err != nil {
		t.Fatalf("SaveViewerSettings() error = %v", err)
	}
	if saved.SpatialPreview.FeatureLimit != 2000 {
		t.Fatalf("saved FeatureLimit = %d, want 2000", saved.SpatialPreview.FeatureLimit)
	}

	loaded, err := app.GetViewerSettings()
	if err != nil {
		t.Fatalf("GetViewerSettings() error = %v", err)
	}
	if loaded.SpatialPreview.FeatureLimit != 2000 {
		t.Fatalf("loaded FeatureLimit = %d, want 2000", loaded.SpatialPreview.FeatureLimit)
	}
	if loaded.Table.DefaultOpen {
		t.Fatal("loaded Table.DefaultOpen = true, want false")
	}
}

func TestResetViewerSettings(t *testing.T) {
	app := NewApp()
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")

	settings := DefaultViewerSettings()
	settings.SpatialPreview.FeatureLimit = 2000
	if _, err := app.SaveViewerSettings(settings); err != nil {
		t.Fatalf("SaveViewerSettings() error = %v", err)
	}

	reset, err := app.ResetViewerSettings()
	if err != nil {
		t.Fatalf("ResetViewerSettings() error = %v", err)
	}
	if reset.SpatialPreview.FeatureLimit != DefaultViewerSettings().SpatialPreview.FeatureLimit {
		t.Fatalf("reset FeatureLimit = %d, want default", reset.SpatialPreview.FeatureLimit)
	}
}

func TestViewerSettingsCorruptFileReturnsDefaults(t *testing.T) {
	app := NewApp()
	app.settingsPathOverride = filepath.Join(t.TempDir(), "settings.json")

	if err := os.WriteFile(app.settingsPathOverride, []byte("{bad json"), 0o644); err != nil {
		t.Fatalf("os.WriteFile() error = %v", err)
	}

	loaded, err := app.GetViewerSettings()
	if err != nil {
		t.Fatalf("GetViewerSettings() error = %v", err)
	}
	if loaded.SpatialPreview.FeatureLimit != DefaultViewerSettings().SpatialPreview.FeatureLimit {
		t.Fatalf("loaded FeatureLimit = %d, want default", loaded.SpatialPreview.FeatureLimit)
	}
}

func requireObjectField(t *testing.T, object map[string]any, name string) map[string]any {
	t.Helper()

	value, ok := object[name]
	if !ok {
		t.Fatalf("missing JSON field %q", name)
	}

	nested, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("JSON field %q has type %T, want object", name, value)
	}

	return nested
}

func requireNumberField(t *testing.T, object map[string]any, name string) {
	t.Helper()

	value, ok := object[name]
	if !ok {
		t.Fatalf("missing JSON field %q", name)
	}
	if _, ok := value.(float64); !ok {
		t.Fatalf("JSON field %q has type %T, want number", name, value)
	}
}

func requireBoolField(t *testing.T, object map[string]any, name string) {
	t.Helper()

	value, ok := object[name]
	if !ok {
		t.Fatalf("missing JSON field %q", name)
	}
	if _, ok := value.(bool); !ok {
		t.Fatalf("JSON field %q has type %T, want bool", name, value)
	}
}
