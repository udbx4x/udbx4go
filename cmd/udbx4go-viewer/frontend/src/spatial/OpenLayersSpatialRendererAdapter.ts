import 'ol/ol.css'

import Feature from 'ol/Feature'
import Map from 'ol/Map'
import View from 'ol/View'
import MultiLineString from 'ol/geom/MultiLineString'
import MultiPolygon from 'ol/geom/MultiPolygon'
import Point from 'ol/geom/Point'
import VectorLayer from 'ol/layer/Vector'
import VectorSource from 'ol/source/Vector'
import { Fill, Stroke, Style, Circle as CircleStyle } from 'ol/style'
import type { BoundingBox, PreviewFeature, SpatialSummary } from './types'
import type { SpatialRendererAdapter } from './SpatialRendererAdapter'

export class OpenLayersSpatialRendererAdapter implements SpatialRendererAdapter {
  private map: Map | null = null
  private source = new VectorSource()
  private selectedIDs = new Set<number>()
  private featureClickHandler: ((featureID: number) => void) | null = null
  private viewportChangeHandler: ((viewport: BoundingBox) => void) | null = null

  mount(container: HTMLElement): void {
    const layer = new VectorLayer({
      source: this.source,
      style: (feature) => {
        const id = Number(feature.get('udbxID'))
        const selected = this.selectedIDs.has(id)
        return new Style({
          image: new CircleStyle({
            radius: selected ? 6 : 4,
            fill: new Fill({ color: selected ? '#d9480f' : '#1971c2' }),
            stroke: new Stroke({ color: '#ffffff', width: 1 }),
          }),
          stroke: new Stroke({ color: selected ? '#d9480f' : '#1971c2', width: selected ? 3 : 1.5 }),
          fill: new Fill({ color: selected ? 'rgba(217,72,15,0.24)' : 'rgba(25,113,194,0.16)' }),
        })
      },
    })

    this.map = new Map({
      target: container,
      layers: [layer],
      view: new View({
        center: [0, 0],
        zoom: 2,
      }),
    })

    this.map.on('singleclick', (event) => {
      const feature = this.map?.forEachFeatureAtPixel(event.pixel, (candidate) => candidate)
      const id = feature?.get('udbxID')
      if (typeof id === 'number') {
        this.featureClickHandler?.(id)
      }
    })

    this.map.getView().on('change:center', () => this.emitViewport())
    this.map.getView().on('change:resolution', () => this.emitViewport())
  }

  destroy(): void {
    this.map?.setTarget(undefined)
    this.map = null
    this.source.clear()
  }

  setSummary(summary: SpatialSummary): void {
    if (summary.extent) {
      this.fitExtent(summary.extent)
    }
  }

  setFeatures(features: PreviewFeature[]): void {
    this.source.clear()
    this.source.addFeatures(features.flatMap((feature) => {
      const geometry = toOpenLayersGeometry(feature)
      if (!geometry) {
        return []
      }
      const olFeature = new Feature({ geometry })
      olFeature.set('udbxID', feature.id)
      return [olFeature]
    }))
  }

  fitExtent(extent: BoundingBox): void {
    this.map?.getView().fit([extent.minX, extent.minY, extent.maxX, extent.maxY], {
      padding: [24, 24, 24, 24],
      duration: 0,
    })
  }

  setSelection(featureIDs: number[]): void {
    this.selectedIDs = new Set(featureIDs)
    this.source.changed()
  }

  onFeatureClick(handler: (featureID: number) => void): void {
    this.featureClickHandler = handler
  }

  onViewportChange(handler: (viewport: BoundingBox) => void): void {
    this.viewportChangeHandler = handler
  }

  private emitViewport(): void {
    const extent = this.map?.getView().calculateExtent(this.map.getSize())
    if (!extent) {
      return
    }
    this.viewportChangeHandler?.({
      minX: extent[0],
      minY: extent[1],
      maxX: extent[2],
      maxY: extent[3],
    })
  }
}

function toOpenLayersGeometry(feature: PreviewFeature): Point | MultiLineString | MultiPolygon | null {
  switch (feature.geometry.type) {
    case 'Point':
    case 'Text':
      return new Point(feature.geometry.coordinates as number[])
    case 'MultiLineString':
      return new MultiLineString(feature.geometry.coordinates as number[][][])
    case 'MultiPolygon':
      return new MultiPolygon(feature.geometry.coordinates as number[][][][])
    default:
      return null
  }
}
