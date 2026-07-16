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

  it('隐藏期间更新视口后，重新显示图层会按最后视口查询', async () => {
    const hiddenViewport = { minX: 200, minY: 100, maxX: 300, maxY: 180 }
    const { result } = renderViewerHook()
    await act(async () => {
      await result.current.addDatasetToMap('BaseMap_P')
    })

    act(() => result.current.setMapLayerVisible('BaseMap_P', false))
    act(() => result.current.queryViewport(hiddenViewport))
    expect(mocks.LoadSpatialPreview).not.toHaveBeenCalled()
    act(() => result.current.setMapLayerVisible('BaseMap_P', true))
    await act(async () => vi.advanceTimersByTimeAsync(250))
    await act(flushPromises)

    expect(mocks.LoadSpatialPreview).toHaveBeenCalledOnce()
    expect(mocks.LoadSpatialPreview).toHaveBeenCalledWith(
      'BaseMap_P',
      expect.objectContaining({
        viewport: { minX: 185, minY: 88, maxX: 315, maxY: 192 },
      }),
    )
  })

  it('隐藏执行中图层后退出 loading 且迟到结果不回写', async () => {
    const deferred = createDeferred<SpatialPreview>()
    mocks.LoadSpatialPreview.mockImplementationOnce(() => deferred.promise)
    const { result } = renderViewerHook()
    await act(async () => {
      await result.current.addDatasetToMap('BaseMap_P')
    })
    act(() => result.current.queryViewport(viewport))
    await act(async () => vi.advanceTimersByTimeAsync(250))

    expect(result.current.mapLayers[0].queryStatus).toBe('loading')
    act(() => result.current.setMapLayerVisible('BaseMap_P', false))
    expect(result.current.mapLayers[0].queryStatus).toBe('idle')

    deferred.resolve(preview({ queriedBounds: mocks.LoadSpatialPreview.mock.calls[0][1].viewport }))
    await act(flushPromises)
    expect(result.current.mapLayers[0]).toMatchObject({ preview: null, queryStatus: 'idle' })
  })

  it('已记录当前视口时，新空间矢量图层无需再次移动即可首查', async () => {
    const { result } = renderViewerHook()
    act(() => result.current.queryViewport(viewport))

    await act(async () => {
      await result.current.addDatasetToMap('BaseMap_P')
    })
    await act(async () => vi.advanceTimersByTimeAsync(250))
    await act(flushPromises)

    expect(mocks.LoadSpatialPreview).toHaveBeenCalledOnce()
    expect(mocks.LoadSpatialPreview).toHaveBeenCalledWith(
      'BaseMap_P',
      expect.objectContaining({
        viewport: { minX: -15, minY: -7.5, maxX: 115, maxY: 57.5 },
      }),
    )
  })

  it('切换文件只失效旧查询，保留地图最后视口供新文件图层首查', async () => {
    mocks.OpenFileDialog.mockResolvedValue('/tmp/next.udbx')
    mocks.OpenUDBXFile.mockResolvedValue({ path: '/tmp/next.udbx', datasetCount: 1 })
    const { result } = renderViewerHook()
    act(() => result.current.queryViewport(viewport))
    await act(async () => {
      await result.current.openFileDialog()
      await result.current.addDatasetToMap('BaseMap_P')
    })
    await act(async () => vi.advanceTimersByTimeAsync(250))
    await act(flushPromises)

    expect(mocks.LoadSpatialPreview).toHaveBeenCalledOnce()
    expect(mocks.LoadSpatialPreview).toHaveBeenCalledWith(
      'BaseMap_P',
      expect.objectContaining({
        viewport: { minX: -15, minY: -7.5, maxX: 115, maxY: 57.5 },
      }),
    )
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
      degradedReason: 'unsupported_dataset_kind',
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
      queryStatus: 'ready',
      preview: { strategy: 'bounded_sample' },
    })
  })

  it('空间矢量仅在包络缓存预算超限降级为 bounded sample', async () => {
    const { result } = renderViewerHook()
    await act(async () => {
      await result.current.addDatasetToMap('BaseMap_P')
    })
    mocks.LoadSpatialPreview.mockResolvedValue(preview({
      queriedBounds: { minX: -15, minY: -7.5, maxX: 115, maxY: 57.5 },
      strategy: 'bounded_sample',
      degradedReason: 'envelope_cache_budget_exceeded',
    }))

    act(() => result.current.queryViewport(viewport))
    await act(async () => vi.advanceTimersByTimeAsync(250))
    await act(flushPromises)

    expect(result.current.mapLayers[0].queryStatus).toBe('degraded')
  })

  it.each([
    ['视口外', { minX: 200, minY: 200, maxX: 200, maxY: 200 }, 2],
    ['视口内', { minX: 20, minY: 20, maxX: 20, maxY: 20 }, 3],
  ] as const)('记录%s required feature 后的普通视口对象数', async (_name, requiredBBox, expectedCount) => {
    mocks.GetFeatureAttributes.mockResolvedValue(featureAttributes(7))
    const { result } = renderViewerHook()
    await act(async () => {
      await result.current.addDatasetToMap('BaseMap_P')
      await result.current.selectFeature('BaseMap_P', 7)
    })
    mocks.LoadSpatialPreview.mockImplementationOnce((_datasetName, request) => Promise.resolve(preview({
      queriedBounds: request.viewport,
      hasMore: true,
      features: [
        previewFeature(1, { minX: 10, minY: 10, maxX: 10, maxY: 10 }),
        previewFeature(2, { minX: 30, minY: 30, maxX: 30, maxY: 30 }),
        previewFeature(7, requiredBBox),
      ],
    })))

    act(() => result.current.queryViewport({ minX: 0, minY: 0, maxX: 100, maxY: 100 }))
    await act(async () => vi.advanceTimersByTimeAsync(250))
    await act(flushPromises)

    expect(result.current.mapLayers[0].preview?.viewportFeatureCount).toBe(expectedCount)
  })

  it('普通视口对象数只计有效有限有序且与查询范围闭区间相交的 BBox', async () => {
    const { result } = renderViewerHook()
    await act(async () => {
      await result.current.addDatasetToMap('BaseMap_P')
    })
    mocks.LoadSpatialPreview.mockImplementationOnce((_datasetName, request) => Promise.resolve(preview({
      queriedBounds: request.viewport,
      features: [
        previewFeature(1, { minX: 10, minY: 10, maxX: 10, maxY: 10 }),
        previewFeature(2, { minX: request.viewport.maxX, minY: 0, maxX: request.viewport.maxX, maxY: 0 }),
        previewFeature(3, { minX: Number.NaN, minY: 0, maxX: 1, maxY: 1 }),
        previewFeature(4, { minX: 20, minY: 0, maxX: 10, maxY: 1 }),
        { id: 5, geometry: { type: 'Point', coordinates: [1, 1], hasZ: false } },
        previewFeature(6, { minX: 1000, minY: 1000, maxX: 1001, maxY: 1001 }),
      ],
    })))

    act(() => result.current.queryViewport(viewport))
    await act(async () => vi.advanceTimersByTimeAsync(250))
    await act(flushPromises)

    expect(result.current.mapLayers[0].preview?.viewportFeatureCount).toBe(2)
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

  it('当前单选生成唯一 requiredIds，并在响应后保留选择供地图高亮', async () => {
    mocks.GetFeatureAttributes.mockResolvedValue({
      datasetName: 'BaseMap_P',
      id: 7,
      geometryType: 'Point',
      bbox: { minX: 7, minY: 8, maxX: 7, maxY: 8 },
      properties: {},
    })
    const { result } = renderViewerHook()
    await act(async () => {
      await result.current.addDatasetToMap('BaseMap_P')
      await result.current.selectFeature('BaseMap_P', 7)
    })

    act(() => result.current.queryViewport(viewport))
    await act(async () => vi.advanceTimersByTimeAsync(250))
    await act(flushPromises)

    expect(mocks.LoadSpatialPreview).toHaveBeenCalledWith(
      'BaseMap_P',
      expect.objectContaining({ requiredIds: [7] }),
    )
    expect(result.current.selectedMapFeature).toEqual({ datasetName: 'BaseMap_P', featureID: 7 })
    expect(result.current.mapLayers[0].preview).not.toBeNull()
  })

  it('迟到的其他对象属性不会替换当前选择或触发定位', async () => {
    mocks.GetFeatureAttributes.mockResolvedValue({
      datasetName: 'BaseMap_P',
      id: 6,
      geometryType: 'Point',
      bbox: { minX: 6, minY: 6, maxX: 6, maxY: 6 },
      properties: {},
    })
    const { result } = renderViewerHook()

    await act(async () => {
      await result.current.selectFeature('BaseMap_P', 7)
    })

    expect(result.current.selectedMapFeature).toEqual({ datasetName: 'BaseMap_P', featureID: 7 })
    expect(result.current.selectedFeatureAttributes).toBeNull()
    expect(result.current.selectionLocationError).toBe('定位失败')
  })

  it('前一次选择的合法属性迟到时不能覆盖当前选择', async () => {
    const first = createDeferred<ReturnType<typeof featureAttributes>>()
    const second = createDeferred<ReturnType<typeof featureAttributes>>()
    mocks.GetFeatureAttributes
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise)
    const { result } = renderViewerHook()

    let firstSelection!: Promise<void>
    let secondSelection!: Promise<void>
    act(() => {
      firstSelection = result.current.selectFeature('BaseMap_P', 7)
      secondSelection = result.current.selectFeature('BaseMap_P', 8)
    })
    second.resolve(featureAttributes(8))
    await act(async () => secondSelection)
    first.resolve(featureAttributes(7))
    await act(async () => firstSelection)

    expect(result.current.selectedMapFeature).toEqual({ datasetName: 'BaseMap_P', featureID: 8 })
    expect(result.current.selectedFeatureAttributes).toEqual(featureAttributes(8))
  })

  it('快速重复选择同一对象时立即清空旧属性并忽略旧请求响应', async () => {
    const first = createDeferred<ReturnType<typeof featureAttributes>>()
    const second = createDeferred<ReturnType<typeof featureAttributes>>()
    mocks.GetFeatureAttributes
      .mockResolvedValueOnce(featureAttributes(6))
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise)
    const { result } = renderViewerHook()
    await act(async () => {
      await result.current.selectFeature('BaseMap_P', 6)
    })
    expect(result.current.selectedFeatureAttributes).toEqual(featureAttributes(6))

    let firstSelection!: Promise<void>
    let secondSelection!: Promise<void>
    act(() => {
      firstSelection = result.current.selectFeature('BaseMap_P', 7)
    })
    expect(result.current.selectedFeatureAttributes).toBeNull()
    act(() => {
      secondSelection = result.current.selectFeature('BaseMap_P', 7)
    })

    first.resolve(featureAttributes(7))
    await act(async () => firstSelection)
    expect(result.current.selectedMapFeature).toEqual({ datasetName: 'BaseMap_P', featureID: 7 })
    expect(result.current.selectedFeatureAttributes).toBeNull()

    second.resolve(featureAttributes(7))
    await act(async () => secondSelection)
    expect(result.current.selectedFeatureAttributes).toEqual(featureAttributes(7))
  })

  it('当前属性响应 ID 不匹配时保留选择并保持属性为空', async () => {
    mocks.GetFeatureAttributes
      .mockResolvedValueOnce(featureAttributes(6))
      .mockResolvedValueOnce(featureAttributes(8))
    const { result } = renderViewerHook()
    await act(async () => {
      await result.current.selectFeature('BaseMap_P', 6)
    })
    await act(async () => {
      await result.current.selectFeature('BaseMap_P', 7)
    })

    expect(result.current.selectedMapFeature).toEqual({ datasetName: 'BaseMap_P', featureID: 7 })
    expect(result.current.selectedFeatureAttributes).toBeNull()
    expect(result.current.selectionLocationError).toBe('定位失败')
  })

  it.each([
    ['缺失对象', undefined, new Error('not found')],
    ['损坏几何', undefined, new Error('corrupt geometry')],
    ['缺少 BBox', { datasetName: 'BaseMap_P', id: 7, geometryType: 'Point', properties: {} }, undefined],
    ['无效 BBox', {
      datasetName: 'BaseMap_P',
      id: 7,
      geometryType: 'Point',
      bbox: { minX: 10, minY: 0, maxX: 0, maxY: 10 },
      properties: {},
    }, undefined],
    ['无效 geometryType', {
      datasetName: 'BaseMap_P',
      id: 7,
      geometryType: 'BrokenGeometry',
      bbox: { minX: 0, minY: 0, maxX: 10, maxY: 10 },
      properties: {},
    }, undefined],
  ] as const)('%s 时保留表格选择并报告定位失败', async (_name, attributes, failure) => {
    if (failure) {
      mocks.GetFeatureAttributes.mockRejectedValue(failure)
    } else {
      mocks.GetFeatureAttributes.mockResolvedValue(attributes)
    }
    const { result } = renderViewerHook()

    await act(async () => {
      await result.current.selectFeature('BaseMap_P', 7)
    })

    expect(result.current.selectedMapFeature).toEqual({ datasetName: 'BaseMap_P', featureID: 7 })
    expect(result.current.selectionLocationError).toBe('定位失败')
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

  it('跨图层选择时 A 的迟到表页不能覆盖已完成的 B', async () => {
    const firstPage = createDeferred<{ columns: string[]; rows: string[][]; currentPage: number; totalPages: number }>()
    const secondPage = {
      columns: ['SmID', 'name'],
      rows: [['2', 'B']],
      currentPage: 1,
      totalPages: 1,
    }
    mocks.GetFeatureAttributes.mockImplementation((datasetName: string, featureID: number) => Promise.resolve({
      datasetName,
      id: featureID,
      geometryType: 'Point',
      bbox: { minX: featureID, minY: featureID, maxX: featureID, maxY: featureID },
      properties: {},
    }))
    mocks.LoadDatasetPage
      .mockImplementationOnce(() => firstPage.promise)
      .mockResolvedValueOnce(secondPage)
    const { result } = renderViewerHook()

    let firstSelection!: Promise<void>
    act(() => {
      firstSelection = result.current.selectFeature('LayerA', 1)
    })
    await act(flushPromises)
    expect(mocks.LoadDatasetPage).toHaveBeenCalledWith('LayerA', 1)

    await act(async () => {
      await result.current.selectFeature('LayerB', 2)
    })
    expect(result.current.activeTableDataset).toBe('LayerB')
    expect(result.current.pageData).toEqual(secondPage)

    firstPage.resolve({
      columns: ['SmID', 'name'],
      rows: [['1', 'A']],
      currentPage: 1,
      totalPages: 1,
    })
    await act(async () => firstSelection)

    expect(result.current.selectedMapFeature).toEqual({ datasetName: 'LayerB', featureID: 2 })
    expect(result.current.selectedFeatureAttributes?.datasetName).toBe('LayerB')
    expect(result.current.selectedDataset).toBe('LayerB')
    expect(result.current.activeTableDataset).toBe('LayerB')
    expect(result.current.pageData).toEqual(secondPage)
    expect(result.current.loading).toBe(false)
    expect(result.current.error).toBeNull()
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

function featureAttributes(id: number) {
  return {
    datasetName: 'BaseMap_P',
    id,
    geometryType: 'Point',
    bbox: { minX: id, minY: id, maxX: id, maxY: id },
    properties: {},
  }
}

function previewFeature(id: number, bbox: BoundingBox) {
  return {
    id,
    bbox,
    geometry: { type: 'Point', coordinates: [bbox.minX, bbox.minY], hasZ: false },
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
