// Dataset information from backend
export interface DatasetInfo {
  name: string
  kind: string
  objectCount: number
  iconType: string
}

// Page of data from backend
export interface PageData {
  rows: string[][]
  columns: string[]
  currentPage: number
  totalPages: number
}

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
  viewportQuerySupported: boolean
  rtreeAvailable: boolean
  queryDiagnosticReason?: string
}

export interface PreviewGeometry {
  type: string
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
  queriedBounds?: BoundingBox
  strategy: string
  hasMore: boolean
  degradedReason?: string
  queryDurationMs: number
  fileGeneration: number
  viewportFeatureCount?: number
}

export interface FeatureAttributes {
  datasetName: string
  id: number
  geometryType: string
  bbox?: BoundingBox
  properties: Record<string, string>
}

export interface PointLayerStyle {
  radius: number
  fillColor: string
  strokeColor: string
  strokeWidth: number
}

export interface LineLayerStyle {
  strokeColor: string
  strokeWidth: number
}

export interface PolygonLayerStyle {
  fillColor: string
  strokeColor: string
  strokeWidth: number
}

export interface SelectionLayerStyle {
  color: string
  pointRadius: number
  strokeWidth: number
  fillColor: string
}

export interface LayerStyle {
  point: PointLayerStyle
  line: LineLayerStyle
  polygon: PolygonLayerStyle
  selected: SelectionLayerStyle
}

export interface MapLayerState {
  datasetName: string
  kind: string
  visible: boolean
  style: LayerStyle
  loading: boolean
  error: string | null
  summary: SpatialSummary | null
  preview: SpatialPreview | null
  queryStatus: 'idle' | 'loading' | 'ready' | 'degraded' | 'error'
  queryError: string | null
  lastQueriedBounds?: BoundingBox
}

export interface SelectedMapFeature {
  datasetName: string
  featureID: number
}

// File information
export interface FileInfo {
  path: string
  datasetCount: number
  fileGeneration: number
}

// Application state
export interface AppState {
  currentFile: string | null
  datasets: DatasetInfo[]
  selectedDataset: string | null
  activeTableDataset: string | null
  pageData: PageData | null
  mapLayers: MapLayerState[]
  selectedMapFeature: SelectedMapFeature | null
  selectedFeatureAttributes: FeatureAttributes | null
  loading: boolean
  error: string | null
}
