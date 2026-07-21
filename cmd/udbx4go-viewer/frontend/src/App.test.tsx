import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { defaultViewerSettings } from './settings/viewerSettings'
import type { ViewerSettings } from './settings/viewerSettings'
import {
  datasetFixtures,
  featureAttributesFixture,
  mapLayerFixtures,
  pageDataFixture,
  selectedFeatureFixture,
} from './test/fixtures'
import type { DatasetInfo, FeatureAttributes, MapLayerState, PageData, SelectedMapFeature } from './types'
import type { AttributeTableMode } from './components/AttributeTableDrawer'

declare global {
  interface Window {
    __vite_plugin_react_preamble_installed__?: boolean
  }
}

const mockUseUDBX = vi.fn()
const mockUseViewerSettings = vi.fn()
const mockGetBenchmarkConfig = vi.fn()
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
  selectedFeatureAttributes: FeatureAttributes | null
  selectionLocationError: string | null
  onViewportChange: (viewport: { minX: number; minY: number; maxX: number; maxY: number }) => void
}
type CapturedInspectorPanelProps = {
  layers: MapLayerState[]
  showPreviewStats: boolean
  selectedFeatureAttributes: FeatureAttributes | null
  onLayerVisibleChange: (datasetName: string, visible: boolean) => void
  onRemoveLayer: (datasetName: string) => void
}
type CapturedDatasetExplorerProps = {
  datasets: DatasetInfo[]
  selectedDataset: string | null
  activeTableDataset: string | null
  mapLayers: MapLayerState[]
  onSelectDataset: (name: string) => void
}
type CapturedAttributeTableDrawerProps = {
  mode: AttributeTableMode
  pageData: PageData | null
  datasetName: string | null
  selectedFeature: SelectedMapFeature | null
  onModeChange: (mode: AttributeTableMode) => void
  onFeatureSelect: (datasetName: string, featureID: number) => void
  onPageChange: (page: number) => void
}

let capturedSettingsDialogProps: CapturedSettingsDialogProps | null = null
let capturedMapWorkspaceProps: CapturedMapWorkspaceProps | null = null
let capturedInspectorPanelProps: CapturedInspectorPanelProps | null = null
let capturedDatasetExplorerProps: CapturedDatasetExplorerProps | null = null
let capturedAttributeTableDrawerProps: CapturedAttributeTableDrawerProps | null = null

vi.mock('./hooks/useUDBX', () => ({
  useUDBX: (options: unknown) => mockUseUDBX(options),
}))

vi.mock('./hooks/useViewerSettings', () => ({
  useViewerSettings: () => mockUseViewerSettings(),
}))

vi.mock('../wailsjs/go/main/App', () => ({
  GetBenchmarkConfig: () => mockGetBenchmarkConfig(),
}))

vi.mock('./benchmark/BenchmarkRunner', () => ({
  BenchmarkRunner: ({ config }: { config: { scenario: { name: string } } }) => (
    <div>{`基准模式 ${config.scenario.name}`}</div>
  ),
}))

