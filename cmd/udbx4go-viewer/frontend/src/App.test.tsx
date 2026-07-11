import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
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
type CapturedSettingsDialogProps = {
  open: boolean
  settings: ViewerSettings
  disabled: boolean
  onClose: () => void
  onSave: (settings: ViewerSettings) => void | Promise<void>
  onReset: () => void | Promise<void>
}
type CapturedMapWorkspaceProps = {
  autoFitOnLayerChange: boolean
  zoomToSelectedFeature: boolean
}
type CapturedInspectorPanelProps = {
  showPreviewStats: boolean
}

let capturedSettingsDialogProps: CapturedSettingsDialogProps | null = null
let capturedMapWorkspaceProps: CapturedMapWorkspaceProps | null = null
let capturedInspectorPanelProps: CapturedInspectorPanelProps | null = null

vi.mock('./hooks/useUDBX', () => ({
  useUDBX: (options: unknown) => mockUseUDBX(options),
}))

vi.mock('./hooks/useViewerSettings', () => ({
  useViewerSettings: () => mockUseViewerSettings(),
}))

vi.mock('./components/AppShell', () => ({
  AppShell: ({
    toolbar,
    mapWorkspace,
    inspector,
    tableDrawer,
  }: {
    toolbar: ReactNode
    mapWorkspace: ReactNode
    inspector: ReactNode
    tableDrawer: ReactNode
  }) => (
    <div>
      {toolbar}
      {mapWorkspace}
      {inspector}
      {tableDrawer}
    </div>
  ),
}))

