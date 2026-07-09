import { useCallback, useEffect, useState } from 'react'
import {
  GetViewerSettings,
  SaveViewerSettings,
  ResetViewerSettings,
} from '../../wailsjs/go/main/App'
import { main } from '../../wailsjs/go/models'
import type { ViewerSettings } from '../settings/viewerSettings'
import { defaultViewerSettings } from '../settings/viewerSettings'

export function useViewerSettings() {
  const [settings, setSettings] = useState<ViewerSettings>(defaultViewerSettings)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const loadSettings = useCallback(async () => {
    try {
      setLoading(true)
      setError(null)
      const loaded = await GetViewerSettings()
      setSettings(loaded as ViewerSettings)
    } catch (err) {
      setSettings(defaultViewerSettings)
      setError(err instanceof Error ? err.message : '设置读取失败，已使用默认设置')
    } finally {
      setLoading(false)
    }
  }, [])

  const saveSettings = useCallback(async (nextSettings: ViewerSettings) => {
    try {
      setError(null)
      const saved = await SaveViewerSettings(new main.ViewerSettingsDTO(nextSettings))
      setSettings(saved as ViewerSettings)
      return saved as ViewerSettings
    } catch (err) {
      setError(`设置保存失败：${err instanceof Error ? err.message : '未知错误'}`)
      throw err
    }
  }, [])

  const resetSettings = useCallback(async () => {
    try {
      setError(null)
      const reset = await ResetViewerSettings()
      setSettings(reset as ViewerSettings)
      return reset as ViewerSettings
    } catch (err) {
      setError(`设置重置失败：${err instanceof Error ? err.message : '未知错误'}`)
      throw err
    }
  }, [])

  useEffect(() => {
    loadSettings()
  }, [loadSettings])

  return {
    settings,
    loading,
    error,
    loadSettings,
    saveSettings,
    resetSettings,
  }
}
