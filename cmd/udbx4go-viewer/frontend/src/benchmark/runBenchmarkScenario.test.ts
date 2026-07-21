import { describe, expect, it, vi } from 'vitest'
import { runBenchmarkScenario } from './runBenchmarkScenario'
import type { BenchmarkConfig, BenchmarkDependencies } from './types'

const config: BenchmarkConfig = {
  runId: 'henan-01',
  outputPath: '/tmp/henan-01.json',
  temperature: 'warm',
  maxConcurrentQueries: 2,
  scenario: {
    name: 'henan-county-envelope-selection',
    filePath: '/data/henan.udbx',
    layers: ['县级行政区划'],
    selection: {
      datasetName: '县级行政区划',
      page: 2,
      rowIndex: 0,
    },
    viewportSteps: [
      {
        bounds: { minX: 110.3, minY: 31.3, maxX: 116.7, maxY: 36.4 },
        expectedStrategy: 'envelope_cache',
        hideLayers: ['县级行政区划'],
      },
      {
        bounds: { minX: 113, minY: 33, maxX: 115, maxY: 35 },
        expectedStrategy: 'envelope_cache',
      },
    ],
  },
}

function createDependencies(calls: string[]): BenchmarkDependencies {
  const ticks = [0, 10, 20, 50, 60, 62, 70, 90]
  return {
    now: () => ticks.shift() ?? 90,
    openFile: async (path) => {
      calls.push(`open:${path}`)
      return { path, datasetCount: 1, fileGeneration: 1 }
    },
    listDatasets: async () => {
      calls.push('list')
      return [{ name: '县级行政区划', kind: 'region', objectCount: 164, iconType: 'region' }]
    },
    getSpatialSummary: async (datasetName) => {
      calls.push(`summary:${datasetName}`)
      return {
        datasetName,
        kind: 'region',
        objectCount: 164,
        estimatedVertexCount: 1000,
        previewSupported: true,
        viewportQuerySupported: true,
        rtreeAvailable: true,
      }
    },
    loadSpatialPreview: async (datasetName, request) => {
      calls.push(`preview:${datasetName}`)
      expect(request).toMatchObject({ limit: 1000, maxVertices: 1000000, simplify: false })
      return {
        datasetName,
        kind: 'region',
        features: [{
          id: 101,
          geometry: { type: 'MultiPolygon', coordinates: [], hasZ: false },
        }],
        estimatedVertexCount: 1000,
        sampled: false,
        strategy: 'envelope_cache',
        hasMore: false,
        queryDurationMs: 1,
        fileGeneration: 0,
      }
    },
    loadDatasetPage: async (datasetName, page) => {
      calls.push(`page:${datasetName}:${page}`)
      return {
        columns: ['SmID', 'Geometry', 'Name'],
        rows: [['101', 'MultiPolygon', '示例县']],
        currentPage: 2,
        totalPages: 2,
      }
    },
    getFeatureAttributes: async (datasetName, featureID) => {
      calls.push(`attributes:${datasetName}:${featureID}`)
      return {
        datasetName,
        id: featureID,
        geometryType: 'MultiPolygon',
        bbox: { minX: 112, minY: 32, maxX: 113, maxY: 33 },
        properties: { Name: '示例县' },
      }
    },
    setLayer: (layer) => calls.push(`setLayer:${layer.datasetName}`),
    fitAllVisibleLayers: () => calls.push('fitAll'),
    setSelection: async (selection) => {
      calls.push(`setSelection:${selection.datasetName}:${selection.featureID}`)
      return true
    },
    runViewportStep: async (step, requiredIDs) => {
      const location = step.geometryKind ? `:${step.bounds.minX}:${step.geometryKind}` : ''
      calls.push(`viewport:${step.expectedStrategy}:${requiredIDs.join(',')}:${step.hideLayers ? 'action' : 'plain'}${location}`)
      return {
        backendQueryMs: [8],
        moveendToRenderMs: 24,
        finalFeatureCount: 164,
        blankRender: false,
        strategies: [step.expectedStrategy],
        featureIDs: [101],
      }
    },
    runStaleViewportProbe: async (first, latest, requiredIDs) => {
      calls.push(`stale:${first.bounds.minX}:${latest.bounds.minX}:${requiredIDs.join(',')}:${first.hideLayers || latest.hideLayers ? 'action' : 'plain'}`)
      return {
        backendQueryMs: [13, 7],
        moveendToRenderMs: 31,
        finalFeatureCount: 23,
        blankRender: false,
        strategies: [latest.expectedStrategy],
        featureIDs: [101],
      }
    },
    getCoordinatorMetrics: () => {
      calls.push('metrics')
      return {
        maxConcurrentQueries: 2,
        pendingPeak: 1,
        activeQueries: 0,
        pendingQueries: 0,
        staleResultsDiscarded: 1,
        staleResultApplied: false,
      }
    },
  }
}

