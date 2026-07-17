import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { BenchmarkRunner } from './BenchmarkRunner'
import type { BenchmarkConfig, BenchmarkResult } from './types'
import { LoadSpatialPreview } from '../../wailsjs/go/main/App'
import { main } from '../../wailsjs/go/models'

vi.mock('../../wailsjs/go/main/App', () => ({
  GetDatasetSpatialSummary: vi.fn(),
  GetFeatureAttributes: vi.fn(),
  ListDatasets: vi.fn(),
  LoadDatasetPage: vi.fn(),
  OpenUDBXFile: vi.fn(),
  QuitBenchmark: vi.fn(),
  SaveBenchmarkResult: vi.fn(),
  LoadSpatialPreview: vi.fn(async (datasetName: string, request: { viewport?: unknown }) => ({
    datasetName,
    kind: 'point',
    features: [{ id: 7, geometry: { type: 'Point', coordinates: [1, 2], hasZ: false } }],
    estimatedVertexCount: 1,
    sampled: false,
    queriedBounds: request.viewport,
    strategy: 'rtree',
    hasMore: false,
    queryDurationMs: 9,
    fileGeneration: 1,
  })),
}))

const config: BenchmarkConfig = {
  runId: 'sampledata-01',
  outputPath: '/tmp/sampledata-01.json',
  temperature: 'cold',
  maxConcurrentQueries: 1,
  scenario: {
    name: 'sampledata-multilayer',
    filePath: '/data/SampleData.udbx',
    layers: ['BaseMap_P'],
    selection: { datasetName: 'BaseMap_P', page: 1, rowIndex: 0 },
    viewportSteps: [{
      bounds: { minX: 0, minY: 0, maxX: 10, maxY: 10 },
      expectedStrategy: 'rtree',
    }],
  },
}

const passedResult: BenchmarkResult = {
  runId: config.runId,
  status: 'passed',
  startedAt: '2026-07-14T16:00:00+08:00',
  scenario: config.scenario.name,
  metrics: {
    openFileMs: 10,
    loadLayersMs: 20,
    fitVisibleLayersMs: 1,
    selectAndFitMs: 5,
    backendQueryMs: [9],
    moveendToRenderMs: [20],
    maxConcurrentQueries: 1,
    pendingPeak: 1,
    pendingFinal: 0,
    staleResultsDiscarded: 0,
    staleResultApplied: false,
    finalFeatureCount: 1,
    blankRenderCount: 0,
  },
  error: '',
}

function createAdapter() {
  let viewportHandler: ((bounds: { minX: number; minY: number; maxX: number; maxY: number }) => void) | null = null
  return {
    mount: vi.fn(),
    destroy: vi.fn(),
    setLayer: vi.fn(),
    fitAllVisibleLayers: vi.fn(),
    setSelection: vi.fn(),
    fitFeature: vi.fn(),
    fitBounds: vi.fn((bounds) => viewportHandler?.(bounds)),
    hasFeature: vi.fn().mockReturnValue(true),
    isSelectionHighlighted: vi.fn().mockReturnValue(true),
    setLayerVisible: vi.fn(),
    removeLayer: vi.fn(),
    onViewportChange: vi.fn((handler) => {
      viewportHandler = handler
      return () => { viewportHandler = null }
    }),
    getViewport: vi.fn().mockReturnValue(null),
    waitForRenderComplete: vi.fn().mockResolvedValue(undefined),
    getVisibleFeatureCount: vi.fn().mockReturnValue(1),
  }
}

