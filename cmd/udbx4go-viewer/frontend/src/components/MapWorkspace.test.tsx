import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mapLayerFixtures, sampledMapLayerFixture, selectedFeatureFixture } from '../test/fixtures'
import type { SpatialRendererAdapter } from '../spatial/SpatialRendererAdapter'
import type { MapLayerState } from '../types'
import { MapWorkspace } from './MapWorkspace'

const adapterInstances = vi.hoisted(() => [] as Array<Record<string, ReturnType<typeof vi.fn>>>)

vi.mock('../spatial/OpenLayersSpatialRendererAdapter', () => ({
  OpenLayersSpatialRendererAdapter: vi.fn().mockImplementation(() => {
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
      onViewportChange: vi.fn(() => vi.fn()),
    } satisfies SpatialRendererAdapter & {
      fitFeature(datasetName: string, featureID: number): void
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
        onFeatureSelect={vi.fn()}
      />,
    )

    expect(adapter.fitFeature).not.toHaveBeenCalled()
  })
})
