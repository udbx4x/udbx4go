import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mapLayerFixtures, sampledMapLayerFixture, selectedFeatureFixture } from '../test/fixtures'
import type { SpatialRendererAdapter } from '../spatial/SpatialRendererAdapter'
import type { BoundingBox, MapLayerState } from '../types'
import { MapWorkspace } from './MapWorkspace'

const adapterInstances = vi.hoisted(() => [] as Array<Record<string, ReturnType<typeof vi.fn>>>)
const adapterState = vi.hoisted(() => ({ viewport: null as BoundingBox | null }))

vi.mock('../spatial/OpenLayersSpatialRendererAdapter', () => ({
  OpenLayersSpatialRendererAdapter: vi.fn().mockImplementation(() => {
    const unsubscribeViewport = vi.fn()
    const instance = {
      mount: vi.fn(),
      destroy: vi.fn(),
      onFeatureClick: vi.fn(),
      setLayer: vi.fn(),
      removeLayer: vi.fn(),
      setLayerVisible: vi.fn(),
      fitAllVisibleLayers: vi.fn(),
      fitBounds: vi.fn(),
      setSelection: vi.fn(),
      fitFeature: vi.fn(),
      getViewport: vi.fn(() => adapterState.viewport),
      onViewportChange: vi.fn(() => unsubscribeViewport),
      unsubscribeViewport,
    } satisfies SpatialRendererAdapter & {
      fitFeature(datasetName: string, featureID: number): void
      unsubscribeViewport(): void
    }
    adapterInstances.push(instance)
    return instance
  }),
}))

function latestAdapter() {
  const adapter = adapterInstances.at(-1)
  if (!adapter) {
    throw new Error('Expected MapWorkspace to create an adapter')
  }
  return adapter
}

