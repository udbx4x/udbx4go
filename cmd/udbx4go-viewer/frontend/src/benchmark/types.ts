import type {
  BoundingBox,
  DatasetInfo,
  FeatureAttributes,
  FileInfo,
  MapLayerState,
  PageData,
  SelectedMapFeature,
  SpatialPreview,
  SpatialSummary,
} from '../types'

export type SpatialQueryStrategy = 'rtree' | 'envelope_cache' | 'bounded_sample'

export interface BenchmarkViewportStep {
  bounds: BoundingBox
  expectedStrategy: SpatialQueryStrategy
  geometryKind?: 'point' | 'line' | 'polygon'
  hideLayers?: string[]
  showLayers?: string[]
  removeLayers?: string[]
}

export interface BenchmarkSelection {
  datasetName: string
  page: number
  rowIndex: number
}

export interface BenchmarkScenario {
  name: string
  filePath: string
  layers: string[]
  selection: BenchmarkSelection
  viewportSteps: BenchmarkViewportStep[]
}

export interface BenchmarkConfig {
  runId: string
  outputPath: string
  temperature: 'cold' | 'warm'
  maxConcurrentQueries: number
  scenario: BenchmarkScenario
}

export interface BenchmarkMetrics {
  openFileMs: number
  loadLayersMs: number
  fitVisibleLayersMs: number
  selectAndFitMs: number
  backendQueryMs: number[]
  moveendToRenderMs: number[]
  maxConcurrentQueries: number
  pendingPeak: number
  pendingFinal: number
  staleResultsDiscarded: number
  staleResultApplied: boolean
  finalFeatureCount: number
  blankRenderCount: number
}

export interface BenchmarkResult {
  runId: string
  status: 'passed' | 'failed'
  startedAt: string
  scenario: string
  metrics: BenchmarkMetrics
  error: string
}

export interface SpatialPreviewRequest {
  limit: number
  maxVertices: number
  simplify: boolean
  viewport?: BoundingBox
  requiredIds?: number[]
}

export interface BenchmarkViewportResult {
  backendQueryMs: number[]
  moveendToRenderMs: number
  finalFeatureCount: number
  blankRender: boolean
  strategies?: string[]
  featureIDs: number[]
}

export interface BenchmarkCoordinatorMetrics {
  maxConcurrentQueries: number
  pendingPeak: number
  activeQueries: number
  pendingQueries: number
  staleResultsDiscarded: number
  staleResultApplied: boolean
}

export interface BenchmarkDependencies {
  now: () => number
  openFile: (path: string) => Promise<FileInfo>
  listDatasets: () => Promise<DatasetInfo[]>
  getSpatialSummary: (datasetName: string) => Promise<SpatialSummary>
  loadSpatialPreview: (datasetName: string, request: SpatialPreviewRequest) => Promise<SpatialPreview>
  loadDatasetPage: (datasetName: string, page: number) => Promise<PageData>
  getFeatureAttributes: (datasetName: string, featureID: number) => Promise<FeatureAttributes>
  setLayer: (layer: MapLayerState) => void
  fitAllVisibleLayers: () => void
  setSelection: (selection: SelectedMapFeature) => Promise<boolean>
  runViewportStep: (step: BenchmarkViewportStep, requiredIDs: number[]) => Promise<BenchmarkViewportResult>
  getCoordinatorMetrics: () => BenchmarkCoordinatorMetrics
  resetCoordinatorMetrics?: () => void
}
