package main

type ViewerSettingsDTO struct {
	SpatialPreview SpatialPreviewSettingsDTO `json:"spatialPreview"`
	MapInteraction MapInteractionSettingsDTO `json:"mapInteraction"`
	Table          TableSettingsDTO          `json:"table"`
	Advanced       AdvancedSettingsDTO       `json:"advanced"`
}

type SpatialPreviewSettingsDTO struct {
	FeatureLimit         int  `json:"featureLimit"`
	VertexBudget         int  `json:"vertexBudget"`
	AutoFitOnLayerChange bool `json:"autoFitOnLayerChange"`
}

type MapInteractionSettingsDTO struct {
	ZoomToSelectedFeature bool `json:"zoomToSelectedFeature"`
}

type TableSettingsDTO struct {
	DefaultOpen bool `json:"defaultOpen"`
}

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
			FeatureLimit:         1000,
			VertexBudget:         1000000,
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
