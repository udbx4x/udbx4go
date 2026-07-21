import { ThemeProvider } from '@mui/material/styles'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { InspectorPanel } from './InspectorPanel'
import { featureAttributesFixture, mapLayerFixtures } from '../test/fixtures'
import { viewerTheme } from '../theme/viewerTheme'
import type { FeatureAttributes, MapLayerState } from '../types'

type InspectorPanelProps = {
  layers: MapLayerState[]
  showPreviewStats: boolean
  selectedFeatureAttributes: FeatureAttributes | null
  onLayerVisibleChange: (datasetName: string, visible: boolean) => void
  onRemoveLayer: (datasetName: string) => void
}

const renderInspectorPanel = (props: Partial<InspectorPanelProps> = {}) => {
  const defaultProps: InspectorPanelProps = {
    layers: mapLayerFixtures,
    showPreviewStats: false,
    selectedFeatureAttributes: null,
    onLayerVisibleChange: vi.fn(),
    onRemoveLayer: vi.fn(),
  }

  const mergedProps = { ...defaultProps, ...props }
  const result = render(
    <ThemeProvider theme={viewerTheme}>
      <InspectorPanel {...mergedProps} />
    </ThemeProvider>,
  )

  return {
    ...result,
    props: mergedProps,
  }
}

describe('InspectorPanel', () => {
  it('默认选中图层 tab 并显示地图图层内容', () => {
    renderInspectorPanel()

    expect(screen.getByRole('tab', { name: '图层', selected: true })).toBeInTheDocument()
    expect(screen.getByText('地图图层')).toBeInTheDocument()
  })

  it('可以从图层 tab 手动切换到属性 tab 并显示空状态', () => {
    renderInspectorPanel({ selectedFeatureAttributes: null })

    expect(screen.getByRole('tab', { name: '图层', selected: true })).toBeInTheDocument()

    fireEvent.click(screen.getByRole('tab', { name: '属性' }))

    expect(screen.getByRole('tab', { name: '属性', selected: true })).toBeInTheDocument()
    expect(screen.getByText('要素属性')).toBeInTheDocument()
    expect(screen.getByText('点击地图要素或属性表行查看属性')).toBeInTheDocument()
  })

  it('可以切换到样式 tab 并显示首阶段空状态', () => {
    renderInspectorPanel()

    fireEvent.click(screen.getByRole('tab', { name: '样式' }))

    expect(screen.getByRole('tab', { name: '样式', selected: true })).toBeInTheDocument()
    expect(screen.getByText('样式设置将在后续版本支持')).toBeInTheDocument()
  })

  it('选中要素属性从空变为非空时自动切换到属性 tab', () => {
    const { rerender, props } = renderInspectorPanel({ selectedFeatureAttributes: null })

    expect(screen.getByRole('tab', { name: '图层', selected: true })).toBeInTheDocument()

    rerender(
      <ThemeProvider theme={viewerTheme}>
        <InspectorPanel {...props} selectedFeatureAttributes={featureAttributesFixture} />
      </ThemeProvider>,
    )

    expect(screen.getByRole('tab', { name: '属性', selected: true })).toBeInTheDocument()
    expect(screen.getByText('BaseMap_P')).toBeInTheDocument()
  })

  it('向图层 tab 透传 showPreviewStats', () => {
    renderInspectorPanel({ showPreviewStats: true })

    expect(screen.getByText('bounded_sample · 0 ms · 要素 0 · 顶点 0')).toBeInTheDocument()
  })
})
