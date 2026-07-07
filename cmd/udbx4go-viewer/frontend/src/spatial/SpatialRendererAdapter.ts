import type { BoundingBox, PreviewFeature, SpatialSummary } from './types'

export interface SpatialRendererAdapter {
  mount(container: HTMLElement): void
  destroy(): void
  setSummary(summary: SpatialSummary): void
  setFeatures(features: PreviewFeature[]): void
  fitExtent(extent: BoundingBox): void
  setSelection(featureIDs: number[]): void
  onFeatureClick(handler: (featureID: number) => void): void
  onViewportChange(handler: (viewport: BoundingBox) => void): void
}

export type SpatialRendererKind = 'openlayers' | 'maplibre'
