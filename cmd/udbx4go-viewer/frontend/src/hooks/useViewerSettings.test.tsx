import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import {
  GetViewerSettings,
  SaveViewerSettings,
  ResetViewerSettings,
} from '../../wailsjs/go/main/App'
import type { ViewerSettings } from '../settings/viewerSettings'
import { defaultViewerSettings } from '../settings/viewerSettings'
import { useViewerSettings } from './useViewerSettings'

vi.mock('../../wailsjs/go/main/App', () => ({
  GetViewerSettings: vi.fn(),
  SaveViewerSettings: vi.fn(),
  ResetViewerSettings: vi.fn(),
}))

vi.mock('../../wailsjs/go/models', () => ({
  main: {
    ViewerSettingsDTO: class ViewerSettingsDTO {
      constructor(source: ViewerSettings) {
        Object.assign(this, source)
      }
    },
  },
}))

const mockGetViewerSettings = vi.mocked(GetViewerSettings)
const mockSaveViewerSettings = vi.mocked(SaveViewerSettings)
const mockResetViewerSettings = vi.mocked(ResetViewerSettings)

type ViewerSettingsDTO = Awaited<ReturnType<typeof GetViewerSettings>>

const loadedSettings: ViewerSettings = {
  spatialPreview: {
    featureLimit: 2500,
    vertexBudget: 2000000,
    autoFitOnLayerChange: false,
  },
  mapInteraction: {
    zoomToSelectedFeature: false,
  },
  table: {
    defaultOpen: false,
  },
  advanced: {
    showPreviewStats: true,
  },
}

const savedSettings: ViewerSettings = {
  ...loadedSettings,
  spatialPreview: {
    ...loadedSettings.spatialPreview,
    featureLimit: 3000,
  },
}

function ViewerSettingsHarness() {
  const { settings, loading, error, saveSettings, resetSettings } = useViewerSettings()
  const [actionError, setActionError] = useState<string | null>(null)

  async function handleSave() {
    try {
      await saveSettings(savedSettings)
      setActionError(null)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : typeof err === 'string' ? err : '保存调用失败')
    }
  }

  async function handleReset() {
    try {
      await resetSettings()
      setActionError(null)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : typeof err === 'string' ? err : '重置调用失败')
    }
  }

  return (
    <div>
      <span data-testid="loading">{String(loading)}</span>
      <span data-testid="error">{error ?? ''}</span>
      <span data-testid="action-error">{actionError ?? ''}</span>
      <span data-testid="feature-limit">{settings.spatialPreview.featureLimit}</span>
      <span data-testid="show-preview-stats">{String(settings.advanced.showPreviewStats)}</span>
      <button type="button" onClick={handleSave}>
        保存
      </button>
      <button type="button" onClick={handleReset}>
        重置
      </button>
    </div>
  )
}

describe('useViewerSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetViewerSettings.mockResolvedValue(loadedSettings as ViewerSettingsDTO)
    mockSaveViewerSettings.mockResolvedValue(savedSettings as ViewerSettingsDTO)
    mockResetViewerSettings.mockResolvedValue(defaultViewerSettings as ViewerSettingsDTO)
  })

  it('loads viewer settings successfully', async () => {
    render(<ViewerSettingsHarness />)

    await waitFor(() => expect(screen.getByTestId('loading')).toHaveTextContent('false'))

    expect(screen.getByTestId('feature-limit')).toHaveTextContent('2500')
    expect(screen.getByTestId('show-preview-stats')).toHaveTextContent('true')
    expect(screen.getByTestId('error')).toHaveTextContent('')
  })

  it('falls back to defaults when loading fails', async () => {
    mockGetViewerSettings.mockRejectedValueOnce(new Error('读取后端设置失败'))

    render(<ViewerSettingsHarness />)

    await waitFor(() => expect(screen.getByTestId('loading')).toHaveTextContent('false'))

    expect(screen.getByTestId('feature-limit')).toHaveTextContent('1000')
    expect(screen.getByTestId('show-preview-stats')).toHaveTextContent('false')
    expect(screen.getByTestId('error')).toHaveTextContent('读取后端设置失败')
  })

  it('clears an existing error when saving succeeds', async () => {
    mockGetViewerSettings.mockRejectedValueOnce(new Error('旧错误'))

    render(<ViewerSettingsHarness />)

    await waitFor(() => expect(screen.getByTestId('error')).toHaveTextContent('旧错误'))

    fireEvent.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => expect(screen.getByTestId('feature-limit')).toHaveTextContent('3000'))
    expect(screen.getByTestId('error')).toBeEmptyDOMElement()
    expect(screen.getByTestId('action-error')).toBeEmptyDOMElement()
  })

  it('sets an error and rethrows when saving fails', async () => {
    mockSaveViewerSettings.mockRejectedValueOnce(new Error('磁盘不可写'))

    render(<ViewerSettingsHarness />)

    await waitFor(() => expect(screen.getByTestId('loading')).toHaveTextContent('false'))

    fireEvent.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => expect(screen.getByTestId('error')).toHaveTextContent('设置保存失败：磁盘不可写'))
    expect(screen.getByTestId('action-error')).toHaveTextContent('磁盘不可写')
    expect(screen.getByTestId('feature-limit')).toHaveTextContent('2500')
  })

  it('clears an existing error and restores defaults when reset succeeds', async () => {
    mockGetViewerSettings.mockRejectedValueOnce(new Error('旧错误'))

    render(<ViewerSettingsHarness />)

    await waitFor(() => expect(screen.getByTestId('error')).toHaveTextContent('旧错误'))

    fireEvent.click(screen.getByRole('button', { name: '重置' }))

    await waitFor(() => expect(screen.getByTestId('feature-limit')).toHaveTextContent('1000'))
    expect(screen.getByTestId('show-preview-stats')).toHaveTextContent('false')
    expect(screen.getByTestId('error')).toBeEmptyDOMElement()
    expect(screen.getByTestId('action-error')).toBeEmptyDOMElement()
  })

  it('sets an error and rethrows when reset fails', async () => {
    mockResetViewerSettings.mockRejectedValueOnce('后端重置失败')

    render(<ViewerSettingsHarness />)

    await waitFor(() => expect(screen.getByTestId('loading')).toHaveTextContent('false'))

    fireEvent.click(screen.getByRole('button', { name: '重置' }))

    await waitFor(() => expect(screen.getByTestId('error')).toHaveTextContent('设置重置失败：后端重置失败'))
    expect(screen.getByTestId('action-error')).toHaveTextContent('后端重置失败')
    expect(screen.getByTestId('feature-limit')).toHaveTextContent('2500')
  })
})
