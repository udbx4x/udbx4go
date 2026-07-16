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
      return { path, datasetCount: 1 }
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
        properties: { Name: '示例县' },
      }
    },
    setLayer: (layer) => calls.push(`setLayer:${layer.datasetName}`),
    fitAllVisibleLayers: () => calls.push('fitAll'),
    setSelection: (selection) => calls.push(`setSelection:${selection.datasetName}:${selection.featureID}`),
    fitFeature: (datasetName, featureID) => calls.push(`fitFeature:${datasetName}:${featureID}`),
    runViewportStep: async (step, requiredIDs) => {
      calls.push(`viewport:${step.expectedStrategy}:${requiredIDs.join(',')}:${step.hideLayers ? 'action' : 'plain'}`)
      return {
        backendQueryMs: [8],
        moveendToRenderMs: 24,
        finalFeatureCount: 164,
        blankRender: false,
      }
    },
    getCoordinatorMetrics: () => ({
      maxConcurrentQueries: 2,
      pendingPeak: 1,
      activeQueries: 0,
      pendingQueries: 0,
      staleResultsDiscarded: 1,
      staleResultApplied: false,
    }),
  }
}

describe('runBenchmarkScenario', () => {
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
      'setSelection:县级行政区划:101',
      'fitFeature:县级行政区划:101',
      'viewport:envelope_cache:101:plain',
      'viewport:envelope_cache:101:plain',
      'viewport:envelope_cache:101:action',
      'viewport:envelope_cache:101:plain',
    ])
    expect(result.status).toBe('passed')
    expect(result.metrics.backendQueryMs).toEqual([8, 8])
    expect(result.metrics.moveendToRenderMs).toEqual([24, 24])
    expect(result.metrics).toMatchObject({
      maxConcurrentQueries: 2,
      pendingPeak: 1,
      pendingFinal: 0,
      staleResultsDiscarded: 1,
      staleResultApplied: false,
      finalFeatureCount: 164,
      blankRenderCount: 0,
    })
  })

  it('策略不匹配时拒绝通过', async () => {
    const dependencies = createDependencies([])
    dependencies.runViewportStep = async () => ({
      backendQueryMs: [1],
      moveendToRenderMs: 2,
      finalFeatureCount: 1,
      blankRender: false,
      strategies: ['bounded_sample'],
    })

    await expect(runBenchmarkScenario(config, dependencies)).rejects.toThrow('expected strategy')
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
