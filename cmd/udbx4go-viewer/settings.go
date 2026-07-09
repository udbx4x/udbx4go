package main

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
