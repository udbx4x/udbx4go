export interface ViewerSettings {
  spatialPreview: {
    featureLimit: number
    vertexBudget: number
    autoFitOnLayerChange: boolean
  }
  mapInteraction: {
    zoomToSelectedFeature: boolean
  }
  table: {
    defaultOpen: boolean
  }
  advanced: {
    showPreviewStats: boolean
  }
}

export const viewerSettingsConstraints = {
  featureLimit: { min: 100, max: 10000 },
  vertexBudget: { min: 50000, max: 10000000 },
}

export const defaultViewerSettings: ViewerSettings = {
  spatialPreview: {
    featureLimit: 1000,
    vertexBudget: 1000000,
    autoFitOnLayerChange: true,
  },
  mapInteraction: {
    zoomToSelectedFeature: true,
  },
  table: {
    defaultOpen: true,
  },
  advanced: {
    showPreviewStats: false,
  },
}
