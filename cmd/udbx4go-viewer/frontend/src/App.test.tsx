import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { defaultViewerSettings } from './settings/viewerSettings'
import type { ViewerSettings } from './settings/viewerSettings'

declare global {
  interface Window {
    __vite_plugin_react_preamble_installed__?: boolean
  }
}

const mockUseUDBX = vi.fn()
const mockUseViewerSettings = vi.fn()

vi.mock('./hooks/useUDBX', () => ({
  useUDBX: () => mockUseUDBX(),
}))

vi.mock('./hooks/useViewerSettings', () => ({
  useViewerSettings: () => mockUseViewerSettings(),
}))

vi.mock('./components/AppShell', () => ({
  AppShell: ({ toolbar, tableDrawer }: { toolbar: ReactNode; tableDrawer: ReactNode }) => (
    <div>
      {toolbar}
      {tableDrawer}
    </div>
  ),
}))

vi.mock('./components/TopToolbar', () => ({
  TopToolbar: ({
    tableOpen,
    onToggleTable,
  }: {
    tableOpen: boolean
    onToggleTable: () => void
  }) => (
    <button type="button" onClick={onToggleTable}>
      {tableOpen ? '收起属性表' : '展开属性表'}
    </button>
  ),
}))

vi.mock('./components/AttributeTableDrawer', () => ({
  AttributeTableDrawer: ({
    open,
    onToggleOpen,
  }: {
    open: boolean
    onToggleOpen: () => void
  }) => (
    <button type="button" onClick={onToggleOpen}>
      {open ? '属性表已展开' : '属性表已收起'}
    </button>
  ),
}))

vi.mock('./components/DatasetExplorer', () => ({
  DatasetExplorer: () => null,
}))

vi.mock('./components/MapWorkspace', () => ({
  MapWorkspace: () => null,
}))

vi.mock('./components/InspectorPanel', () => ({
  InspectorPanel: () => null,
}))

vi.mock('./components/SettingsDialog', () => ({
  SettingsDialog: () => null,
}))

let App: typeof import('./App').default

const baseUdbxState = {
  currentFile: null,
  datasets: [],
  selectedDataset: null,
  activeTableDataset: null,
  pageData: null,
  mapLayers: [],
  selectedMapFeature: null,
  selectedFeatureAttributes: null,
  loading: false,
  error: null,
  openFileDialog: vi.fn(),
  closeFile: vi.fn(),
  loadDataset: vi.fn(),
  setMapLayerVisible: vi.fn(),
  removeMapLayer: vi.fn(),
  selectFeature: vi.fn(),
}

const loadedSettings: ViewerSettings = {
  ...defaultViewerSettings,
  table: {
    defaultOpen: false,
  },
}

describe('App settings integration', () => {
  beforeAll(async () => {
    window.__vite_plugin_react_preamble_installed__ = true
    App = (await import('./App')).default
  })

  beforeEach(() => {
    vi.clearAllMocks()
    mockUseUDBX.mockReturnValue(baseUdbxState)
    mockUseViewerSettings.mockReturnValue({
      settings: defaultViewerSettings,
      loading: true,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })
  })

  it('只在设置首次加载完成后应用默认属性表展开状态', async () => {
    const { rerender } = render(<App />)

    expect(screen.getByRole('button', { name: '收起属性表' })).toBeInTheDocument()

    mockUseViewerSettings.mockReturnValue({
      settings: loadedSettings,
      loading: false,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })
    rerender(<App />)

    await waitFor(() => expect(screen.getByRole('button', { name: '展开属性表' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: '展开属性表' }))
    fireEvent.click(screen.getByRole('button', { name: '收起属性表' }))

    mockUseViewerSettings.mockReturnValue({
      settings: defaultViewerSettings,
      loading: false,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })
    rerender(<App />)

    await waitFor(() => expect(screen.getByRole('button', { name: '展开属性表' })).toBeInTheDocument())
  })
})
