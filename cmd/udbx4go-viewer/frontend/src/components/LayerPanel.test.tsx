import { ThemeProvider } from '@mui/material/styles'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { LayerPanel } from './LayerPanel'
import { mapLayerFixtures } from '../test/fixtures'
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
  it('显示地图图层摘要并支持切换可见性和移除', () => {
    const onVisibleChange = vi.fn()
    const onRemoveLayer = vi.fn()

    renderLayerPanel({ onVisibleChange, onRemoveLayer })

    expect(screen.getByText('地图图层')).toBeInTheDocument()
    expect(screen.getByText('BaseMap_P')).toBeInTheDocument()
    expect(screen.getByText('point · 0 个预览要素')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('checkbox', { name: '切换 BaseMap_P 图层可见性' }))
    fireEvent.click(screen.getByRole('button', { name: '从地图移除 BaseMap_P' }))

    expect(onVisibleChange).toHaveBeenCalledWith('BaseMap_P', false)
    expect(onRemoveLayer).toHaveBeenCalledWith('BaseMap_P')
  })

  it('开启预览统计时显示要素数、顶点数和采样提示', () => {
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
    expect(screen.getByText('达到预览上限')).toBeInTheDocument()
  })
})
