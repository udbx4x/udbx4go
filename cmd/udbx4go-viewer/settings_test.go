package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	udbx4go "github.com/udbx4x/udbx4go"
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
	tests := []struct {
		name                  string
		featureLimit          int
		vertexBudget          int
		wantFeatureLimit      int
		wantVertexBudget      int
		autoFitOnLayerChange  bool
		zoomToSelectedFeature bool
		defaultOpen           bool
		wantAutoFitOnLayer    bool
		wantZoomToSelected    bool
		wantDefaultOpen       bool
	}{
		{
			name:                  "below minimum clamps to minimum",
			featureLimit:          1,
			vertexBudget:          1,
			wantFeatureLimit:      minSpatialPreviewFeatureLimit,
			wantVertexBudget:      minSpatialPreviewVertexBudget,
			autoFitOnLayerChange:  true,
			zoomToSelectedFeature: true,
			defaultOpen:           true,
			wantAutoFitOnLayer:    true,
			wantZoomToSelected:    true,
			wantDefaultOpen:       true,
		},
		{
			name:                  "above maximum clamps to maximum",
			featureLimit:          maxSpatialPreviewFeatureLimit + 1,
			vertexBudget:          maxSpatialPreviewVertexBudget + 1,
			wantFeatureLimit:      maxSpatialPreviewFeatureLimit,
			wantVertexBudget:      maxSpatialPreviewVertexBudget,
			autoFitOnLayerChange:  true,
			zoomToSelectedFeature: true,
			defaultOpen:           true,
			wantAutoFitOnLayer:    true,
			wantZoomToSelected:    true,
			wantDefaultOpen:       true,
		},
		{
			name:                  "zero uses defaults and false bools are preserved",
			featureLimit:          0,
			vertexBudget:          0,
			wantFeatureLimit:      DefaultViewerSettings().SpatialPreview.FeatureLimit,
			wantVertexBudget:      DefaultViewerSettings().SpatialPreview.VertexBudget,
			autoFitOnLayerChange:  false,
			zoomToSelectedFeature: false,
			defaultOpen:           false,
			wantAutoFitOnLayer:    false,
			wantZoomToSelected:    false,
			wantDefaultOpen:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := DefaultViewerSettings()
			settings.SpatialPreview.FeatureLimit = tt.featureLimit
			settings.SpatialPreview.VertexBudget = tt.vertexBudget
			settings.SpatialPreview.AutoFitOnLayerChange = tt.autoFitOnLayerChange
			settings.MapInteraction.ZoomToSelectedFeature = tt.zoomToSelectedFeature
			settings.Table.DefaultOpen = tt.defaultOpen

			normalized := NormalizeViewerSettings(settings)

			if normalized.SpatialPreview.FeatureLimit != tt.wantFeatureLimit {
				t.Fatalf("FeatureLimit = %d, want %d", normalized.SpatialPreview.FeatureLimit, tt.wantFeatureLimit)
			}
			if normalized.SpatialPreview.VertexBudget != tt.wantVertexBudget {
				t.Fatalf("VertexBudget = %d, want %d", normalized.SpatialPreview.VertexBudget, tt.wantVertexBudget)
			}
			if normalized.SpatialPreview.AutoFitOnLayerChange != tt.wantAutoFitOnLayer {
				t.Fatalf("AutoFitOnLayerChange = %v, want %v", normalized.SpatialPreview.AutoFitOnLayerChange, tt.wantAutoFitOnLayer)
			}
			if normalized.MapInteraction.ZoomToSelectedFeature != tt.wantZoomToSelected {
				t.Fatalf("ZoomToSelectedFeature = %v, want %v", normalized.MapInteraction.ZoomToSelectedFeature, tt.wantZoomToSelected)
			}
			if normalized.Table.DefaultOpen != tt.wantDefaultOpen {
				t.Fatalf("Table.DefaultOpen = %v, want %v", normalized.Table.DefaultOpen, tt.wantDefaultOpen)
			}
		})
	}
}

func TestGetViewerSettingsMissingFileReturnsDefaults(t *testing.T) {
	app := NewApp()
	app.settingsPathOverride = filepath.Join(t.TempDir(), "missing", "settings.json")

	loaded, err := app.GetViewerSettings()
	if err != nil {
		t.Fatalf("GetViewerSettings() error = %v", err)
	}
	want := DefaultViewerSettings()
	if *loaded != want {
		t.Fatalf("GetViewerSettings() = %+v, want %+v", *loaded, want)
	}
	if _, err := os.Stat(app.settingsPathOverride); !os.IsNotExist(err) {
		t.Fatalf("os.Stat() error = %v, want os.ErrNotExist", err)
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
	if err == nil {
		t.Fatal("GetViewerSettings() error = nil, want corrupt settings warning")
	}
	if loaded.SpatialPreview.FeatureLimit != DefaultViewerSettings().SpatialPreview.FeatureLimit {
		t.Fatalf("loaded FeatureLimit = %d, want default", loaded.SpatialPreview.FeatureLimit)
	}
}

func TestSpatialQueryPolicyUsesSDKDefaultsWithoutViewerSettingsFields(t *testing.T) {
	if got, want := spatialQueryPolicy(), udbx4go.DefaultSpatialQueryPolicy(); got != want {
		t.Fatalf("spatialQueryPolicy() = %+v, want SDK default %+v", got, want)
	}

	content, err := json.Marshal(DefaultViewerSettings())
	if err != nil {
		t.Fatalf("json.Marshal(DefaultViewerSettings()) error = %v", err)
	}
	for _, forbidden := range []string{"cacheMiB", "concurrency", "maxDatasetCacheBytes", "maxTotalCacheBytes"} {
		if strings.Contains(string(content), forbidden) {
			t.Fatalf("viewer settings expose internal spatial policy field %q: %s", forbidden, content)
		}
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
