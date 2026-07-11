import { ThemeProvider } from '@mui/material/styles'
import { fireEvent, render, screen } from '@testing-library/react'
import { beforeAll, describe, expect, it, vi } from 'vitest'
import {
  datasetFixtures,
  mapLayerFixtures,
  sampledMapLayerFixture,
} from '../test/fixtures'
import { viewerTheme } from '../theme/viewerTheme'
import type { DatasetInfo, MapLayerState } from '../types'

declare global {
  interface Window {
    __vite_plugin_react_preamble_installed__?: boolean
  }
}

type DatasetExplorerProps = {
  datasets: DatasetInfo[]
  selectedDataset: string | null
  activeTableDataset: string | null
  mapLayers: MapLayerState[]
  onSelectDataset: (name: string) => void
}

let DatasetExplorer: React.FC<DatasetExplorerProps>

const renderExplorer = (
  onSelectDataset = vi.fn(),
  props: Partial<DatasetExplorerProps> = {},
) => ({
  ...render(
    <ThemeProvider theme={viewerTheme}>
      <DatasetExplorer
        datasets={datasetFixtures}
        selectedDataset="BaseMap_P"
        activeTableDataset={null}
        mapLayers={mapLayerFixtures}
        onSelectDataset={onSelectDataset}
        {...props}
      />
    </ThemeProvider>,
  ),
  onSelectDataset,
})

describe('DatasetExplorer', () => {
  beforeAll(async () => {
    window.__vite_plugin_react_preamble_installed__ = true
    DatasetExplorer = (await import('./DatasetExplorer')).DatasetExplorer
  })

  it('显示数据集列表摘要和已加入状态', () => {
    renderExplorer()

    expect(screen.getByText('数据集')).toBeInTheDocument()
    expect(screen.getByText('6 个数据集')).toBeInTheDocument()
    expect(screen.getByText('已加入')).toBeInTheDocument()
  })

  it('点击数据集时通知选中项', () => {
    const onSelectDataset = vi.fn()
    renderExplorer(onSelectDataset)

    fireEvent.click(screen.getByText('BaseMap_L'))

    expect(onSelectDataset).toHaveBeenCalledWith('BaseMap_L')
    expect(onSelectDataset).toHaveBeenCalledTimes(1)
  })

  it('按数据集名称搜索', () => {
    renderExplorer()

    fireEvent.change(screen.getByRole('textbox', { name: '搜索数据集' }), {
      target: { value: 'Jingjin' },
    })

    expect(screen.getByText('Jingjin_NetworkZ_Node')).toBeInTheDocument()
    expect(screen.queryByText('BaseMap_P')).not.toBeInTheDocument()
  })

  it('按数据集类型筛选未知类型', () => {
    renderExplorer()

    fireEvent.change(screen.getByRole('textbox', { name: '搜索数据集' }), {
      target: { value: 'Jingjin' },
    })
    fireEvent.change(screen.getByRole('textbox', { name: '搜索数据集' }), {
      target: { value: '' },
    })
    fireEvent.click(screen.getByRole('button', { name: '未知' }))

    expect(screen.getByText('modeldt_Texture')).toBeInTheDocument()
    expect(screen.queryByText('BaseMap_P')).not.toBeInTheDocument()
  })

  it('长名称带完整 title 且已加入地图时显示轻量状态', () => {
    renderExplorer(vi.fn(), {
      mapLayers: [...mapLayerFixtures, sampledMapLayerFixture],
    })

    expect(screen.getByText('Jingjin_NetworkZ_Node')).toHaveAttribute(
      'title',
      'Jingjin_NetworkZ_Node',
    )
    expect(screen.getAllByText('已加入')).toHaveLength(2)
  })
})
