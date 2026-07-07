import 'maplibre-gl/dist/maplibre-gl.css'

import maplibregl, { GeoJSONSource, Map as MapLibreMap } from 'maplibre-gl'
import type { BoundingBox, PreviewFeature, SpatialSummary } from './types'
import type { SpatialRendererAdapter } from './SpatialRendererAdapter'

type FeatureCollection = {
  type: 'FeatureCollection'
  features: Array<{
    type: 'Feature'
    id: number
    properties: Record<string, string | number>
    geometry: {
      type: string
      coordinates: unknown[]
    }
  }>
}

const sourceID = 'udbx-preview'
const fillLayerID = 'udbx-preview-fill'
const lineLayerID = 'udbx-preview-line'
const pointLayerID = 'udbx-preview-point'

export class MapLibreSpatialRendererAdapter implements SpatialRendererAdapter {
  private map: MapLibreMap | null = null
  private features: PreviewFeature[] = []
  private selectedIDs = new Set<number>()
  private featureClickHandler: ((featureID: number) => void) | null = null
  private viewportChangeHandler: ((viewport: BoundingBox) => void) | null = null

  mount(container: HTMLElement): void {
    this.map = new maplibregl.Map({
      container,
      style: {
        version: 8,
        sources: {},
        layers: [],
      },
      center: [0, 0],
      zoom: 1,
      attributionControl: false,
    })

    this.map.on('load', () => {
      this.installLayers()
      this.updateSource()
    })

    this.map.on('click', (event) => {
      if (!this.map) {
        return
      }
      const features = this.map.queryRenderedFeatures(event.point, {
        layers: [fillLayerID, lineLayerID, pointLayerID],
      })
      const id = features[0]?.properties?.udbxID
      if (typeof id === 'number') {
        this.featureClickHandler?.(id)
      }
    })

    this.map.on('moveend', () => this.emitViewport())
  }

  destroy(): void {
    this.map?.remove()
    this.map = null
  }

  setSummary(summary: SpatialSummary): void {
    if (summary.extent) {
      this.fitExtent(summary.extent)
    }
  }

  setFeatures(features: PreviewFeature[]): void {
    this.features = features
    this.updateSource()
  }

  fitExtent(extent: BoundingBox): void {
    this.map?.fitBounds(
      [
        [extent.minX, extent.minY],
        [extent.maxX, extent.maxY],
      ],
      { padding: 24, duration: 0 },
    )
  }

  setSelection(featureIDs: number[]): void {
    this.selectedIDs = new Set(featureIDs)
    this.updateSource()
  }

  onFeatureClick(handler: (featureID: number) => void): void {
    this.featureClickHandler = handler
  }

  onViewportChange(handler: (viewport: BoundingBox) => void): void {
    this.viewportChangeHandler = handler
  }

  private installLayers(): void {
    if (!this.map || this.map.getSource(sourceID)) {
      return
    }
    this.map.addSource(sourceID, {
      type: 'geojson',
      data: toFeatureCollection(this.features, this.selectedIDs),
    })
    this.map.addLayer({
      id: fillLayerID,
      type: 'fill',
      source: sourceID,
      filter: ['==', ['geometry-type'], 'Polygon'],
      paint: {
        'fill-color': ['case', ['boolean', ['get', 'selected'], false], '#d9480f', '#1971c2'],
        'fill-opacity': ['case', ['boolean', ['get', 'selected'], false], 0.28, 0.16],
      },
    })
    this.map.addLayer({
      id: lineLayerID,
      type: 'line',
      source: sourceID,
      paint: {
        'line-color': ['case', ['boolean', ['get', 'selected'], false], '#d9480f', '#1971c2'],
        'line-width': ['case', ['boolean', ['get', 'selected'], false], 3, 1.5],
      },
    })
    this.map.addLayer({
      id: pointLayerID,
      type: 'circle',
      source: sourceID,
      filter: ['==', ['geometry-type'], 'Point'],
      paint: {
        'circle-radius': ['case', ['boolean', ['get', 'selected'], false], 6, 4],
        'circle-color': ['case', ['boolean', ['get', 'selected'], false], '#d9480f', '#1971c2'],
        'circle-stroke-color': '#ffffff',
        'circle-stroke-width': 1,
      },
    })
  }

  private updateSource(): void {
    const source = this.map?.getSource(sourceID) as GeoJSONSource | undefined
    source?.setData(toFeatureCollection(this.features, this.selectedIDs))
  }

  private emitViewport(): void {
    const bounds = this.map?.getBounds()
    if (!bounds) {
      return
    }
    this.viewportChangeHandler?.({
      minX: bounds.getWest(),
      minY: bounds.getSouth(),
      maxX: bounds.getEast(),
      maxY: bounds.getNorth(),
    })
  }
}

function toFeatureCollection(features: PreviewFeature[], selectedIDs: Set<number>): FeatureCollection {
  return {
    type: 'FeatureCollection',
    features: features
      .filter((feature) => ['Point', 'MultiLineString', 'MultiPolygon', 'Text'].includes(feature.geometry.type))
      .map((feature) => ({
        type: 'Feature',
        id: feature.id,
        properties: {
          udbxID: feature.id,
          selected: selectedIDs.has(feature.id) ? 1 : 0,
          ...feature.properties,
        },
        geometry: {
          type: feature.geometry.type === 'Text' ? 'Point' : feature.geometry.type,
          coordinates: feature.geometry.coordinates,
        },
      })),
  }
}
