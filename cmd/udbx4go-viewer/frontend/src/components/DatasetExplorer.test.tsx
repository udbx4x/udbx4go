import { ThemeProvider } from '@mui/material/styles'
import { fireEvent, render, screen } from '@testing-library/react'
import { beforeAll, describe, expect, it, vi } from 'vitest'
import { datasetFixtures, mapLayerFixtures } from '../test/fixtures'
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

const renderExplorer = (onSelectDataset = vi.fn()) => ({
  ...render(
    <ThemeProvider theme={viewerTheme}>
      <DatasetExplorer
        datasets={datasetFixtures}
        selectedDataset="BaseMap_P"
        activeTableDataset={null}
        mapLayers={mapLayerFixtures}
        onSelectDataset={onSelectDataset}
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

  it('显示数据集列表摘要和已加入地图状态', () => {
    renderExplorer()

    expect(screen.getByText('数据集')).toBeInTheDocument()
    expect(screen.getByText('4 个数据集')).toBeInTheDocument()
    expect(screen.getByText('已加入地图')).toBeInTheDocument()
  })

  it('点击数据集时通知选中项', () => {
    const onSelectDataset = vi.fn()
    renderExplorer(onSelectDataset)

    fireEvent.click(screen.getByText('BaseMap_L'))

    expect(onSelectDataset).toHaveBeenCalledWith('BaseMap_L')
  })
})
