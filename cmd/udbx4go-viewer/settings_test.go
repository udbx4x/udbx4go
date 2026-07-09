package main

import (
	"encoding/json"
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
