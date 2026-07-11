import { ThemeProvider } from '@mui/material/styles'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { LayerPanel } from './LayerPanel'
import { mapLayerFixtures, sampledMapLayerFixture } from '../test/fixtures'
import { viewerTheme } from '../theme/viewerTheme'
import type { MapLayerState } from '../types'

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
    preview: {
      datasetName: 'BaseMap_L',
      kind: 'line',
      features: [],
      estimatedVertexCount: 0,
      sampled: false,
    },
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

    expect(screen.getByText('预览要素 1')).toBeInTheDocument()
    expect(screen.getByText('顶点 1')).toBeInTheDocument()
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
    expect(screen.getByText('预览要素 0')).toBeInTheDocument()
    expect(screen.getByText('顶点 50000')).toBeInTheDocument()
  })
})
