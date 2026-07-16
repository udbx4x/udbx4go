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
    const preview = summary.viewportQuerySupported
      ? null
      : await dependencies.loadSpatialPreview(datasetName, previewRequest)
    const layer: MapLayerState = {
      datasetName,
      kind: summary.kind,
      visible: true,
      style: createDefaultLayerStyle(summary.kind),
      loading: false,
      error: null,
      summary,
      preview,
      queryStatus: preview?.degradedReason ? 'degraded' : 'idle',
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

  if (config.temperature === 'warm') {
    for (const step of config.scenario.viewportSteps) {
      const { hideLayers: _hide, showLayers: _show, removeLayers: _remove, ...warmupStep } = step
      await dependencies.runViewportStep(warmupStep, [featureID])
    }
    dependencies.resetCoordinatorMetrics?.()
  }

  const backendQueryMs: number[] = []
  const moveendToRenderMs: number[] = []
  let finalFeatureCount = 0
  let blankRenderCount = 0
  for (const step of config.scenario.viewportSteps) {
    const viewportResult = await dependencies.runViewportStep(step, [featureID])
    const strategies = viewportResult.strategies ?? [step.expectedStrategy]
    if (strategies.some((strategy) => strategy !== step.expectedStrategy)) {
      throw new Error(`expected strategy ${step.expectedStrategy}, received ${strategies.join(', ')}`)
    }
    backendQueryMs.push(...viewportResult.backendQueryMs)
    moveendToRenderMs.push(viewportResult.moveendToRenderMs)
    finalFeatureCount = viewportResult.finalFeatureCount
    if (viewportResult.blankRender) {
      blankRenderCount += 1
    }
  }
  const coordinatorMetrics = dependencies.getCoordinatorMetrics()

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
      backendQueryMs,
      moveendToRenderMs,
      maxConcurrentQueries: coordinatorMetrics.maxConcurrentQueries,
      pendingPeak: coordinatorMetrics.pendingPeak,
      pendingFinal: coordinatorMetrics.pendingQueries,
      staleResultsDiscarded: coordinatorMetrics.staleResultsDiscarded,
      staleResultApplied: coordinatorMetrics.staleResultApplied,
      finalFeatureCount,
      blankRenderCount,
    },
    error: '',
  }
}
