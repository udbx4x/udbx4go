import 'ol/ol.css'

import Feature from 'ol/Feature'
import OlMap from 'ol/Map'
import type MapBrowserEvent from 'ol/MapBrowserEvent'
import View from 'ol/View'
import MultiLineString from 'ol/geom/MultiLineString'
import MultiPolygon from 'ol/geom/MultiPolygon'
import Point from 'ol/geom/Point'
import VectorLayer from 'ol/layer/Vector'
import VectorSource from 'ol/source/Vector'
import { Fill, Stroke, Style, Circle as CircleStyle } from 'ol/style'
import type { BoundingBox, MapLayerState, PreviewFeature, SelectedMapFeature } from '../types'
import type { SpatialRendererAdapter } from './SpatialRendererAdapter'

type GeometryKind = 'point' | 'line' | 'polygon'
type ExtentTuple = [number, number, number, number]
const MAX_PIXEL_READ_PIXELS = 256 * 1024

export class OpenLayersSpatialRendererAdapter implements SpatialRendererAdapter {
  private map: OlMap | null = null
  private container: HTMLElement | null = null
  private layers = new globalThis.Map<string, VectorLayer>()
  private sources = new Map<string, VectorSource>()
  private layerStates = new Map<string, MapLayerState>()
  private selectedFeature: SelectedMapFeature | null = null
  private featureClickHandler: ((datasetName: string, featureID: number) => void) | null = null
  private viewportChangeHandler: ((viewport: BoundingBox) => void) | null = null
  private singleClickListener: ((event: MapBrowserEvent) => void) | null = null
  private moveendListener: (() => void) | null = null
  private pendingRenderWaits = new Set<(error: Error) => void>()

  mount(container: HTMLElement): void {
    this.destroy()
    this.container = container
    try {
      this.map = new OlMap({
        target: container,
        layers: [],
        view: new View({
          center: [0, 0],
          zoom: 2,
        }),
      })

      this.singleClickListener = (event) => {
        const feature = this.map?.forEachFeatureAtPixel(event.pixel, (candidate) => candidate)
        const datasetName = feature?.get('datasetName')
        const featureID = feature?.get('featureID')
        if (typeof datasetName === 'string' && typeof featureID === 'number') {
          this.featureClickHandler?.(datasetName, featureID)
        }
      }
      this.map.on('singleclick', this.singleClickListener)

      this.moveendListener = () => this.emitViewport()
      this.map.on('moveend', this.moveendListener)
      this.map.renderSync()
    } catch (error) {
      this.destroy()
      throw error
    }
  }

  destroy(): void {
    const map = this.map
    const singleClickListener = this.singleClickListener
    const moveendListener = this.moveendListener
    this.map = null
    this.container = null
    this.singleClickListener = null
    this.moveendListener = null

    const error = new Error('OpenLayers map was destroyed before rendercomplete')
    for (const cancel of [...this.pendingRenderWaits]) {
      cancel(error)
    }

    if (map && singleClickListener) {
      map.un('singleclick', singleClickListener)
    }
    if (map && moveendListener) {
      map.un('moveend', moveendListener)
    }
    map?.setTarget(undefined)
    this.sources.forEach((source) => source.clear())
    this.sources.clear()
    this.layers.clear()
    this.layerStates.clear()
  }

  setLayer(layer: MapLayerState): void {
    if (!this.map || !layer.preview) {
      this.layerStates.set(layer.datasetName, layer)
      return
    }

    const features = layer.preview.features.map((feature) => {
      const geometry = toOpenLayersGeometry(feature)
      const olFeature = new Feature({ geometry })
      olFeature.set('datasetName', layer.datasetName)
      olFeature.set('featureID', feature.id)
      olFeature.set('geometryKind', geometryKindFromPreviewType(feature.geometry.type))
      return olFeature
    })
    this.layerStates.set(layer.datasetName, layer)

    let source = this.sources.get(layer.datasetName)
    let vectorLayer = this.layers.get(layer.datasetName)
    if (!source || !vectorLayer) {
      source = new VectorSource()
      vectorLayer = new VectorLayer({
        source,
        visible: layer.visible,
        style: (feature) => this.createStyle(String(feature.get('datasetName')), Number(feature.get('featureID'))),
      })
      this.sources.set(layer.datasetName, source)
      this.layers.set(layer.datasetName, vectorLayer)
      this.map.addLayer(vectorLayer)
    }

    source.clear(true)
    source.addFeatures(features)
    vectorLayer.setVisible(layer.visible)
    this.map.updateSize()
  }

