import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mapLayerFixtures, sampledMapLayerFixture, selectedFeatureFixture } from '../test/fixtures'
import type { SpatialRendererAdapter } from '../spatial/SpatialRendererAdapter'
import type { MapLayerState } from '../types'
import { MapWorkspace } from './MapWorkspace'

const adapterInstances = vi.hoisted(() => [] as Array<Record<string, ReturnType<typeof vi.fn>>>)

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
