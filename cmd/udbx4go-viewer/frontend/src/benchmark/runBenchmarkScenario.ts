import { createDefaultLayerStyle } from '../spatial/layerStyle'
import type { MapLayerState } from '../types'
import type { BenchmarkConfig, BenchmarkDependencies, BenchmarkResult } from './types'

const previewRequest = {
  limit: 1000,
  maxVertices: 1000000,
  simplify: false,
}

export async function runBenchmarkScenario(
  config: BenchmarkConfig,
  dependencies: BenchmarkDependencies,
): Promise<BenchmarkResult> {
  const startedAt = new Date().toISOString()

  let started = dependencies.now()
  await dependencies.openFile(config.scenario.filePath)
  const datasets = await dependencies.listDatasets()
  const openFileMs = dependencies.now() - started

  const datasetNames = new Set(datasets.map((dataset) => dataset.name))
  for (const datasetName of config.scenario.layers) {
    if (!datasetNames.has(datasetName)) {
      throw new Error(`基准图层数据集不存在: ${datasetName}`)
    }
  }

  started = dependencies.now()
  for (const datasetName of config.scenario.layers) {
    const summary = await dependencies.getSpatialSummary(datasetName)
    if (!summary.previewSupported) {
      throw new Error(summary.unsupportedReason || `数据集不支持空间预览: ${datasetName}`)
    }
    const preview = await dependencies.loadSpatialPreview(datasetName, previewRequest)
    const layer: MapLayerState = {
      datasetName,
      kind: summary.kind,
      visible: true,
      style: createDefaultLayerStyle(summary.kind),
      loading: false,
      error: null,
      summary,
      preview,
      queryStatus: preview.degradedReason ? 'degraded' : 'ready',
      queryError: null,
    }
    dependencies.setLayer(layer)
  }
  const loadLayersMs = dependencies.now() - started

  started = dependencies.now()
  dependencies.fitAllVisibleLayers()
  const fitVisibleLayersMs = dependencies.now() - started

  started = dependencies.now()
  const selection = config.scenario.selection
  const page = await dependencies.loadDatasetPage(selection.datasetName, selection.page)
  if (selection.rowIndex >= page.rows.length) {
    throw new Error(`selection.rowIndex ${selection.rowIndex} 超出第 ${selection.page} 页记录范围`)
  }
  const smIDColumn = page.columns.findIndex((column) => column === 'SmID')
  if (smIDColumn < 0) {
    throw new Error('属性表缺少 SmID 列')
  }
  const rawFeatureID = page.rows[selection.rowIndex][smIDColumn]
  const featureID = Number(rawFeatureID)
  if (!/^\d+$/.test(rawFeatureID) || !Number.isSafeInteger(featureID) || featureID <= 0) {
    throw new Error(`属性表 SmID 非法: ${rawFeatureID}`)
  }
  await dependencies.getFeatureAttributes(selection.datasetName, featureID)
  dependencies.setSelection({ datasetName: selection.datasetName, featureID })
  dependencies.fitFeature(selection.datasetName, featureID)
  const selectAndFitMs = dependencies.now() - started

  return {
    runId: config.runId,
    status: 'passed',
    startedAt,
    scenario: config.scenario.name,
    metrics: {
      openFileMs,
      loadLayersMs,
      fitVisibleLayersMs,
      selectAndFitMs,
    },
    error: '',
  }
}
