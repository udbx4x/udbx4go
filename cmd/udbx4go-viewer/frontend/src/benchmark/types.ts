import type {
  DatasetInfo,
  FeatureAttributes,
  FileInfo,
  MapLayerState,
  PageData,
  SelectedMapFeature,
  SpatialPreview,
  SpatialSummary,
} from '../types'

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
}

export interface BenchmarkConfig {
  runId: string
  outputPath: string
  scenario: BenchmarkScenario
}

export interface BenchmarkMetrics {
  openFileMs: number
  loadLayersMs: number
  fitVisibleLayersMs: number
  selectAndFitMs: number
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
  setSelection: (selection: SelectedMapFeature) => void
  fitFeature: (datasetName: string, featureID: number) => void
}
