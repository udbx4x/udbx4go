import { useState, useCallback } from 'react'
import {
  OpenFileDialog,
  OpenUDBXFile,
  CloseUDBXFile,
  ListDatasets,
  LoadDatasetPage,
  GetCurrentFile,
} from '../../wailsjs/go/main/App'
import type { DatasetInfo, PageData, FileInfo } from '../types'

export function useUDBX() {
  const [currentFile, setCurrentFile] = useState<string | null>(null)
  const [datasets, setDatasets] = useState<DatasetInfo[]>([])
  const [selectedDataset, setSelectedDataset] = useState<string | null>(null)
  const [pageData, setPageData] = useState<PageData | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const openFileDialog = useCallback(async (): Promise<boolean> => {
    try {
      setLoading(true)
      setError(null)

      const path = await OpenFileDialog()
      if (!path) {
        setLoading(false)
        return false
      }

      const fileInfo: FileInfo = await OpenUDBXFile(path)
      setCurrentFile(fileInfo.path)

      const dsList: DatasetInfo[] = await ListDatasets()
      setDatasets(dsList)

      setLoading(false)
      return true
    } catch (err) {
      setError(err instanceof Error ? err.message : '打开文件失败')
      setLoading(false)
      return false
    }
  }, [])

  const closeFile = useCallback(async () => {
    try {
      await CloseUDBXFile()
      setCurrentFile(null)
      setDatasets([])
      setSelectedDataset(null)
      setPageData(null)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : '关闭文件失败')
    }
  }, [])

  const loadDataset = useCallback(async (datasetName: string, page: number = 1) => {
    try {
      setLoading(true)
      setError(null)
      setSelectedDataset(datasetName)

      const data: PageData = await LoadDatasetPage(datasetName, page)
      setPageData(data)

      setLoading(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载数据集失败')
      setLoading(false)
    }
  }, [])

  const loadCurrentFile = useCallback(async () => {
    try {
      const path = await GetCurrentFile()
      if (path) {
        setCurrentFile(path)
        const dsList: DatasetInfo[] = await ListDatasets()
        setDatasets(dsList)
      }
    } catch {
      // Ignore errors when loading current file on startup
    }
  }, [])

  return {
    currentFile,
    datasets,
    selectedDataset,
    pageData,
    loading,
    error,
    openFileDialog,
    closeFile,
    loadDataset,
    loadCurrentFile,
  }
}