describe('MapWorkspace settings behavior', () => {
  beforeEach(() => {
    adapterInstances.length = 0
    adapterState.viewport = null
  })

  it('无图层时提示从左侧选择空间数据集加入地图', () => {
    render(
      <MapWorkspace
        layers={[]}
        selectedFeature={null}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(screen.getByText('从左侧选择空间数据集加入地图')).toBeInTheDocument()
  })

  it('存在采样图层时显示采样预览提示', () => {
    render(
      <MapWorkspace
        layers={[sampledMapLayerFixture]}
        selectedFeature={null}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(screen.getByText('部分图层为采样预览')).toBeInTheDocument()
  })

  it('autoFitOnLayerChange=false 时图层变化后不自动适配全部可见图层', () => {
    render(
      <MapWorkspace
        layers={mapLayerFixtures}
        selectedFeature={null}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(latestAdapter().setLayer).toHaveBeenCalledWith(mapLayerFixtures[0])
    expect(latestAdapter().fitAllVisibleLayers).not.toHaveBeenCalled()
  })

  it('zoomToSelectedFeature=false 时只设置选中态，不缩放到要素', () => {
    render(
      <MapWorkspace
        layers={mapLayerFixtures}
        selectedFeature={selectedFeatureFixture}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(latestAdapter().setSelection).toHaveBeenCalledWith(selectedFeatureFixture)
    expect(latestAdapter().fitFeature).not.toHaveBeenCalled()
  })

  it('zoomToSelectedFeature=false 时仅改变选中要素不会触发自动适配图层', () => {
    const { rerender } = render(
      <MapWorkspace
        layers={mapLayerFixtures}
        selectedFeature={null}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )
    const adapter = latestAdapter()

    adapter.fitAllVisibleLayers.mockClear()
    adapter.fitFeature.mockClear()
    rerender(
      <MapWorkspace
        layers={mapLayerFixtures}
        selectedFeature={selectedFeatureFixture}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(adapter.setSelection).toHaveBeenCalledWith(selectedFeatureFixture)
    expect(adapter.fitAllVisibleLayers).not.toHaveBeenCalled()
    expect(adapter.fitFeature).not.toHaveBeenCalled()
  })

  it('zoomToSelectedFeature=true 时选中要素后缩放到该要素', () => {
    render(
      <MapWorkspace
        layers={mapLayerFixtures}
        selectedFeature={selectedFeatureFixture}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(latestAdapter().setSelection).toHaveBeenCalledWith(selectedFeatureFixture)
    expect(latestAdapter().fitFeature).toHaveBeenCalledWith(
      selectedFeatureFixture.datasetName,
      selectedFeatureFixture.featureID,
    )
  })

  it('选中要素先于目标图层 preview 写入时，在图层加载完成后补充定位', () => {
    const pendingLayer: MapLayerState = {
      ...mapLayerFixtures[0],
      loading: true,
      preview: null,
    }
    const { rerender } = render(
      <MapWorkspace
        layers={[pendingLayer]}
        selectedFeature={selectedFeatureFixture}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )
    const adapter = latestAdapter()

    adapter.fitFeature.mockClear()
    rerender(
      <MapWorkspace
        layers={mapLayerFixtures}
        selectedFeature={selectedFeatureFixture}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(adapter.setLayer).toHaveBeenCalledWith(mapLayerFixtures[0])
    expect(adapter.fitFeature).toHaveBeenCalledWith(
      selectedFeatureFixture.datasetName,
      selectedFeatureFixture.featureID,
    )
  })

  it('目标图层 preview 已就绪后，无关图层变化不会重复定位同一选中要素', () => {
    const pendingLayer: MapLayerState = {
      ...mapLayerFixtures[0],
      loading: true,
      preview: null,
    }
    const otherLayer: MapLayerState = {
      ...mapLayerFixtures[0],
      datasetName: 'BaseMap_L',
      kind: 'line',
      preview: {
        ...mapLayerFixtures[0].preview!,
        datasetName: 'BaseMap_L',
        kind: 'line',
      },
    }
    const { rerender } = render(
      <MapWorkspace
        layers={[pendingLayer]}
        selectedFeature={selectedFeatureFixture}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )
    const adapter = latestAdapter()

    adapter.fitFeature.mockClear()
    rerender(
      <MapWorkspace
        layers={mapLayerFixtures}
        selectedFeature={selectedFeatureFixture}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )
    expect(adapter.fitFeature).toHaveBeenCalledTimes(1)

    adapter.fitFeature.mockClear()
    rerender(
      <MapWorkspace
        layers={[mapLayerFixtures[0], otherLayer]}
        selectedFeature={selectedFeatureFixture}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(adapter.fitFeature).not.toHaveBeenCalled()
  })

  it('zoomToSelectedFeature=false 时目标图层 preview 后到也不定位', () => {
    const pendingLayer: MapLayerState = {
      ...mapLayerFixtures[0],
      loading: true,
      preview: null,
    }
    const { rerender } = render(
      <MapWorkspace
        layers={[pendingLayer]}
        selectedFeature={selectedFeatureFixture}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )
    const adapter = latestAdapter()

    adapter.fitFeature.mockClear()
    rerender(
      <MapWorkspace
        layers={mapLayerFixtures}
        selectedFeature={selectedFeatureFixture}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(adapter.fitFeature).not.toHaveBeenCalled()
  })

  it('订阅稳定视口并在卸载时解除订阅', () => {
    const onViewportChange = vi.fn()
    const { unmount } = render(
      <MapWorkspace
        layers={[]}
        selectedFeature={null}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={onViewportChange}
        onFeatureSelect={vi.fn()}
      />,
    )
    const adapter = latestAdapter()
    const handler = adapter.onViewportChange.mock.calls[0][0]

    handler({ minX: 1, minY: 2, maxX: 3, maxY: 4 })
    expect(onViewportChange).toHaveBeenCalledWith({ minX: 1, minY: 2, maxX: 3, maxY: 4 })

    unmount()
    expect(adapter.unsubscribeViewport).toHaveBeenCalledOnce()
  })

  it('mount 后立即上报 adapter 当前有限视口供 AutoFit=false 图层首查', () => {
    adapterState.viewport = { minX: -20, minY: -10, maxX: 20, maxY: 10 }
    const onViewportChange = vi.fn()

    render(
      <MapWorkspace
        layers={[]}
        selectedFeature={null}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={true}
        onViewportChange={onViewportChange}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(latestAdapter().getViewport).toHaveBeenCalledOnce()
    expect(onViewportChange).toHaveBeenCalledWith(adapterState.viewport)
  })

  it('AutoFit=true 时不在声明范围 fit 的 moveend 前上报默认视口', () => {
    adapterState.viewport = { minX: -20, minY: -10, maxX: 20, maxY: 10 }
    const onViewportChange = vi.fn()

    render(
      <MapWorkspace
        layers={[]}
        selectedFeature={null}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={onViewportChange}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(latestAdapter().getViewport).not.toHaveBeenCalled()
    expect(onViewportChange).not.toHaveBeenCalled()
  })

  it('mount 时无有效视口，AutoFit=false 新增空间层后再次读取当前范围且只上报一次', () => {
    const onViewportChange = vi.fn()
    const { rerender } = render(
      <MapWorkspace
        layers={[]}
        selectedFeature={null}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={true}
        onViewportChange={onViewportChange}
        onFeatureSelect={vi.fn()}
      />,
    )
    const adapter = latestAdapter()
    adapterState.viewport = { minX: -30, minY: -20, maxX: 30, maxY: 20 }
    const queryLayer = {
      ...mapLayerFixtures[0],
      preview: null,
      summary: {
        datasetName: 'BaseMap_P',
        kind: 'point',
        extent: { minX: 10, minY: 20, maxX: 40, maxY: 60 },
        objectCount: 10,
        estimatedVertexCount: 10,
        previewSupported: true,
        viewportQuerySupported: true,
        rtreeAvailable: true,
      },
    }

    rerender(
      <MapWorkspace
        layers={[queryLayer]}
        selectedFeature={null}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={true}
        onViewportChange={onViewportChange}
        onFeatureSelect={vi.fn()}
      />,
    )
    expect(adapter.getViewport).toHaveBeenCalledTimes(2)
    expect(onViewportChange).toHaveBeenCalledOnce()
    expect(onViewportChange).toHaveBeenCalledWith(adapterState.viewport)

    rerender(
      <MapWorkspace
        layers={[{ ...queryLayer, queryStatus: 'loading' }]}
        selectedFeature={null}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={true}
        onViewportChange={onViewportChange}
        onFeatureSelect={vi.fn()}
      />,
    )
    expect(adapter.getViewport).toHaveBeenCalledTimes(2)
    expect(onViewportChange).toHaveBeenCalledOnce()
  })

  it('多层仅 queryStatus 和 queryError 变化时不重建任何 Source', () => {
    const secondLayer = createSecondLayer()
    const { rerender } = render(
      <MapWorkspace
        layers={[mapLayerFixtures[0], secondLayer]}
        selectedFeature={null}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )
    const adapter = latestAdapter()
    adapter.setLayer.mockClear()

    rerender(
      <MapWorkspace
        layers={[
          { ...mapLayerFixtures[0], queryStatus: 'loading', queryError: null },
          { ...secondLayer, queryStatus: 'error', queryError: 'query failed' },
        ]}
        selectedFeature={null}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(adapter.setLayer).not.toHaveBeenCalled()
  })

  it('多层中只有 preview 引用变化的图层重建 Source', () => {
    const secondLayer = createSecondLayer()
    const { rerender } = render(
      <MapWorkspace
        layers={[mapLayerFixtures[0], secondLayer]}
        selectedFeature={null}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )
    const adapter = latestAdapter()
    adapter.setLayer.mockClear()
    const changedSecondLayer = {
      ...secondLayer,
      preview: { ...secondLayer.preview!, queryDurationMs: 12 },
    }

    rerender(
      <MapWorkspace
        layers={[{ ...mapLayerFixtures[0], queryStatus: 'loading' }, changedSecondLayer]}
        selectedFeature={null}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(adapter.setLayer).toHaveBeenCalledOnce()
    expect(adapter.setLayer).toHaveBeenCalledWith(changedSecondLayer)
  })

  it('多层中仅可见性变化时只更新该层显隐而不重建 Source', () => {
    const secondLayer = createSecondLayer()
    const { rerender } = render(
      <MapWorkspace
        layers={[mapLayerFixtures[0], secondLayer]}
        selectedFeature={null}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )
    const adapter = latestAdapter()
    adapter.setLayer.mockClear()
    adapter.setLayerVisible.mockClear()

    rerender(
      <MapWorkspace
        layers={[{ ...mapLayerFixtures[0], visible: false }, secondLayer]}
        selectedFeature={null}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(adapter.setLayer).not.toHaveBeenCalled()
    expect(adapter.setLayerVisible).toHaveBeenCalledOnce()
    expect(adapter.setLayerVisible).toHaveBeenCalledWith('BaseMap_P', false)
  })

  it('新增可查询图层用声明范围 fitBounds，等待 moveend 首查', () => {
    const pendingLayer: MapLayerState = {
      ...mapLayerFixtures[0],
      preview: null,
      summary: {
        datasetName: 'BaseMap_P',
        kind: 'point',
        extent: { minX: 10, minY: 20, maxX: 40, maxY: 60 },
        objectCount: 10,
        estimatedVertexCount: 10,
        previewSupported: true,
        viewportQuerySupported: true,
        rtreeAvailable: true,
      },
    }

    render(
      <MapWorkspace
        layers={[pendingLayer]}
        selectedFeature={null}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(latestAdapter().fitBounds).toHaveBeenCalledWith(
      { minX: 10, minY: 20, maxX: 40, maxY: 60 },
      'point',
    )
    expect(latestAdapter().fitAllVisibleLayers).not.toHaveBeenCalled()
  })

  it('可查询图层状态和结果更新后不重复自动 fit', () => {
    const summary = {
      datasetName: 'BaseMap_P',
      kind: 'point',
      extent: { minX: 10, minY: 20, maxX: 40, maxY: 60 },
      objectCount: 10,
      estimatedVertexCount: 10,
      previewSupported: true,
      viewportQuerySupported: true,
      rtreeAvailable: true,
    }
    const pendingLayer: MapLayerState = {
      ...mapLayerFixtures[0],
      preview: null,
      summary,
      queryStatus: 'idle',
    }
    const { rerender } = render(
      <MapWorkspace
        layers={[pendingLayer]}
        selectedFeature={null}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )
    const adapter = latestAdapter()
    adapter.fitBounds.mockClear()
    adapter.fitAllVisibleLayers.mockClear()

    rerender(
      <MapWorkspace
        layers={[{ ...mapLayerFixtures[0], summary, queryStatus: 'ready' }]}
        selectedFeature={null}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(adapter.fitBounds).not.toHaveBeenCalled()
    expect(adapter.fitAllVisibleLayers).not.toHaveBeenCalled()
  })

  it.each([
    ['Point', 'point'],
    ['MultiLineString', 'line'],
    ['MultiPolygon', 'polygon'],
  ] as const)('视口外 %s 选择使用真实 BBox 按 %s 定位', (geometryType, geometryKind) => {
    const bbox = { minX: 10, minY: 20, maxX: 40, maxY: 50 }

    render(
      <MapWorkspace
        layers={mapLayerFixtures}
        selectedFeature={selectedFeatureFixture}
        selectedFeatureAttributes={{
          datasetName: 'BaseMap_P',
          id: 1,
          geometryType,
          bbox,
          properties: {},
        }}
        selectionLocationError={null}
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(latestAdapter().fitBounds).toHaveBeenCalledWith(bbox, geometryKind)
    expect(latestAdapter().fitFeature).not.toHaveBeenCalled()
  })

  it.each([
    undefined,
    { minX: Number.NaN, minY: 0, maxX: 10, maxY: 10 },
    { minX: 10, minY: 0, maxX: 0, maxY: 10 },
  ])('无效或缺失 BBox 不定位到虚构点并显示定位失败', (bbox) => {
    render(
      <MapWorkspace
        layers={mapLayerFixtures}
        selectedFeature={selectedFeatureFixture}
        selectedFeatureAttributes={{
          datasetName: 'BaseMap_P',
          id: 1,
          geometryType: 'Point',
          bbox,
          properties: {},
        }}
        selectionLocationError="定位失败"
        autoFitOnLayerChange={true}
        zoomToSelectedFeature={true}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(latestAdapter().fitBounds).not.toHaveBeenCalled()
    expect(latestAdapter().fitFeature).not.toHaveBeenCalled()
    expect(screen.getByText('定位失败')).toBeInTheDocument()
  })

  it('视口响应替换 Source 后重新应用选择以高亮 required feature', () => {
    const { rerender } = render(
      <MapWorkspace
        layers={[{ ...mapLayerFixtures[0], preview: null }]}
        selectedFeature={selectedFeatureFixture}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )
    const adapter = latestAdapter()
    adapter.setSelection.mockClear()

    rerender(
      <MapWorkspace
        layers={mapLayerFixtures}
        selectedFeature={selectedFeatureFixture}
        autoFitOnLayerChange={false}
        zoomToSelectedFeature={false}
        onViewportChange={vi.fn()}
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(adapter.setLayer).toHaveBeenCalledWith(mapLayerFixtures[0])
    expect(adapter.setSelection).toHaveBeenCalledWith(selectedFeatureFixture)
  })
})

function createSecondLayer(): MapLayerState {
  return {
    ...mapLayerFixtures[0],
    datasetName: 'BaseMap_L',
    kind: 'line',
    preview: {
      ...mapLayerFixtures[0].preview!,
      datasetName: 'BaseMap_L',
      kind: 'line',
    },
  }
}
