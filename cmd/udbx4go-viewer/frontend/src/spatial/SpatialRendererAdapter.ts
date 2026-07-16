import type { BoundingBox, MapLayerState, SelectedMapFeature } from '../types'

export interface SpatialRendererAdapter {
  mount(container: HTMLElement): void
  destroy(): void
  setLayer(layer: MapLayerState): void
  removeLayer(datasetName: string): void
  setLayerVisible(datasetName: string, visible: boolean): void
  fitAllVisibleLayers(): void
  fitBounds(bounds: BoundingBox, geometryKind: 'point' | 'line' | 'polygon'): void
  setSelection(selection: SelectedMapFeature | null): void
  onFeatureClick(handler: (datasetName: string, featureID: number) => void): void
  getViewport(): BoundingBox | null
  onViewportChange(handler: (viewport: BoundingBox) => void): () => void
}

export type SpatialRendererKind = 'openlayers' | 'maplibre'