  removeLayer(datasetName: string): void {
    const layer = this.layers.get(datasetName)
    if (layer && this.map) {
      this.map.removeLayer(layer)
    }
    this.layers.delete(datasetName)
    this.sources.delete(datasetName)
    this.layerStates.delete(datasetName)
  }

  setLayerVisible(datasetName: string, visible: boolean): void {
    this.layers.get(datasetName)?.setVisible(visible)
  }

  fitAllVisibleLayers(): void {
    if (!this.map) {
      return
    }

    let extent: [number, number, number, number] | null = null
    this.sources.forEach((source, datasetName) => {
      if (!this.layers.get(datasetName)?.getVisible()) {
        return
      }
      const sourceExtent = source.getExtent()
      if (!sourceExtent || sourceExtent.some((value) => !Number.isFinite(value))) {
        return
      }
      extent = extent
        ? [
            Math.min(extent[0], sourceExtent[0]),
            Math.min(extent[1], sourceExtent[1]),
            Math.max(extent[2], sourceExtent[2]),
            Math.max(extent[3], sourceExtent[3]),
          ]
        : [sourceExtent[0], sourceExtent[1], sourceExtent[2], sourceExtent[3]]
    })

    if (extent) {
      this.map.getView().fit(extent, { padding: [24, 24, 24, 24], duration: 0 })
    }
  }

  fitFeature(datasetName: string, featureID: number): void {
    const source = this.sources.get(datasetName)
    const feature = source?.getFeatures().find((candidate) => Number(candidate.get('featureID')) === featureID)
    const featureExtent = feature?.getGeometry()?.getExtent()
    const sourceExtent = source?.getExtent()
    const geometryKind = feature?.get('geometryKind') as GeometryKind | undefined
    if (!this.map || !featureExtent || !sourceExtent || !geometryKind) {
      return
    }
    const extent = calculateSelectedFeatureFitExtent(
      [featureExtent[0], featureExtent[1], featureExtent[2], featureExtent[3]],
      [sourceExtent[0], sourceExtent[1], sourceExtent[2], sourceExtent[3]],
      geometryKind,
    )
    if (!extent) {
      return
    }
    this.map.getView().fit(extent, { padding: [64, 64, 64, 64], duration: 0 })
  }

  fitBounds(bounds: BoundingBox, geometryKind: GeometryKind): void {
    if (!this.map) {
      return
    }
    const boundsExtent: ExtentTuple = [bounds.minX, bounds.minY, bounds.maxX, bounds.maxY]
    const extent = calculateSelectedFeatureFitExtent(boundsExtent, boundsExtent, geometryKind)
    if (!extent) {
      return
    }
    this.map.getView().fit(extent, { padding: [64, 64, 64, 64], duration: 0 })
  }

  setSelection(selection: SelectedMapFeature | null): void {
    this.selectedFeature = selection
    this.sources.forEach((source) => source.changed())
    this.map?.render()
  }

  onFeatureClick(handler: (datasetName: string, featureID: number) => void): void {
    this.featureClickHandler = handler
  }

  getViewport(): BoundingBox | null {
    const extent = this.map?.getView().calculateExtent(this.map.getSize())
    if (!extent || !isValidExtent([extent[0], extent[1], extent[2], extent[3]])) {
      return null
    }
    return {
      minX: extent[0],
      minY: extent[1],
      maxX: extent[2],
      maxY: extent[3],
    }
  }

  onViewportChange(handler: (viewport: BoundingBox) => void): () => void {
    this.viewportChangeHandler = handler
    return () => {
      if (this.viewportChangeHandler === handler) {
        this.viewportChangeHandler = null
      }
    }
  }