vi.mock('./components/TopToolbar', () => ({
  TopToolbar: ({ onOpenSettings }: { onOpenSettings: () => void }) => (
    <div>
      <button type="button" onClick={onOpenSettings}>
        设置
      </button>
    </div>
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
  MapWorkspace: (props: CapturedMapWorkspaceProps) => {
    capturedMapWorkspaceProps = props
    return null
  },
}))

vi.mock('./components/InspectorPanel', () => ({
  InspectorPanel: (props: CapturedInspectorPanelProps) => {
    capturedInspectorPanelProps = props
    return null
  },
}))

vi.mock('./components/SettingsDialog', () => ({
  SettingsDialog: (props: CapturedSettingsDialogProps) => {
    capturedSettingsDialogProps = props
    return (
      <div data-testid="settings-dialog">
        <span data-testid="settings-dialog-open">{String(props.open)}</span>
        <span data-testid="settings-dialog-disabled">{String(props.disabled)}</span>
      </div>
    )
  },
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
    capturedSettingsDialogProps = null
    capturedMapWorkspaceProps = null
    capturedInspectorPanelProps = null
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

    expect(screen.getByRole('button', { name: '属性表已展开' })).toBeInTheDocument()

    mockUseViewerSettings.mockReturnValue({
      settings: loadedSettings,
      loading: false,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })
    rerender(<App />)

    await waitFor(() => expect(screen.getByRole('button', { name: '属性表已收起' })).toBeInTheDocument())

    fireEvent.click(screen.getByRole('button', { name: '属性表已收起' }))
    fireEvent.click(screen.getByRole('button', { name: '属性表已展开' }))

    mockUseViewerSettings.mockReturnValue({
      settings: defaultViewerSettings,
      loading: false,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })
    rerender(<App />)

    await waitFor(() => expect(screen.getByRole('button', { name: '属性表已收起' })).toBeInTheDocument())
  })

  it('向 UDBX hook 和地图工作区传入 viewer settings', () => {
    const settings: ViewerSettings = {
      ...defaultViewerSettings,
      spatialPreview: {
        featureLimit: 2500,
        vertexBudget: 250000,
        autoFitOnLayerChange: false,
      },
      mapInteraction: {
        zoomToSelectedFeature: false,
      },
      advanced: {
        showPreviewStats: true,
      },
    }

    mockUseViewerSettings.mockReturnValue({
      settings,
      loading: false,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })

    render(<App />)

    expect(mockUseUDBX).toHaveBeenCalledWith({
      spatialPreviewFeatureLimit: 2500,
      spatialPreviewVertexBudget: 250000,
    })
    expect(capturedMapWorkspaceProps?.autoFitOnLayerChange).toBe(false)
    expect(capturedMapWorkspaceProps?.zoomToSelectedFeature).toBe(false)
    expect(capturedInspectorPanelProps?.showPreviewStats).toBe(true)
  })

  it('点击工具栏设置入口会打开设置弹窗', async () => {
    mockUseViewerSettings.mockReturnValue({
      settings: defaultViewerSettings,
      loading: false,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })

    render(<App />)

    expect(screen.getByTestId('settings-dialog-open')).toHaveTextContent('false')

    fireEvent.click(screen.getByRole('button', { name: '设置' }))

    await waitFor(() => expect(screen.getByTestId('settings-dialog-open')).toHaveTextContent('true'))
    expect(capturedSettingsDialogProps?.open).toBe(true)
  })

  it('设置保存成功时调用 saveSettings 并关闭弹窗', async () => {
    const saveSettings = vi.fn().mockResolvedValue(defaultViewerSettings)

    mockUseViewerSettings.mockReturnValue({
      settings: defaultViewerSettings,
      loading: false,
      error: null,
      saveSettings,
      resetSettings: vi.fn(),
    })

    render(<App />)
    fireEvent.click(screen.getByRole('button', { name: '设置' }))

    await waitFor(() => expect(capturedSettingsDialogProps?.open).toBe(true))
    await act(async () => {
      await expect(capturedSettingsDialogProps?.onSave(defaultViewerSettings)).resolves.toBeUndefined()
    })

    expect(saveSettings).toHaveBeenCalledWith(defaultViewerSettings)
    await waitFor(() => expect(screen.getByTestId('settings-dialog-open')).toHaveTextContent('false'))
    expect(screen.getByTestId('settings-dialog-disabled')).toHaveTextContent('false')
  })

  it('设置保存失败时不抛出、不关闭弹窗并恢复可操作状态', async () => {
    const saveSettings = vi.fn().mockRejectedValue(new Error('保存失败'))

    mockUseViewerSettings.mockReturnValue({
      settings: defaultViewerSettings,
      loading: false,
      error: null,
      saveSettings,
      resetSettings: vi.fn(),
    })

    render(<App />)
    fireEvent.click(screen.getByRole('button', { name: '设置' }))

    await waitFor(() => expect(capturedSettingsDialogProps?.open).toBe(true))
    await act(async () => {
      await expect(capturedSettingsDialogProps?.onSave(defaultViewerSettings)).resolves.toBeUndefined()
    })

    expect(saveSettings).toHaveBeenCalledWith(defaultViewerSettings)
    expect(screen.getByTestId('settings-dialog-open')).toHaveTextContent('true')
    expect(screen.getByTestId('settings-dialog-disabled')).toHaveTextContent('false')
  })

  it('设置重置失败时不抛出并恢复可操作状态', async () => {
    const resetSettings = vi.fn().mockRejectedValue(new Error('重置失败'))

    mockUseViewerSettings.mockReturnValue({
      settings: defaultViewerSettings,
      loading: false,
      error: null,
      saveSettings: vi.fn(),
      resetSettings,
    })

    render(<App />)
    fireEvent.click(screen.getByRole('button', { name: '设置' }))

    await waitFor(() => expect(capturedSettingsDialogProps?.open).toBe(true))
    await act(async () => {
      await expect(capturedSettingsDialogProps?.onReset()).resolves.toBeUndefined()
    })

    expect(resetSettings).toHaveBeenCalledTimes(1)
    expect(screen.getByTestId('settings-dialog-open')).toHaveTextContent('true')
    expect(screen.getByTestId('settings-dialog-disabled')).toHaveTextContent('false')
  })
})
