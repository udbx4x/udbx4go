import { createDefaultLayerStyle } from '../spatial/layerStyle'
import { featureGeometryKind, isValidBounds } from '../spatial/featureLocation'
import type { BoundingBox, MapLayerState } from '../types'
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
  const viewportSteps = config.scenario.viewportSteps
  if (viewportSteps.length < 2) {
    throw new Error('stale viewport probe requires at least two viewport steps')
  }
  if (boundsEqual(viewportSteps[0].bounds, viewportSteps[viewportSteps.length - 1].bounds)) {
    throw new Error('stale viewport probe requires different first and latest bounds')
  }

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

  const selectionPreparationStarted = dependencies.now()
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
  const attributes = await dependencies.getFeatureAttributes(selection.datasetName, featureID)
  const geometryKind = featureGeometryKind(attributes.geometryType)
  if (!isValidBounds(attributes.bbox) || geometryKind === null) {
    throw new Error(`选中要素缺少可定位 bbox 或 geometryType: ${selection.datasetName}/${featureID}`)
  }
  const selectionStep = {
    bounds: attributes.bbox,
    expectedStrategy: config.scenario.viewportSteps[0].expectedStrategy,
    geometryKind,
  }
  const selectionPreparationMs = dependencies.now() - selectionPreparationStarted
  const validateSelection = async () => {
    const validationStarted = dependencies.now()
    const selectionResult = await dependencies.runViewportStep(selectionStep, [featureID])
    assertExpectedStrategies(
      'selection validation',
      selectionStep.expectedStrategy,
      selectionResult.strategies,
    )
    if (!selectionResult.featureIDs.includes(featureID)) {
      throw new Error(`required 选中 ID ${featureID} 不在查询返回或地图 source 中`)
    }
    if (!await dependencies.setSelection({ datasetName: selection.datasetName, featureID })) {
      throw new Error(`选中 ID ${featureID} 未形成地图高亮状态`)
    }
    return dependencies.now() - validationStarted
  }

  let selectAndFitMs = selectionPreparationMs
  if (config.temperature === 'warm') {
    selectAndFitMs += await validateSelection()
  }

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
  for (const [index, step] of config.scenario.viewportSteps.entries()) {
    const requiredIDs = config.temperature === 'warm' ? [featureID] : []
    const viewportResult = await dependencies.runViewportStep(step, requiredIDs)
    assertExpectedStrategies(`viewport step ${index + 1}`, step.expectedStrategy, viewportResult.strategies)
    backendQueryMs.push(...viewportResult.backendQueryMs)
    moveendToRenderMs.push(viewportResult.moveendToRenderMs)
    finalFeatureCount = viewportResult.finalFeatureCount
    if (viewportResult.blankRender) {
      blankRenderCount += 1
    }
  }
  const firstViewportStep = withoutLayerActions(config.scenario.viewportSteps[0])
  const latestViewportStep = withoutLayerActions(
    config.scenario.viewportSteps[config.scenario.viewportSteps.length - 1],
  )
  const staleProbeResult = await dependencies.runStaleViewportProbe(
    firstViewportStep,
    latestViewportStep,
    config.temperature === 'warm' ? [featureID] : [],
  )
  assertExpectedStrategies(
    'stale viewport probe',
    latestViewportStep.expectedStrategy,
    staleProbeResult.strategies,
  )
  backendQueryMs.push(...staleProbeResult.backendQueryMs)
  moveendToRenderMs.push(staleProbeResult.moveendToRenderMs)
  finalFeatureCount = staleProbeResult.finalFeatureCount
  if (staleProbeResult.blankRender) {
    blankRenderCount += 1
  }
  const coordinatorMetrics = dependencies.getCoordinatorMetrics()
  if (coordinatorMetrics.staleResultsDiscarded < 1) {
    throw new Error('stale viewport probe did not discard an obsolete result')
  }
  if (config.temperature === 'cold') {
    selectAndFitMs += await validateSelection()
  }

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

function withoutLayerActions(step: BenchmarkConfig['scenario']['viewportSteps'][number]) {
  const { hideLayers: _hide, showLayers: _show, removeLayers: _remove, ...probeStep } = step
  return probeStep
}

function boundsEqual(left: BoundingBox, right: BoundingBox): boolean {
  return left.minX === right.minX &&
    left.minY === right.minY &&
    left.maxX === right.maxX &&
    left.maxY === right.maxY
}

function assertExpectedStrategies(
  context: string,
  expectedStrategy: string,
  strategies: string[] | undefined,
): void {
  if (!strategies?.length) {
    throw new Error(`${context}: missing strategy evidence for ${expectedStrategy}`)
  }
  if (strategies.some((strategy) => strategy !== expectedStrategy)) {
    throw new Error(`${context}: expected strategy ${expectedStrategy}, received ${strategies.join(', ')}`)
  }
}
