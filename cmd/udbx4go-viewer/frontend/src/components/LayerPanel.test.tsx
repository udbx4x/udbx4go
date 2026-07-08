import { ThemeProvider } from '@mui/material/styles'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { LayerPanel } from './LayerPanel'
import { mapLayerFixtures } from '../test/fixtures'
import { viewerTheme } from '../theme/viewerTheme'
import type { MapLayerState } from '../types'

type LayerPanelProps = {
  layers: MapLayerState[]
  onVisibleChange: (datasetName: string, visible: boolean) => void
  onRemoveLayer: (datasetName: string) => void
}

const renderLayerPanel = (props: Partial<LayerPanelProps> = {}) => {
  const defaultProps: LayerPanelProps = {
    layers: mapLayerFixtures,
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
})
