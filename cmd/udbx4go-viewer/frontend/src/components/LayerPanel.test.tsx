import { ThemeProvider } from '@mui/material/styles'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { LayerPanel } from './LayerPanel'
import {
  createDegradedSpatialPreviewFixture,
  createSpatialPreviewFixture,
  mapLayerFixtures,
  sampledMapLayerFixture,
} from '../test/fixtures'
import { viewerTheme } from '../theme/viewerTheme'
import type { BoundingBox, MapLayerState } from '../types'

type LayerPanelProps = {
  layers: MapLayerState[]
  showPreviewStats: boolean
  onVisibleChange: (datasetName: string, visible: boolean) => void
  onRemoveLayer: (datasetName: string) => void
}

const renderLayerPanel = (props: Partial<LayerPanelProps> = {}) => {
  const defaultProps: LayerPanelProps = {
    layers: mapLayerFixtures,
    showPreviewStats: false,
    onVisibleChange: vi.fn(),
    onRemoveLayer: vi.fn(),
  }

  const mergedProps = { ...defaultProps, ...props }

  return {
    ...render(
      <ThemeProvider theme={viewerTheme}>
        <LayerPanel {...mergedProps} />
      </ThemeProvider>,
    ),
    props: mergedProps,
  }
}

describe('LayerPanel', () => {
  const secondMapLayerFixture: MapLayerState = {
    ...sampledMapLayerFixture,
    datasetName: 'BaseMap_L',
    kind: 'line',
    visible: false,
    preview: createSpatialPreviewFixture({
      datasetName: 'BaseMap_L',
      kind: 'line',
      strategy: 'rtree',
    }),
  }

  it('显示地图图层摘要并支持切换可见性和移除', () => {
    const onVisibleChange = vi.fn()
    const onRemoveLayer = vi.fn()

    renderLayerPanel({ onVisibleChange, onRemoveLayer })

    expect(screen.getByText('地图图层')).toBeInTheDocument()
    expect(screen.getByText('BaseMap_P')).toBeInTheDocument()
    expect(screen.getByText('point · 0 个预览要素')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('checkbox', { name: '切换 BaseMap_P 图层可见性' }))
    fireEvent.click(screen.getByRole('button', { name: 'BaseMap_P 更多操作' }))

    expect(screen.getByRole('menuitem', { name: '样式设置' })).toHaveAttribute('aria-disabled', 'true')

    fireEvent.click(screen.getByRole('menuitem', { name: '移除图层' }))

    expect(onVisibleChange).toHaveBeenCalledWith('BaseMap_P', false)
    expect(onRemoveLayer).toHaveBeenCalledWith('BaseMap_P')
  })

  it('开启预览统计时显示要素数和顶点数', () => {
    const [layer] = mapLayerFixtures
    const sampledLayer: MapLayerState = {
      ...layer,
      preview: layer.preview && {
        ...layer.preview,
        features: [
          {
            id: 1,
            geometry: {
              type: 'Point',
              coordinates: [113.5, 34.8],
              hasZ: false,
            },
            properties: {},
          },
        ],
        estimatedVertexCount: 1,
        sampled: true,
        sampleReason: '达到预览上限',
      },
    }

    renderLayerPanel({ layers: [sampledLayer], showPreviewStats: true })

    expect(screen.getByText('bounded_sample · 0 ms · 要素 1 · 顶点 1')).toBeInTheDocument()
  })

  it('多图层时通过当前打开的菜单移除对应图层', () => {
    const onRemoveLayer = vi.fn()

    renderLayerPanel({ layers: [mapLayerFixtures[0], secondMapLayerFixture], onRemoveLayer })

    fireEvent.click(screen.getByRole('button', { name: 'BaseMap_L 更多操作' }))
    fireEvent.click(screen.getByRole('menuitem', { name: '移除图层' }))

    expect(onRemoveLayer).toHaveBeenCalledTimes(1)
    expect(onRemoveLayer).toHaveBeenCalledWith('BaseMap_L')
    expect(onRemoveLayer).not.toHaveBeenCalledWith('BaseMap_P')
  })

  it('图层列表移除当前菜单目标时关闭菜单且不会触发移除回调', () => {
    const onRemoveLayer = vi.fn()
    const { rerender } = renderLayerPanel({ layers: [mapLayerFixtures[0]], onRemoveLayer })

    fireEvent.click(screen.getByRole('button', { name: 'BaseMap_P 更多操作' }))
    expect(screen.getByRole('menuitem', { name: '移除图层' })).toBeInTheDocument()

    rerender(
      <ThemeProvider theme={viewerTheme}>
        <LayerPanel
          layers={[]}
          showPreviewStats={false}
          onVisibleChange={vi.fn()}
          onRemoveLayer={onRemoveLayer}
        />
      </ThemeProvider>,
    )

    expect(screen.queryByRole('menuitem', { name: '移除图层' })).not.toBeInTheDocument()
    expect(onRemoveLayer).not.toHaveBeenCalled()
  })

  it('关闭预览统计时仍内联显示采样原因并可通过更多菜单移除图层', () => {
    const onRemoveLayer = vi.fn()

    renderLayerPanel({ layers: [sampledMapLayerFixture], onRemoveLayer })

    expect(screen.getByText('预览达到要素上限')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Jingjin_NetworkZ_Node 更多操作' }))
    fireEvent.click(screen.getByRole('menuitem', { name: '移除图层' }))

    expect(onRemoveLayer).toHaveBeenCalledWith('Jingjin_NetworkZ_Node')
  })

  it('开启预览统计时采样原因只显示一次', () => {
    renderLayerPanel({ layers: [sampledMapLayerFixture], showPreviewStats: true })

    expect(screen.getAllByText('预览达到要素上限')).toHaveLength(1)
    expect(screen.getByText('bounded_sample · 0 ms · 要素 0 · 顶点 50000')).toBeInTheDocument()
  })

  it('loading 优先于旧 hasMore 和 sampled，完成后恢复 degraded 与采样提示', () => {
    const loadingLayer: MapLayerState = {
      ...mapLayerFixtures[0],
      queryStatus: 'loading',
      preview: createSpatialPreviewFixture({
        strategy: 'bounded_sample',
        hasMore: true,
        sampled: true,
        sampleReason: '旧采样原因',
      }),
    }
    const { rerender } = renderLayerPanel({ layers: [loadingLayer], showPreviewStats: true })

    expect(screen.getByText('加载当前范围')).toBeInTheDocument()
    expect(screen.queryByText(/当前范围 .*\+ 个对象/)).not.toBeInTheDocument()
    expect(screen.queryByText('旧采样原因')).not.toBeInTheDocument()
    expect(screen.queryByText(/bounded_sample ·/)).not.toBeInTheDocument()

    const completedLayer: MapLayerState = {
      ...loadingLayer,
      queryStatus: 'degraded',
      preview: {
        ...createDegradedSpatialPreviewFixture('spatial_index_unavailable'),
        sampled: true,
        sampleReason: '新采样原因',
      },
    }
    rerender(
      <ThemeProvider theme={viewerTheme}>
        <LayerPanel
          layers={[completedLayer]}
          showPreviewStats
          onVisibleChange={vi.fn()}
          onRemoveLayer={vi.fn()}
        />
      </ThemeProvider>,
    )

    expect(screen.getByText('范围索引不可用，显示有界预览')).toBeInTheDocument()
    expect(screen.getByText('新采样原因')).toBeInTheDocument()
    expect(screen.getByText(/bounded_sample ·/)).toBeInTheDocument()
  })

  it('error 优先于旧 sampled 并隐藏旧预览辅助提示', () => {
    const layer: MapLayerState = {
      ...sampledMapLayerFixture,
      queryStatus: 'error',
      queryError: '当前请求失败',
    }

    renderLayerPanel({ layers: [layer], showPreviewStats: true })

    expect(screen.getByText('当前请求失败')).toBeInTheDocument()
    expect(screen.queryByText('预览达到要素上限')).not.toBeInTheDocument()
    expect(screen.queryByText(/bounded_sample ·/)).not.toBeInTheDocument()
  })

  it.each([
    ['error', 'backend details'],
    ['degraded', '范围索引不可用，显示有界预览'],
    ['ready', '当前范围 0+ 个对象，请继续放大'],
    ['loading', '加载当前范围'],
  ] as const)('按最高优先级显示 %s 查询状态', (queryStatus, message) => {
    const layer: MapLayerState = {
      ...mapLayerFixtures[0],
      queryStatus,
      queryError: queryStatus === 'error' ? 'backend details' : null,
      preview: queryStatus === 'degraded'
        ? createDegradedSpatialPreviewFixture('spatial_index_unavailable')
        : createSpatialPreviewFixture({
            strategy: 'rtree',
            hasMore: queryStatus === 'ready',
          }),
    }

    renderLayerPanel({ layers: [layer] })

    expect(screen.getByText(message)).toBeInTheDocument()
  })

  it.each([
    ['envelope_cache_budget_exceeded', '缓存预算不足，显示有界预览'],
    ['spatial_index_unavailable', '范围索引不可用，显示有界预览'],
  ] as const)('%s 显示明确中文降级原因', (degradedReason, message) => {
    const layer: MapLayerState = {
      ...mapLayerFixtures[0],
      queryStatus: 'degraded',
      preview: createDegradedSpatialPreviewFixture(degradedReason),
    }

    renderLayerPanel({ layers: [layer] })

    expect(screen.getByText(message)).toBeInTheDocument()
  })

  it('ShowPreviewStats 开启时显示查询策略、耗时、要素、顶点和原因', () => {
    const layer: MapLayerState = {
      ...mapLayerFixtures[0],
      queryStatus: 'degraded',
      preview: createDegradedSpatialPreviewFixture('spatial_index_unavailable', {
        queryDurationMs: 12.5,
        hasMore: false,
      }),
    }

    renderLayerPanel({ layers: [layer], showPreviewStats: true })

    expect(screen.getByText('bounded_sample · 12.5 ms · 要素 0 · 顶点 0 · spatial_index_unavailable')).toBeInTheDocument()
  })

  it.each(['text', 'cad'])('%s bounded preview 不显示空间索引降级', (kind) => {
    const layer: MapLayerState = {
      ...mapLayerFixtures[0],
      kind,
      queryStatus: 'ready',
      preview: createSpatialPreviewFixture({
        kind,
        strategy: 'bounded_sample',
      }),
    }

    renderLayerPanel({ layers: [layer] })

    expect(screen.queryByText('缓存预算不足，显示有界预览')).not.toBeInTheDocument()
    expect(screen.queryByText('范围索引不可用，显示有界预览')).not.toBeInTheDocument()
    expect(screen.getByText(`${kind} · 0 个预览要素`)).toBeInTheDocument()
  })

  it('hasMore 计数排除视口外 required feature', () => {
    const layer = viewportLayerWithRequiredFeature({ minX: 200, minY: 200, maxX: 200, maxY: 200 })

    renderLayerPanel({ layers: [layer] })

    expect(screen.getByText('当前范围 2+ 个对象，请继续放大')).toBeInTheDocument()
  })

  it('hasMore 计数保留视口内 required feature且不误扣', () => {
    const layer = viewportLayerWithRequiredFeature({ minX: 20, minY: 20, maxX: 20, maxY: 20 })

    renderLayerPanel({ layers: [layer] })

    expect(screen.getByText('当前范围 3+ 个对象，请继续放大')).toBeInTheDocument()
  })
})

function viewportLayerWithRequiredFeature(requiredBBox: BoundingBox): MapLayerState {
  return {
    ...mapLayerFixtures[0],
    queryStatus: 'ready',
    preview: {
      ...mapLayerFixtures[0].preview!,
      queriedBounds: { minX: 0, minY: 0, maxX: 100, maxY: 100 },
      hasMore: true,
      viewportFeatureCount: requiredBBox.minX > 100 ? 2 : 3,
      features: [
        { id: 1, bbox: { minX: 10, minY: 10, maxX: 10, maxY: 10 }, geometry: { type: 'Point', coordinates: [10, 10], hasZ: false } },
        { id: 2, bbox: { minX: 30, minY: 30, maxX: 30, maxY: 30 }, geometry: { type: 'Point', coordinates: [30, 30], hasZ: false } },
        { id: 7, bbox: requiredBBox, geometry: { type: 'Point', coordinates: [requiredBBox.minX, requiredBBox.minY], hasZ: false } },
      ],
    },
  }
}