  waitForRenderComplete(timeoutMS = 5000): Promise<void> {
    const map = this.map
    if (!map) {
      return Promise.reject(new Error('OpenLayers map is not mounted'))
    }
    return new Promise((resolve, reject) => {
      let settled = false
      let timeoutID: number | null = null
      const cleanup = () => {
        if (settled) {
          return false
        }
        settled = true
        if (timeoutID !== null) {
          window.clearTimeout(timeoutID)
        }
        map.un('rendercomplete', finish)
        this.pendingRenderWaits.delete(cancel)
        return true
      }
      const finish = () => {
        if (cleanup()) {
          resolve()
        }
      }
      const cancel = (error: Error) => {
        if (cleanup()) {
          reject(error)
        }
      }
      this.pendingRenderWaits.add(cancel)
      map.once('rendercomplete', finish)
      timeoutID = window.setTimeout(() => {
        cancel(new Error(`rendercomplete timed out after ${timeoutMS}ms`))
      }, timeoutMS)
      try {
        map.renderSync()
      } catch (error) {
        cancel(asError(error))
      }
    })
  }

  hasRenderedFeaturePixels(): boolean {
    const canvases = this.container?.querySelectorAll<HTMLCanvasElement>('.ol-layer canvas') ?? []
    return Array.from(canvases).some((canvas) => {
      if (canvas.width <= 0 || canvas.height <= 0) {
        return false
      }
      try {
        const context = canvas.getContext('2d', { willReadFrequently: true })
        if (!context) {
          return false
        }
        const chunkWidth = Math.min(canvas.width, MAX_PIXEL_READ_PIXELS)
        const chunkHeight = Math.max(1, Math.floor(MAX_PIXEL_READ_PIXELS / chunkWidth))
        for (let y = 0; y < canvas.height; y += chunkHeight) {
          const readHeight = Math.min(chunkHeight, canvas.height - y)
          for (let x = 0; x < canvas.width; x += chunkWidth) {
            const readWidth = Math.min(chunkWidth, canvas.width - x)
            const pixels = context.getImageData(x, y, readWidth, readHeight).data
            for (let index = 3; index < pixels.length; index += 4) {
              if (pixels[index] !== 0) {
                return true
              }
            }
          }
        }
      } catch {
        return false
      }
      return false
    })
  }

  getVisibleFeatureCount(): number {
    let count = 0
    this.sources.forEach((source, datasetName) => {
      if (this.layers.get(datasetName)?.getVisible()) {
        count += source.getFeatures().length
      }
    })
    return count
  }

  hasFeature(datasetName: string, featureID: number): boolean {
    return Boolean(this.sources.get(datasetName)?.getFeatures().some((feature) =>
      Number(feature.get('featureID')) === featureID,
    ))
  }

  isSelectionHighlighted(selection: SelectedMapFeature): boolean {
    return this.selectedFeature?.datasetName === selection.datasetName
      && this.selectedFeature.featureID === selection.featureID
      && this.hasFeature(selection.datasetName, selection.featureID)
  }

  private emitViewport(): void {
    const viewport = this.getViewport()
    if (!viewport) {
      return
    }
    this.viewportChangeHandler?.(viewport)
  }

  private createStyle(datasetName: string, featureID: number): Style {
    const sourceFeature = this.sources
      .get(datasetName)
      ?.getFeatures()
      .find((feature) => Number(feature.get('featureID')) === featureID)
    const geometryKind = sourceFeature?.get('geometryKind') as 'point' | 'line' | 'polygon' | undefined
    const mapLayerState = this.layerStates.get(datasetName)
    const style = mapLayerState?.style
    const selected = this.selectedFeature?.datasetName === datasetName && this.selectedFeature.featureID === featureID
    const selectedStyle = style?.selected
    const pointStyle = style?.point ?? { radius: 4, fillColor: '#1971c2', strokeColor: '#ffffff', strokeWidth: 1 }
    const lineStyle = style?.line ?? { strokeColor: '#1971c2', strokeWidth: 1.5 }
    const polygonStyle = style?.polygon ?? { fillColor: 'rgba(25,113,194,0.16)', strokeColor: '#1971c2', strokeWidth: 1.5 }

    if (geometryKind === 'polygon') {
      return new Style({
        stroke: new Stroke({
          color: selected ? selectedStyle?.color ?? '#d9480f' : polygonStyle.strokeColor,
          width: selected ? selectedStyle?.strokeWidth ?? 3 : polygonStyle.strokeWidth,
        }),
        fill: new Fill({
          color: selected ? selectedStyle?.fillColor ?? 'rgba(217,72,15,0.24)' : polygonStyle.fillColor,
        }),
      })
    }

    if (geometryKind === 'line') {
      return new Style({
        stroke: new Stroke({
          color: selected ? selectedStyle?.color ?? '#d9480f' : lineStyle.strokeColor,
          width: selected ? selectedStyle?.strokeWidth ?? 3 : lineStyle.strokeWidth,
        }),
      })
    }

    return new Style({
      image: new CircleStyle({
        radius: selected ? selectedStyle?.pointRadius ?? 6 : pointStyle.radius,
        fill: new Fill({ color: selected ? selectedStyle?.color ?? '#d9480f' : pointStyle.fillColor }),
        stroke: new Stroke({ color: pointStyle.strokeColor, width: pointStyle.strokeWidth }),
      }),
    })
  }
}