describe('BenchmarkRunner', () => {
  it('保存成功结果后退出基准应用', async () => {
    const adapter = createAdapter()
    const runScenario = vi.fn().mockResolvedValue(passedResult)
    const saveResult = vi.fn().mockResolvedValue(undefined)
    const quitBenchmark = vi.fn().mockResolvedValue(undefined)

    render(
      <BenchmarkRunner
        config={config}
        adapterFactory={() => adapter}
        runScenario={runScenario}
        saveResult={saveResult}
        quitBenchmark={quitBenchmark}
      />,
    )

    expect(screen.getByText('本机基准运行中')).toBeInTheDocument()
    expect(screen.getByText(config.scenario.name)).toBeInTheDocument()
    await waitFor(() => expect(saveResult).toHaveBeenCalledWith(passedResult))
    await waitFor(() => expect(quitBenchmark).toHaveBeenCalledTimes(1))
    expect(runScenario).toHaveBeenCalledWith(config, expect.any(Object))
    expect(adapter.mount).toHaveBeenCalledTimes(1)
  })

  it('执行失败时保存可诊断失败结果并退出', async () => {
    const runScenario = vi.fn().mockRejectedValue(new Error('boom'))
    const saveResult = vi.fn().mockResolvedValue(undefined)
    const quitBenchmark = vi.fn().mockResolvedValue(undefined)

    render(
      <BenchmarkRunner
        config={config}
        adapterFactory={createAdapter}
        runScenario={runScenario}
        saveResult={saveResult}
        quitBenchmark={quitBenchmark}
      />,
    )

    await waitFor(() => expect(saveResult).toHaveBeenCalledWith(expect.objectContaining({
      runId: config.runId,
      status: 'failed',
      scenario: config.scenario.name,
      metrics: {
        openFileMs: 0,
        loadLayersMs: 0,
        fitVisibleLayersMs: 0,
        selectAndFitMs: 0,
        backendQueryMs: [],
        moveendToRenderMs: [],
        maxConcurrentQueries: 0,
        pendingPeak: 0,
        pendingFinal: 0,
        staleResultsDiscarded: 0,
        staleResultApplied: false,
        finalFeatureCount: 0,
        blankRenderCount: 0,
      },
      error: 'boom',
    })))
    await waitFor(() => expect(quitBenchmark).toHaveBeenCalledTimes(1))
  })

  it('动画帧被节流时仍在保存结果后退出', async () => {
    const requestAnimationFrameSpy = vi
      .spyOn(window, 'requestAnimationFrame')
      .mockImplementation(() => 1)
    const saveResult = vi.fn().mockResolvedValue(undefined)
    const quitBenchmark = vi.fn().mockResolvedValue(undefined)

    try {
      render(
        <BenchmarkRunner
          config={config}
          adapterFactory={createAdapter}
          runScenario={vi.fn().mockResolvedValue(passedResult)}
          saveResult={saveResult}
          quitBenchmark={quitBenchmark}
        />,
      )

      await waitFor(() => expect(saveResult).toHaveBeenCalledWith(passedResult))
      await waitFor(() => expect(quitBenchmark).toHaveBeenCalledTimes(1), { timeout: 600 })
    } finally {
      requestAnimationFrameSpy.mockRestore()
    }
  })

  it('视口步骤等待协调器结束和 rendercomplete 后返回真实指标', async () => {
    const adapter = createAdapter()
    let viewportResult: Awaited<ReturnType<Parameters<NonNullable<Parameters<typeof BenchmarkRunner>[0]['runScenario']>>[1]['runViewportStep']>> | undefined
    const runScenario = vi.fn(async (_config, dependencies) => {
      await dependencies.openFile(config.scenario.filePath)
      dependencies.setLayer({
        datasetName: 'BaseMap_P', kind: 'point', visible: true, style: {} as never,
        loading: false, error: null,
        summary: { datasetName: 'BaseMap_P', kind: 'point', objectCount: 1, estimatedVertexCount: 1, previewSupported: true, viewportQuerySupported: true, rtreeAvailable: true },
        preview: null, queryStatus: 'idle', queryError: null,
      })
      viewportResult = await dependencies.runViewportStep(config.scenario.viewportSteps[0], [])
      return passedResult
    })
    const saveResult = vi.fn().mockResolvedValue(undefined)

    render(
      <BenchmarkRunner
        config={config}
        adapterFactory={() => adapter}
        runScenario={runScenario}
        saveResult={saveResult}
        quitBenchmark={vi.fn().mockResolvedValue(undefined)}
      />,
    )

    await waitFor(() => expect(saveResult).toHaveBeenCalled())
    expect(viewportResult).toMatchObject({
      backendQueryMs: [9],
      finalFeatureCount: 1,
      blankRender: false,
      strategies: ['rtree'],
      featureIDs: [7],
    })
    expect(adapter.hasFeature).toHaveBeenCalledWith('BaseMap_P', 7)
    expect(adapter.fitBounds).toHaveBeenCalledWith(config.scenario.viewportSteps[0].bounds, 'point')
    expect(adapter.waitForRenderComplete).toHaveBeenCalledTimes(1)
  })

  it('并发为 1 时等待所有可查询空间层完成本轮请求范围', async () => {
    const adapter = createAdapter()
    const first = createDeferred<main.SpatialPreviewDTO>()
    const second = createDeferred<main.SpatialPreviewDTO>()
    vi.mocked(LoadSpatialPreview).mockClear()
    vi.mocked(LoadSpatialPreview).mockImplementation((datasetName) => {
      if (datasetName === 'BaseMap_P') {
        return first.promise
      }
      if (datasetName === 'BaseMap_L') {
        return second.promise
      }
      throw new Error(`unexpected dataset ${datasetName}`)
    })
    let viewportResolved = false
    const oldBounds = { minX: -1, minY: -1, maxX: 1, maxY: 1 }
    const runScenario = vi.fn(async (_config, dependencies) => {
      await dependencies.openFile(config.scenario.filePath)
      for (const [datasetName, kind] of [['BaseMap_P', 'point'], ['BaseMap_L', 'line']] as const) {
        dependencies.setLayer({
          datasetName, kind, visible: true, style: {} as never,
          loading: false, error: null,
          summary: { datasetName, kind, objectCount: 1, estimatedVertexCount: 1, previewSupported: true, viewportQuerySupported: true, rtreeAvailable: true },
          preview: {
            datasetName, kind, features: [], estimatedVertexCount: 0, sampled: false,
            queriedBounds: oldBounds, strategy: 'rtree', hasMore: false, queryDurationMs: 1, fileGeneration: 1,
          },
          queryStatus: 'ready', queryError: null, lastQueriedBounds: oldBounds,
        })
      }
      await dependencies.runViewportStep(config.scenario.viewportSteps[0], [])
      viewportResolved = true
      return passedResult
    })
    const saveResult = vi.fn().mockResolvedValue(undefined)

    render(
      <BenchmarkRunner
        config={config}
        adapterFactory={() => adapter}
        runScenario={runScenario}
        saveResult={saveResult}
        quitBenchmark={vi.fn().mockResolvedValue(undefined)}
      />,
    )

    await waitFor(() => expect(LoadSpatialPreview).toHaveBeenCalledTimes(1))
    const firstRequest = vi.mocked(LoadSpatialPreview).mock.calls[0][1]
    first.resolve(new main.SpatialPreviewDTO({
      datasetName: 'BaseMap_P', kind: 'point', features: [], estimatedVertexCount: 0, sampled: false,
      queriedBounds: firstRequest.viewport, strategy: 'rtree', hasMore: false, queryDurationMs: 5, fileGeneration: 1,
    }))
    await waitFor(() => expect(LoadSpatialPreview).toHaveBeenCalledTimes(2))
    expect(viewportResolved).toBe(false)
    expect(saveResult).not.toHaveBeenCalled()

    const secondRequest = vi.mocked(LoadSpatialPreview).mock.calls[1][1]
    second.resolve(new main.SpatialPreviewDTO({
      datasetName: 'BaseMap_L', kind: 'line', features: [], estimatedVertexCount: 0, sampled: false,
      queriedBounds: secondRequest.viewport, strategy: 'rtree', hasMore: false, queryDurationMs: 6, fileGeneration: 1,
    }))

    await waitFor(() => expect(saveResult).toHaveBeenCalledWith(passedResult))
    expect(viewportResolved).toBe(true)
  })

  it('视口步骤超时报告 moveend 和图层查询阶段', async () => {
    const adapter = createAdapter()
    vi.mocked(LoadSpatialPreview).mockImplementationOnce(() => new Promise<main.SpatialPreviewDTO>(() => undefined))
    const runScenario = vi.fn(async (_config, dependencies) => {
      await dependencies.openFile(config.scenario.filePath)
      dependencies.setLayer({
        datasetName: 'BaseMap_P', kind: 'point', visible: true, style: {} as never,
        loading: false, error: null,
        summary: { datasetName: 'BaseMap_P', kind: 'point', objectCount: 1, estimatedVertexCount: 1, previewSupported: true, viewportQuerySupported: true, rtreeAvailable: true },
        preview: null, queryStatus: 'idle', queryError: null,
      })
      await dependencies.runViewportStep(config.scenario.viewportSteps[0], [])
      return passedResult
    })
    const saveResult = vi.fn().mockResolvedValue(undefined)

    render(
      <BenchmarkRunner
        config={config}
        adapterFactory={() => adapter}
        runScenario={runScenario}
        saveResult={saveResult}
        quitBenchmark={vi.fn().mockResolvedValue(undefined)}
        viewportStepTimeoutMs={400}
      />,
    )

    await waitFor(() => expect(saveResult).toHaveBeenCalledWith(expect.objectContaining({
      status: 'failed',
      error: expect.stringMatching(/viewportObserved=true.*BaseMap_P=loading/),
    })), { timeout: 1000 })
  })

  it('视口查询错误立即结束步骤而不是等待超时', async () => {
    const adapter = createAdapter()
    vi.mocked(LoadSpatialPreview).mockClear()
    vi.mocked(LoadSpatialPreview).mockRejectedValueOnce(new Error('query failed'))
    const runScenario = vi.fn(async (_config, dependencies) => {
      await dependencies.openFile(config.scenario.filePath)
      dependencies.setLayer({
        datasetName: 'BaseMap_P', kind: 'point', visible: true, style: {} as never,
        loading: false, error: null,
        summary: { datasetName: 'BaseMap_P', kind: 'point', objectCount: 1, estimatedVertexCount: 1, previewSupported: true, viewportQuerySupported: true, rtreeAvailable: true },
        preview: null, queryStatus: 'idle', queryError: null,
      })
      await dependencies.runViewportStep(config.scenario.viewportSteps[0], [])
      return passedResult
    })
    const saveResult = vi.fn().mockResolvedValue(undefined)

    render(
      <BenchmarkRunner
        config={config}
        adapterFactory={() => adapter}
        runScenario={runScenario}
        saveResult={saveResult}
        quitBenchmark={vi.fn().mockResolvedValue(undefined)}
        viewportStepTimeoutMs={400}
      />,
    )

    await waitFor(() => expect(saveResult).toHaveBeenCalledWith(expect.objectContaining({
      status: 'failed',
      error: 'query failed',
    })), { timeout: 1000 })
  })

  it('fit 未触发 moveend 时使用 adapter 当前视口启动查询', async () => {
    const adapter = createAdapter()
    adapter.fitBounds.mockImplementation(() => undefined)
    adapter.getViewport.mockReturnValue(config.scenario.viewportSteps[0].bounds)
    let viewportResult: unknown
    const runScenario = vi.fn(async (_config, dependencies) => {
      await dependencies.openFile(config.scenario.filePath)
      dependencies.setLayer({
        datasetName: 'BaseMap_P', kind: 'point', visible: true, style: {} as never,
        loading: false, error: null,
        summary: { datasetName: 'BaseMap_P', kind: 'point', objectCount: 1, estimatedVertexCount: 1, previewSupported: true, viewportQuerySupported: true, rtreeAvailable: true },
        preview: null, queryStatus: 'idle', queryError: null,
      })
      viewportResult = await dependencies.runViewportStep(config.scenario.viewportSteps[0], [])
      return passedResult
    })
    const saveResult = vi.fn().mockResolvedValue(undefined)

    render(<BenchmarkRunner config={config} adapterFactory={() => adapter} runScenario={runScenario}
      saveResult={saveResult} quitBenchmark={vi.fn().mockResolvedValue(undefined)} />)

    await waitFor(() => expect(saveResult).toHaveBeenCalled())
    expect(viewportResult).toMatchObject({ strategies: ['rtree'], finalFeatureCount: 1 })
    expect(adapter.onViewportChange).not.toHaveBeenCalled()
  })
})

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}
