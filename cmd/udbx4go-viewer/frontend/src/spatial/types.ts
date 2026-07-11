export interface BoundingBox {
  minX: number
  minY: number
  maxX: number
  maxY: number
}

export interface SpatialSummary {
  datasetName: string
  kind: string
  srid?: number
  extent?: BoundingBox
  objectCount: number
  estimatedVertexCount: number
  previewSupported: boolean
  unsupportedReason?: string
}

export interface PreviewGeometry {
  type: 'Point' | 'MultiLineString' | 'MultiPolygon' | 'Text' | string
  coordinates: unknown[]
  hasZ: boolean
}

export interface PreviewFeature {
  id: number
  geometry: PreviewGeometry
  bbox?: BoundingBox
  properties?: Record<string, string>
}

export interface SpatialPreview {
  datasetName: string
  kind: string
  srid?: number
  extent?: BoundingBox
  features: PreviewFeature[]
  estimatedVertexCount: number
  sampled: boolean
  sampleReason?: string
}