function setMeasuredStepStrategies(
  dependencies: BenchmarkDependencies,
  strategies: string[] | undefined,
): void {
  const runViewportStep = dependencies.runViewportStep
  dependencies.runViewportStep = async (step, requiredIDs) => {
    const result = await runViewportStep(step, requiredIDs)
    return {
      ...result,
      strategies: step.hideLayers ? strategies : [step.expectedStrategy],
    }
  }
}

describe('runBenchmarkScenario', () => {
  it('stale probe 拒绝只有一个 viewport step 的场景且不启动副作用', async () => {
    const calls: string[] = []
    const singleStepConfig: BenchmarkConfig = {
      ...config,
      scenario: {
        ...config.scenario,
        viewportSteps: [config.scenario.viewportSteps[0]],
      },
    }

    await expect(runBenchmarkScenario(singleStepConfig, createDependencies(calls))).rejects.toThrow(
      'stale viewport probe requires at least two viewport steps',
    )
    expect(calls).toEqual([])
  })

  it('stale probe 拒绝首尾 bounds 相同的场景且不启动副作用', async () => {
    const calls: string[] = []
    const repeatedBoundsConfig: BenchmarkConfig = {
      ...config,
      scenario: {
        ...config.scenario,
        viewportSteps: [
          config.scenario.viewportSteps[0],
          {
            ...config.scenario.viewportSteps[1],
            bounds: { ...config.scenario.viewportSteps[0].bounds },
          },
        ],
      },
    }

    await expect(runBenchmarkScenario(repeatedBoundsConfig, createDependencies(calls))).rejects.toThrow(
      'stale viewport probe requires different first and latest bounds',
    )
    expect(calls).toEqual([])
  })

  it('第二页选择成为 required ID，并按固定视口等待查询与渲染', async () => {
    const calls: string[] = []

    const result = await runBenchmarkScenario(config, createDependencies(calls))

    expect(calls).toEqual([
      'open:/data/henan.udbx',
      'list',
      'summary:县级行政区划',
      'setLayer:县级行政区划',
      'fitAll',
      'page:县级行政区划:2',
      'attributes:县级行政区划:101',
      'viewport:envelope_cache:101:plain:112:polygon',
      'setSelection:县级行政区划:101',
      'viewport:envelope_cache:101:plain',
      'viewport:envelope_cache:101:plain',
      'viewport:envelope_cache:101:action',
      'viewport:envelope_cache:101:plain',
      'stale:110.3:113:101:plain',
      'metrics',
    ])
    expect(result.status).toBe('passed')
    expect(result.metrics.backendQueryMs).toEqual([8, 8, 13, 7])
    expect(result.metrics.moveendToRenderMs).toEqual([24, 24, 31])
    expect(result.metrics).toMatchObject({
      maxConcurrentQueries: 2,
      pendingPeak: 1,
      pendingFinal: 0,
      staleResultsDiscarded: 1,
      staleResultApplied: false,
      finalFeatureCount: 23,
      blankRenderCount: 0,
    })
  })

  it('cold 先测正式视口序列，再做不计入 cold metrics 的 selection 验证', async () => {
    const calls: string[] = []
    const coldConfig = { ...config, temperature: 'cold' as const }

    const result = await runBenchmarkScenario(coldConfig, createDependencies(calls))

    expect(calls).toEqual([
      'open:/data/henan.udbx',
      'list',
      'summary:县级行政区划',
      'setLayer:县级行政区划',
      'fitAll',
      'page:县级行政区划:2',
      'attributes:县级行政区划:101',
      'viewport:envelope_cache::action',
      'viewport:envelope_cache::plain',
      'stale:110.3:113::plain',
      'metrics',
      'viewport:envelope_cache:101:plain:112:polygon',
      'setSelection:县级行政区划:101',
    ])
    expect(result.metrics.backendQueryMs).toEqual([8, 8, 13, 7])
    expect(result.metrics.moveendToRenderMs).toEqual([24, 24, 31])
  })

  it('真实 stale probe 未丢弃旧结果时拒绝通过', async () => {
    const dependencies = createDependencies([])
    dependencies.getCoordinatorMetrics = () => ({
      maxConcurrentQueries: 1,
      pendingPeak: 1,
      activeQueries: 0,
      pendingQueries: 0,
      staleResultsDiscarded: 0,
      staleResultApplied: false,
    })

    await expect(runBenchmarkScenario(config, dependencies)).rejects.toThrow(
      'stale viewport probe did not discard an obsolete result',
    )
  })

  it('stale probe 必须返回 latest 策略证据', async () => {
    const dependencies = createDependencies([])
    dependencies.runStaleViewportProbe = async () => ({
      backendQueryMs: [1, 1],
      moveendToRenderMs: 2,
      finalFeatureCount: 1,
      blankRender: false,
      strategies: ['bounded_sample'],
      featureIDs: [101],
    })

    await expect(runBenchmarkScenario(config, dependencies)).rejects.toThrow(
      'stale viewport probe: expected strategy envelope_cache, received bounded_sample',
    )
  })

  it('策略不匹配时拒绝通过', async () => {
    const calls: string[] = []
    const dependencies = createDependencies(calls)
    setMeasuredStepStrategies(dependencies, ['bounded_sample'])

    await expect(runBenchmarkScenario(config, dependencies)).rejects.toThrow(
      'viewport step 1: expected strategy envelope_cache, received bounded_sample',
    )
    expect(calls).toContain('viewport:envelope_cache:101:action')
  })

  it('策略证据为 undefined 时拒绝通过', async () => {
    const calls: string[] = []
    const dependencies = createDependencies(calls)
    setMeasuredStepStrategies(dependencies, undefined)

    await expect(runBenchmarkScenario(config, dependencies)).rejects.toThrow(
      'viewport step 1: missing strategy evidence for envelope_cache',
    )
    expect(calls).toContain('viewport:envelope_cache:101:action')
  })

  it('策略证据为空数组时拒绝通过', async () => {
    const calls: string[] = []
    const dependencies = createDependencies(calls)
    setMeasuredStepStrategies(dependencies, [])

    await expect(runBenchmarkScenario(config, dependencies)).rejects.toThrow(
      'viewport step 1: missing strategy evidence for envelope_cache',
    )
    expect(calls).toContain('viewport:envelope_cache:101:action')
  })

  it('定位属性缺少 bbox 时失败', async () => {
    const dependencies = createDependencies([])
    dependencies.getFeatureAttributes = async (datasetName, featureID) => ({
      datasetName,
      id: featureID,
      geometryType: 'MultiPolygon',
      properties: {},
    })

    await expect(runBenchmarkScenario(config, dependencies)).rejects.toThrow(/bbox|定位/)
  })

  it('required ID 不在查询返回或 source 时失败', async () => {
    const dependencies = createDependencies([])
    dependencies.runViewportStep = async () => ({
      backendQueryMs: [1],
      moveendToRenderMs: 2,
      finalFeatureCount: 1,
      blankRender: false,
      strategies: ['envelope_cache'],
      featureIDs: [],
    })

    await expect(runBenchmarkScenario(config, dependencies)).rejects.toThrow(/required|source|选中/)
  })

  it('设置选择后未形成高亮状态时失败', async () => {
    const dependencies = createDependencies([])
    dependencies.setSelection = async () => false

    await expect(runBenchmarkScenario(config, dependencies)).rejects.toThrow(/高亮/)
  })

  it('选择行越界时停止要素查询', async () => {
    const calls: string[] = []
    const dependencies = createDependencies(calls)
    dependencies.loadDatasetPage = async () => ({
      columns: ['SmID'],
      rows: [],
      currentPage: 2,
      totalPages: 2,
    })
    dependencies.getFeatureAttributes = vi.fn(dependencies.getFeatureAttributes)

    await expect(runBenchmarkScenario(config, dependencies)).rejects.toThrow('rowIndex')
    expect(dependencies.getFeatureAttributes).not.toHaveBeenCalled()
  })

  it('拒绝未返回的图层数据集', async () => {
    const calls: string[] = []
    const dependencies = createDependencies(calls)
    dependencies.listDatasets = async () => []

    await expect(runBenchmarkScenario(config, dependencies)).rejects.toThrow('县级行政区划')
    expect(calls).not.toContain('summary:县级行政区划')
  })
})