vi.mock('./components/AppShell', () => ({
  AppShell: ({
    toolbar,
    datasetExplorer,
    mapWorkspace,
    inspector,
    tableDrawer,
  }: {
    toolbar: ReactNode
    datasetExplorer: ReactNode
    mapWorkspace: ReactNode
    inspector: ReactNode
    tableDrawer: ReactNode
  }) => (
    <div>
      {toolbar}
      {datasetExplorer}
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
  AttributeTableDrawer: (props: CapturedAttributeTableDrawerProps) => {
    capturedAttributeTableDrawerProps = props
    return (
      <button type="button" onClick={() => props.onModeChange('collapsed')}>
        {`属性表模式 ${props.mode}`}
      </button>
    )
  },
}))

vi.mock('./components/DatasetExplorer', () => ({
  DatasetExplorer: (props: CapturedDatasetExplorerProps) => {
    capturedDatasetExplorerProps = props
    return null
  },
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
let ApplicationRoot: typeof import('./App').default

const baseUdbxState = {
  currentFile: null,
  datasets: [],
  selectedDataset: null,
  activeTableDataset: null,
  pageData: null,
  mapLayers: [],
  selectedMapFeature: null,
  selectedFeatureAttributes: null,
  selectionLocationError: null,
  loading: false,
  error: null,
  openFileDialog: vi.fn(),
  closeFile: vi.fn(),
  loadDataset: vi.fn(),
  loadTableDataset: vi.fn(),
  setMapLayerVisible: vi.fn(),
  removeMapLayer: vi.fn(),
  selectFeature: vi.fn(),
  queryViewport: vi.fn(),
  loadCurrentFile: vi.fn(),
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
    const appModule = await import('./App')
    App = appModule.ViewerApp
    ApplicationRoot = appModule.default
  })

  beforeEach(() => {
    vi.clearAllMocks()
    capturedSettingsDialogProps = null
    capturedMapWorkspaceProps = null
    capturedInspectorPanelProps = null
    capturedDatasetExplorerProps = null
    capturedAttributeTableDrawerProps = null
    mockUseUDBX.mockReturnValue(baseUdbxState)
    mockGetBenchmarkConfig.mockResolvedValue(null)
    mockUseViewerSettings.mockReturnValue({
      settings: defaultViewerSettings,
      loading: true,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })
  })

  it('默认入口在无基准配置时进入普通 Viewer', async () => {
    render(<ApplicationRoot />)

    await waitFor(() => expect(screen.getByRole('button', { name: '设置' })).toBeInTheDocument())
    expect(mockGetBenchmarkConfig).toHaveBeenCalledTimes(1)
  })

  it('普通 Viewer 启动时恢复当前文件生命周期', () => {
    render(<App />)

    expect(baseUdbxState.loadCurrentFile).toHaveBeenCalledOnce()
  })

  it('默认入口在有基准配置时只进入基准模式', async () => {
    mockGetBenchmarkConfig.mockResolvedValue({
      runId: 'sampledata-01',
      outputPath: '/tmp/sampledata-01.json',
      temperature: 'cold',
      maxConcurrentQueries: 1,
      scenario: {
        name: 'sampledata-multilayer',
        filePath: '/data/SampleData.udbx',
        layers: ['BaseMap_P'],
        selection: { datasetName: 'BaseMap_P', page: 1, rowIndex: 0 },
        viewportSteps: [{
          bounds: { minX: 115, minY: 38, maxX: 118, maxY: 42 },
          expectedStrategy: 'envelope_cache',
        }],
      },
    })

    render(<ApplicationRoot />)

    await waitFor(() => expect(screen.getByText('基准模式 sampledata-multilayer')).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: '设置' })).not.toBeInTheDocument()
  })

  it('基准配置读取失败时显示错误且不进入普通 Viewer', async () => {
    mockGetBenchmarkConfig.mockRejectedValue(new Error('配置损坏'))

    render(<ApplicationRoot />)

    await waitFor(() => expect(screen.getByText('无法读取基准配置：配置损坏')).toBeInTheDocument())
    expect(screen.queryByRole('button', { name: '设置' })).not.toBeInTheDocument()
  })

  it('只在设置首次加载完成后应用默认属性表展开状态', async () => {
    const { rerender } = render(<App />)

    expect(screen.getByRole('button', { name: '属性表模式 half' })).toBeInTheDocument()

    mockUseViewerSettings.mockReturnValue({
      settings: loadedSettings,
      loading: false,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })
    rerender(<App />)

    await waitFor(() => expect(screen.getByRole('button', { name: '属性表模式 collapsed' })).toBeInTheDocument())

    act(() => {
      capturedAttributeTableDrawerProps?.onModeChange('full')
    })
    await waitFor(() => expect(screen.getByRole('button', { name: '属性表模式 full' })).toBeInTheDocument())
    act(() => {
      capturedAttributeTableDrawerProps?.onModeChange('collapsed')
    })
    await waitFor(() => expect(screen.getByRole('button', { name: '属性表模式 collapsed' })).toBeInTheDocument())

    mockUseViewerSettings.mockReturnValue({
      settings: defaultViewerSettings,
      loading: false,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })
    rerender(<App />)

    await waitFor(() => expect(screen.getByRole('button', { name: '属性表模式 collapsed' })).toBeInTheDocument())
  })

  it('设置默认打开属性表时映射为 half 模式', async () => {
    mockUseViewerSettings.mockReturnValue({
      settings: {
        ...defaultViewerSettings,
        table: { defaultOpen: true },
      },
      loading: false,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })

    render(<App />)

    await waitFor(() => expect(screen.getByRole('button', { name: '属性表模式 half' })).toBeInTheDocument())
  })

  it('设置默认关闭属性表时映射为 collapsed 模式', async () => {
    mockUseViewerSettings.mockReturnValue({
      settings: {
        ...defaultViewerSettings,
        table: { defaultOpen: false },
      },
      loading: false,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })

    render(<App />)

    await waitFor(() => expect(screen.getByRole('button', { name: '属性表模式 collapsed' })).toBeInTheDocument())
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
    expect(capturedMapWorkspaceProps?.onViewportChange).toBe(baseUdbxState.queryViewport)
    expect(capturedInspectorPanelProps?.showPreviewStats).toBe(true)
  })

  it('默认装配半展开属性表并向检查器和属性表传入当前状态', () => {
    const loadTableDataset = vi.fn()
    const setMapLayerVisible = vi.fn()
    const removeMapLayer = vi.fn()
    const selectFeature = vi.fn()
    const settings: ViewerSettings = {
      ...defaultViewerSettings,
      advanced: {
        showPreviewStats: true,
      },
    }

    mockUseViewerSettings.mockReturnValue({
      settings,
      loading: true,
      error: null,
      saveSettings: vi.fn(),
      resetSettings: vi.fn(),
    })
    mockUseUDBX.mockReturnValue({
      ...baseUdbxState,
      mapLayers: mapLayerFixtures,
      pageData: pageDataFixture,
      activeTableDataset: 'BaseMap_P',
      selectedMapFeature: selectedFeatureFixture,
      selectedFeatureAttributes: featureAttributesFixture,
      loadTableDataset,
      setMapLayerVisible,
      removeMapLayer,
      selectFeature,
    })

    render(<App />)

    expect(capturedAttributeTableDrawerProps?.mode).toBe('half')
    expect(capturedInspectorPanelProps?.layers).toBe(mapLayerFixtures)
    expect(capturedInspectorPanelProps?.showPreviewStats).toBe(true)
    expect(capturedInspectorPanelProps?.selectedFeatureAttributes).toBe(featureAttributesFixture)
    expect(capturedMapWorkspaceProps?.selectedFeatureAttributes).toBe(featureAttributesFixture)
    expect(capturedMapWorkspaceProps?.selectionLocationError).toBeNull()
    expect(capturedAttributeTableDrawerProps?.pageData).toBe(pageDataFixture)
    expect(capturedAttributeTableDrawerProps?.datasetName).toBe('BaseMap_P')
    expect(capturedAttributeTableDrawerProps?.selectedFeature).toBe(selectedFeatureFixture)

    capturedInspectorPanelProps?.onLayerVisibleChange('BaseMap_P', false)
    capturedInspectorPanelProps?.onRemoveLayer('BaseMap_P')
    capturedAttributeTableDrawerProps?.onFeatureSelect('BaseMap_P', 2)
    capturedAttributeTableDrawerProps?.onPageChange(2)

    expect(setMapLayerVisible).toHaveBeenCalledWith('BaseMap_P', false)
    expect(removeMapLayer).toHaveBeenCalledWith('BaseMap_P')
    expect(selectFeature).toHaveBeenCalledWith('BaseMap_P', 2)
    expect(loadTableDataset).toHaveBeenCalledWith('BaseMap_P', 2)
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

  it('选择表格数据集时只加载属性表', () => {
    const loadDataset = vi.fn()
    const loadTableDataset = vi.fn()
    mockUseUDBX.mockReturnValue({
      ...baseUdbxState,
      datasets: datasetFixtures,
      loadDataset,
      loadTableDataset,
    })

    render(<App />)

    capturedDatasetExplorerProps?.onSelectDataset('TabularDT')

    expect(loadTableDataset).toHaveBeenCalledWith('TabularDT', 1)
    expect(loadDataset).not.toHaveBeenCalled()
  })

  it('选择未知类型数据集时只尝试加载属性表', () => {
    const loadDataset = vi.fn()
    const loadTableDataset = vi.fn().mockRejectedValue(new Error('暂不支持该数据集类型'))
    mockUseUDBX.mockReturnValue({
      ...baseUdbxState,
      datasets: datasetFixtures,
      loadDataset,
      loadTableDataset,
    })

    render(<App />)

    capturedDatasetExplorerProps?.onSelectDataset('modeldt_Texture')

    expect(loadTableDataset).toHaveBeenCalledWith('modeldt_Texture', 1)
    expect(loadDataset).not.toHaveBeenCalled()
  })

  it('选择已添加到地图的空间数据集时只切换属性表', () => {
    const loadDataset = vi.fn()
    const loadTableDataset = vi.fn()
    mockUseUDBX.mockReturnValue({
      ...baseUdbxState,
      datasets: datasetFixtures,
      mapLayers: mapLayerFixtures,
      loadDataset,
      loadTableDataset,
    })

    render(<App />)

    capturedDatasetExplorerProps?.onSelectDataset('BaseMap_P')

    expect(loadTableDataset).toHaveBeenCalledWith('BaseMap_P', 1)
    expect(loadDataset).not.toHaveBeenCalled()
  })

  it('选择空间数据集时加载地图预览和属性表', () => {
    const loadDataset = vi.fn()
    const loadTableDataset = vi.fn()
    mockUseUDBX.mockReturnValue({
      ...baseUdbxState,
      datasets: datasetFixtures,
      loadDataset,
      loadTableDataset,
    })

    render(<App />)

    capturedDatasetExplorerProps?.onSelectDataset('BaseMap_L')

    expect(loadDataset).toHaveBeenCalledWith('BaseMap_L', 1)
    expect(loadTableDataset).not.toHaveBeenCalled()
  })

  it('选择 CAD 数据集时加载地图预览和属性表', () => {
    const loadDataset = vi.fn()
    const loadTableDataset = vi.fn()
    mockUseUDBX.mockReturnValue({
      ...baseUdbxState,
      datasets: [
        ...datasetFixtures,
        { name: 'CADDT', kind: 'cad', objectCount: 92, iconType: 'cad' },
      ],
      loadDataset,
      loadTableDataset,
    })

    render(<App />)

    capturedDatasetExplorerProps?.onSelectDataset('CADDT')

    expect(loadDataset).toHaveBeenCalledWith('CADDT', 1)
    expect(loadTableDataset).not.toHaveBeenCalled()
  })

  it('属性表分页只重新加载属性表数据', () => {
    const loadDataset = vi.fn()
    const loadTableDataset = vi.fn()
    mockUseUDBX.mockReturnValue({
      ...baseUdbxState,
      activeTableDataset: 'TabularDT',
      loadDataset,
      loadTableDataset,
    })

    render(<App />)

    capturedAttributeTableDrawerProps?.onPageChange(3)

    expect(loadTableDataset).toHaveBeenCalledWith('TabularDT', 3)
    expect(loadDataset).not.toHaveBeenCalled()
  })
})
