import { render } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mapLayerFixtures, selectedFeatureFixture } from '../test/fixtures'
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
      setSelection: vi.fn(),
      fitFeature: vi.fn(),
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
