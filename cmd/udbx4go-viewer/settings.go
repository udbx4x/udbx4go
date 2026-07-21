package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	udbx4go "github.com/udbx4x/udbx4go"
)

// ViewerSettingsDTO describes viewer settings exchanged with the frontend.
type ViewerSettingsDTO struct {
	SpatialPreview SpatialPreviewSettingsDTO `json:"spatialPreview"`
	MapInteraction MapInteractionSettingsDTO `json:"mapInteraction"`
	Table          TableSettingsDTO          `json:"table"`
	Advanced       AdvancedSettingsDTO       `json:"advanced"`
}

// SpatialPreviewSettingsDTO describes spatial preview loading behavior.
type SpatialPreviewSettingsDTO struct {
	FeatureLimit         int  `json:"featureLimit"`
	VertexBudget         int  `json:"vertexBudget"`
	AutoFitOnLayerChange bool `json:"autoFitOnLayerChange"`
}

// MapInteractionSettingsDTO describes map interaction behavior.
type MapInteractionSettingsDTO struct {
	ZoomToSelectedFeature bool `json:"zoomToSelectedFeature"`
}

// TableSettingsDTO describes attribute table behavior.
type TableSettingsDTO struct {
	DefaultOpen bool `json:"defaultOpen"`
}

// AdvancedSettingsDTO describes advanced viewer diagnostics.
type AdvancedSettingsDTO struct {
	ShowPreviewStats bool `json:"showPreviewStats"`
}

const (
	minSpatialPreviewFeatureLimit = 100
	maxSpatialPreviewFeatureLimit = 10000
	minSpatialPreviewVertexBudget = 50000
	maxSpatialPreviewVertexBudget = 10000000
	// Viewer requests include RTree lookup, geometry decoding, and Wails transfer.
	// This deadline is independent from the SDK's envelope-cache build budget.
	viewerSpatialQueryTimeout = 2 * time.Second
)

func DefaultViewerSettings() ViewerSettingsDTO {
	return ViewerSettingsDTO{
		SpatialPreview: SpatialPreviewSettingsDTO{
			FeatureLimit:         defaultSpatialPreviewLimit,
			VertexBudget:         defaultSpatialVertexBudget,
			AutoFitOnLayerChange: true,
		},
		MapInteraction: MapInteractionSettingsDTO{
			ZoomToSelectedFeature: true,
		},
		Table: TableSettingsDTO{
			DefaultOpen: true,
		},
		Advanced: AdvancedSettingsDTO{
			ShowPreviewStats: false,
		},
	}
}

func NormalizeViewerSettings(settings ViewerSettingsDTO) ViewerSettingsDTO {
	defaults := DefaultViewerSettings()

	if settings.SpatialPreview.FeatureLimit == 0 {
		settings.SpatialPreview.FeatureLimit = defaults.SpatialPreview.FeatureLimit
	}
	if settings.SpatialPreview.VertexBudget == 0 {
		settings.SpatialPreview.VertexBudget = defaults.SpatialPreview.VertexBudget
	}

	settings.SpatialPreview.FeatureLimit = clampInt(settings.SpatialPreview.FeatureLimit, minSpatialPreviewFeatureLimit, maxSpatialPreviewFeatureLimit)
	settings.SpatialPreview.VertexBudget = clampInt(settings.SpatialPreview.VertexBudget, minSpatialPreviewVertexBudget, maxSpatialPreviewVertexBudget)

	return settings
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func spatialQueryPolicy() udbx4go.SpatialQueryPolicy {
	return udbx4go.DefaultSpatialQueryPolicy()
}

func (a *App) GetViewerSettings() (*ViewerSettingsDTO, error) {
	path, err := a.viewerSettingsPath()
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		settings := DefaultViewerSettings()
		return &settings, nil
	}
	if err != nil {
		return nil, err
	}

	settings := DefaultViewerSettings()
	if err := json.Unmarshal(content, &settings); err != nil {
		settings := DefaultViewerSettings()
		return &settings, fmt.Errorf("viewer 设置文件损坏，已使用默认设置: %w", err)
	}

	settings = NormalizeViewerSettings(settings)
	return &settings, nil
}

func (a *App) SaveViewerSettings(settings ViewerSettingsDTO) (*ViewerSettingsDTO, error) {
	normalized := NormalizeViewerSettings(settings)
	path, err := a.viewerSettingsPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, append(content, '\n'), 0o644); err != nil {
		return nil, err
	}
	return &normalized, nil
}

func (a *App) ResetViewerSettings() (*ViewerSettingsDTO, error) {
	defaults := DefaultViewerSettings()
	return a.SaveViewerSettings(defaults)
}

func (a *App) viewerSettingsPath() (string, error) {
	if a.settingsPathOverride != "" {
		return a.settingsPathOverride, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "udbx4go-viewer", "settings.json"), nil
}
