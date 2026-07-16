import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { BoundingBox, SpatialPreview } from '../types'
import { useUDBX } from './useUDBX'

const mocks = vi.hoisted(() => ({
  GetDatasetSpatialSummary: vi.fn(),
  LoadSpatialPreview: vi.fn(),
  OpenFileDialog: vi.fn(),
  OpenUDBXFile: vi.fn(),
  CloseUDBXFile: vi.fn(),
  ListDatasets: vi.fn(),
  LoadDatasetPage: vi.fn(),
  GetCurrentFile: vi.fn(),
  GetFeatureAttributes: vi.fn(),
}))

vi.mock('../../wailsjs/go/main/App', () => mocks)

const viewport: BoundingBox = { minX: 0, minY: 0, maxX: 100, maxY: 50 }

describe('useUDBX viewport spatial previews', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    vi.useFakeTimers()
    mocks.LoadDatasetPage.mockResolvedValue({
      columns: ['SmID'],
      rows: [['1']],
      currentPage: 1,
      totalPages: 1,
    })
    mocks.GetDatasetSpatialSummary.mockResolvedValue(vectorSummary())
    mocks.LoadSpatialPreview.mockResolvedValue(preview({
      queriedBounds: { minX: -15, minY: -7.5, maxX: 115, maxY: 57.5 },
    }))
    mocks.ListDatasets.mockResolvedValue([])
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('Point/Line/Region/Z 加层时只加载 summary，等待声明范围 fit 后的 moveend', async () => {
    const { result } = renderViewerHook()

    await act(async () => {
      await result.current.addDatasetToMap('BaseMap_P')
    })

    expect(mocks.GetDatasetSpatialSummary).toHaveBeenCalledWith('BaseMap_P')
    expect(mocks.LoadSpatialPreview).not.toHaveBeenCalled()
    expect(result.current.mapLayers[0]).toMatchObject({
      summary: vectorSummary(),
      preview: null,
      queryStatus: 'idle',
      queryError: null,
    })
  })

  it.each(['text', 'cad'])('%s 加层时立即加载有界预览', async (kind) => {
    mocks.GetDatasetSpatialSummary.mockResolvedValue({
      ...vectorSummary(),
      datasetName: `${kind}Layer`,
      kind,
      viewportQuerySupported: false,
    })
    mocks.LoadSpatialPreview.mockResolvedValue(preview({
      datasetName: `${kind}Layer`,
      kind,
      queriedBounds: undefined,
      strategy: 'bounded_sample',
    }))
    const { result } = renderViewerHook()

    await act(async () => {
      await result.current.addDatasetToMap(`${kind}Layer`)
    })

    expect(mocks.LoadSpatialPreview).toHaveBeenCalledWith(
      `${kind}Layer`,
      expect.objectContaining({
        viewport: undefined,
        limit: 1234,
        maxVertices: 567890,
        simplify: false,
      }),
    )
    expect(result.current.mapLayers[0]).toMatchObject({
      queryStatus: 'degraded',
      preview: { strategy: 'bounded_sample' },
    })
  })

  it('查询时保留旧 preview，成功后原子替换并记录范围', async () => {
    const deferred = createDeferred<SpatialPreview>()
    const { result } = renderViewerHook()
    await act(async () => {
      await result.current.addDatasetToMap('BaseMap_P')
    })
    act(() => result.current.queryViewport(viewport))
    await act(async () => vi.advanceTimersByTimeAsync(250))
    await act(flushPromises)
    const oldPreview = result.current.mapLayers[0].preview
    mocks.LoadSpatialPreview.mockImplementationOnce(() => deferred.promise)

    act(() => result.current.queryViewport({ minX: 100, minY: 100, maxX: 200, maxY: 200 }))
    await act(async () => vi.advanceTimersByTimeAsync(250))

    expect(result.current.mapLayers[0].queryStatus).toBe('loading')
    expect(result.current.mapLayers[0].preview).toBe(oldPreview)
    const requestedBounds = mocks.LoadSpatialPreview.mock.calls[1][1].viewport
    deferred.resolve(preview({ queriedBounds: requestedBounds }))
    await act(flushPromises)

    expect(result.current.mapLayers[0]).toMatchObject({
      queryStatus: 'ready',
      queryError: null,
      lastQueriedBounds: requestedBounds,
    })
    expect(result.current.mapLayers[0].preview).not.toBe(oldPreview)
  })

  it('查询失败时保留旧 preview 和错误原因', async () => {
    const deferred = createDeferred<SpatialPreview>()
    const { result } = renderViewerHook()
    await act(async () => {
      await result.current.addDatasetToMap('BaseMap_P')
    })
    act(() => result.current.queryViewport(viewport))
    await act(async () => vi.advanceTimersByTimeAsync(250))
    await act(flushPromises)
    const oldPreview = result.current.mapLayers[0].preview
    mocks.LoadSpatialPreview.mockImplementationOnce(() => deferred.promise)

    act(() => result.current.queryViewport({ minX: 100, minY: 100, maxX: 200, maxY: 200 }))
    await act(async () => vi.advanceTimersByTimeAsync(250))
    deferred.reject(new Error('query failed'))
    await act(flushPromises)

    expect(result.current.mapLayers[0]).toMatchObject({
      queryStatus: 'error',
      queryError: 'query failed',
    })
    expect(result.current.mapLayers[0].preview).toBe(oldPreview)
  })

  it.each(['hide', 'remove', 'switch file'] as const)('%s 后迟到结果不能回写', async (action) => {
    const deferred = createDeferred<SpatialPreview>()
    mocks.LoadSpatialPreview.mockImplementation(() => deferred.promise)
    const { result } = renderViewerHook()
    await act(async () => {
      await result.current.addDatasetToMap('BaseMap_P')
    })
    act(() => result.current.queryViewport(viewport))
    await act(async () => vi.advanceTimersByTimeAsync(250))

    if (action === 'hide') {
      act(() => result.current.setMapLayerVisible('BaseMap_P', false))
    } else if (action === 'remove') {
      act(() => result.current.removeMapLayer('BaseMap_P'))
    } else {
      mocks.OpenFileDialog.mockResolvedValue('/tmp/next.udbx')
      mocks.OpenUDBXFile.mockResolvedValue({ path: '/tmp/next.udbx', datasetCount: 0 })
      await act(async () => {
        await result.current.openFileDialog()
      })
    }

    deferred.resolve(preview({
      queriedBounds: mocks.LoadSpatialPreview.mock.calls[0][1].viewport,
    }))
    await act(flushPromises)

    const layer = result.current.mapLayers.find((item) => item.datasetName === 'BaseMap_P')
    expect(layer?.preview ?? null).toBeNull()
  })

  it('属性表加载失败时设置错误状态并保留当前表格状态', async () => {
    mocks.LoadDatasetPage.mockRejectedValue(new Error("dataset kind 'unknown' is not supported"))
    const { result } = renderViewerHook()

    await act(async () => {
      await expect(result.current.loadTableDataset('modeldt')).rejects.toThrow('not supported')
    })

    expect(result.current.error).toBe("dataset kind 'unknown' is not supported")
    expect(result.current.activeTableDataset).toBeNull()
    expect(result.current.pageData).toBeNull()
    expect(result.current.loading).toBe(false)
  })
})

function renderViewerHook() {
  return renderHook(() => useUDBX({
    spatialPreviewFeatureLimit: 1234,
    spatialPreviewVertexBudget: 567890,
  }))
}

function vectorSummary() {
  return {
    datasetName: 'BaseMap_P',
    kind: 'point',
    extent: { minX: 0, minY: 0, maxX: 100, maxY: 50 },
    objectCount: 1,
    estimatedVertexCount: 1,
    previewSupported: true,
    viewportQuerySupported: true,
    rtreeAvailable: true,
  }
}

function preview(overrides: Partial<SpatialPreview> = {}): SpatialPreview {
  return {
    datasetName: 'BaseMap_P',
    kind: 'point',
    features: [],
    estimatedVertexCount: 0,
    sampled: false,
    strategy: 'rtree',
    hasMore: false,
    queryDurationMs: 1,
    fileGeneration: 0,
    ...overrides,
  }
}

function createDeferred<Value>() {
  let resolve!: (value: Value) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<Value>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

async function flushPromises(): Promise<void> {
  await Promise.resolve()
  await Promise.resolve()
}
