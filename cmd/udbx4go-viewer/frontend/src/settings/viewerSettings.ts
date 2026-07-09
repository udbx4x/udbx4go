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