function asError(error: unknown): Error {
  return error instanceof Error ? error : new Error(String(error))
}

function toOpenLayersGeometry(feature: PreviewFeature): Point | MultiLineString | MultiPolygon {
  switch (feature.geometry.type) {
    case 'Point':
    case 'Text':
      return new Point(feature.geometry.coordinates as number[])
    case 'MultiLineString':
      return new MultiLineString(feature.geometry.coordinates as number[][][])
    case 'MultiPolygon':
      return new MultiPolygon(feature.geometry.coordinates as number[][][][])
    default:
      throw new Error(`不支持的预览几何类型：${feature.geometry.type}`)
  }
}

function geometryKindFromPreviewType(type: string): GeometryKind {
  switch (type) {
    case 'MultiLineString':
    case 'LineString':
      return 'line'
    case 'MultiPolygon':
    case 'Polygon':
      return 'polygon'
    default:
      return 'point'
  }
}

export function calculateSelectedFeatureFitExtent(
  featureExtent: ExtentTuple,
  layerExtent: ExtentTuple,
  geometryKind: GeometryKind,
): ExtentTuple | null {
  if (!isValidExtent(featureExtent)) {
    return null
  }

  const effectiveLayerExtent = isValidExtent(layerExtent) ? layerExtent : featureExtent
  const featureWidth = Math.max(0, featureExtent[2] - featureExtent[0])
  const featureHeight = Math.max(0, featureExtent[3] - featureExtent[1])
  const layerWidth = Math.max(0, effectiveLayerExtent[2] - effectiveLayerExtent[0])
  const layerHeight = Math.max(0, effectiveLayerExtent[3] - effectiveLayerExtent[1])

  const centerX = (featureExtent[0] + featureExtent[2]) / 2
  const centerY = (featureExtent[1] + featureExtent[3]) / 2
  const settings = selectedFeatureFitSettings(geometryKind)
  const fallbackSpan = Math.max(layerWidth, layerHeight, featureWidth, featureHeight, 1)
  const minWidth = Math.max(layerWidth * settings.minLayerRatio, fallbackSpan * 0.01)
  const minHeight = Math.max(layerHeight * settings.minLayerRatio, fallbackSpan * 0.01)
  const targetWidth = Math.max(featureWidth * (1 + settings.marginRatio * 2), minWidth)
  const targetHeight = Math.max(featureHeight * (1 + settings.marginRatio * 2), minHeight)

  return [
    centerX - targetWidth / 2,
    centerY - targetHeight / 2,
    centerX + targetWidth / 2,
    centerY + targetHeight / 2,
  ]
}

function selectedFeatureFitSettings(geometryKind: GeometryKind) {
  switch (geometryKind) {
    case 'line':
      return { marginRatio: 0.15, minLayerRatio: 0.15 }
    case 'polygon':
      return { marginRatio: 0.2, minLayerRatio: 0.12 }
    default:
      return { marginRatio: 0, minLayerRatio: 0.08 }
  }
}

function isValidExtent(extent: ExtentTuple): boolean {
  return extent.every((value) => Number.isFinite(value)) && extent[0] <= extent[2] && extent[1] <= extent[3]
}
