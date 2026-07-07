import { MapLibreSpatialRendererAdapter } from './MapLibreSpatialRendererAdapter'
import { OpenLayersSpatialRendererAdapter } from './OpenLayersSpatialRendererAdapter'
import type { SpatialRendererAdapter, SpatialRendererKind } from './SpatialRendererAdapter'

export function createSpatialRenderer(kind: SpatialRendererKind): SpatialRendererAdapter {
  switch (kind) {
    case 'openlayers':
      return new OpenLayersSpatialRendererAdapter()
    case 'maplibre':
      return new MapLibreSpatialRendererAdapter()
  }
}
