import type { BoundingBox, MapLayerState, SelectedMapFeature } from '../types'
import type { SpatialRendererAdapter } from './SpatialRendererAdapter'

export class MapLibreSpatialRendererAdapter implements SpatialRendererAdapter {
  mount(): void {
    throw new Error('MapLibre 空间预览 adapter 暂未接入多图层 viewer UI')
  }

  destroy(): void {
    // No-op until MapLibre is wired into the viewer UI.
  }

  setLayer(_layer: MapLayerState): void {
    throw new Error('MapLibre 空间预览 adapter 暂未接入多图层 viewer UI')
  }

  removeLayer(_datasetName: string): void {
    throw new Error('MapLibre 空间预览 adapter 暂未接入多图层 viewer UI')
  }

  setLayerVisible(_datasetName: string, _visible: boolean): void {
    throw new Error('MapLibre 空间预览 adapter 暂未接入多图层 viewer UI')
  }

  fitAllVisibleLayers(): void {
    throw new Error('MapLibre 空间预览 adapter 暂未接入多图层 viewer UI')
  }

  setSelection(_selection: SelectedMapFeature | null): void {
    throw new Error('MapLibre 空间预览 adapter 暂未接入多图层 viewer UI')
  }

  onFeatureClick(_handler: (datasetName: string, featureID: number) => void): void {
    // No-op until MapLibre is wired into the viewer UI.
  }

  onViewportChange(_handler: (viewport: BoundingBox) => void): void {
    // No-op until MapLibre is wired into the viewer UI.
  }
}
