import { useState, useCallback } from 'react'
import {
  OpenFileDialog,
  OpenUDBXFile,
  CloseUDBXFile,
  ListDatasets,
  LoadDatasetPage,
  GetCurrentFile,
  GetDatasetSpatialSummary,
  LoadSpatialPreview,
  GetFeatureAttributes,
} from '../../wailsjs/go/main/App'
import { main } from '../../wailsjs/go/models'
import type {
  DatasetInfo,
  PageData,
  FileInfo,
  SpatialSummary,
  SpatialPreview,
  FeatureAttributes,
  LayerStyle,
  MapLayerState,
  SelectedMapFeature,
} from '../types'

function createDefaultLayerStyle(kind: string): LayerStyle {
  const selected = {
    color: '#d9480f',
    pointRadius: 6,
    strokeWidth: 3,
    fillColor: 'rgba(217,72,15,0.24)',
  }

  switch (kind) {
    case 'line':
    case 'lineZ':
      return {
        point: { radius: 4, fillColor: '#2b8a3e', strokeColor: '#ffffff', strokeWidth: 1 },
        line: { strokeColor: '#2b8a3e', strokeWidth: 1.8 },
        polygon: { fillColor: 'rgba(43,138,62,0.16)', strokeColor: '#2b8a3e', strokeWidth: 1.5 },
        selected,
      }
    case 'region':
    case 'regionZ':
      return {
        point: { radius: 4, fillColor: '#f08c00', strokeColor: '#ffffff', strokeWidth: 1 },
        line: { strokeColor: '#f08c00', strokeWidth: 1.5 },
        polygon: { fillColor: 'rgba(240,140,0,0.18)', strokeColor: '#f08c00', strokeWidth: 1.4 },
        selected,
      }
    default:
      return {
        point: { radius: 4, fillColor: '#1971c2', strokeColor: '#ffffff', strokeWidth: 1 },
        line: { strokeColor: '#1971c2', strokeWidth: 1.5 },
        polygon: { fillColor: 'rgba(25,113,194,0.16)', strokeColor: '#1971c2', strokeWidth: 1.5 },
        selected,
      }
  }
}

export function useUDBX() {
  const [currentFile, setCurrentFile] = useState<string | null>(null)
  const [datasets, setDatasets] = useState<DatasetInfo[]>([])
  const [selectedDataset, setSelectedDataset] = useState<string | null>(null)
  const [activeTableDataset, setActiveTableDataset] = useState<string | null>(null)
  const [pageData, setPageData] = useState<PageData | null>(null)
  const [mapLayers, setMapLayers] = useState<MapLayerState[]>([])
  const [selectedMapFeature, setSelectedMapFeature] = useState<SelectedMapFeature | null>(null)
  const [selectedFeatureAttributes, setSelectedFeatureAttributes] = useState<FeatureAttributes | null>(null)
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
      setSelectedDataset(null)
      setActiveTableDataset(null)
      setPageData(null)
      setMapLayers([])
      setSelectedMapFeature(null)
      setSelectedFeatureAttributes(null)

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
      setActiveTableDataset(null)
      setPageData(null)
      setMapLayers([])
      setSelectedMapFeature(null)
      setSelectedFeatureAttributes(null)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : '关闭文件失败')
    }
  }, [])

  const loadTableDataset = useCallback(async (datasetName: string, page: number = 1) => {
    const data: PageData = await LoadDatasetPage(datasetName, page)
    setSelectedDataset(datasetName)
    setActiveTableDataset(datasetName)
    setPageData(data)
  }, [])

  const addDatasetToMap = useCallback(async (datasetName: string) => {
    setMapLayers((layers) => {
      if (layers.some((layer) => layer.datasetName === datasetName)) {
        return layers
      }
      return [
        ...layers,
        {
          datasetName,
          kind: 'unknown',
          visible: true,
          style: createDefaultLayerStyle('unknown'),
          loading: true,
          error: null,
          summary: null,
          preview: null,
        },
      ]
    })

    try {
      const summary: SpatialSummary = await GetDatasetSpatialSummary(datasetName)
      if (!summary.previewSupported) {
        setMapLayers((layers) =>
          layers.map((layer) =>
            layer.datasetName === datasetName
              ? {
                  ...layer,
                  kind: summary.kind,
                  style: createDefaultLayerStyle(summary.kind),
                  loading: false,
                  error: summary.unsupportedReason || '该数据集不支持空间预览',
                  summary,
                  preview: null,
                }
              : layer,
          ),
        )
        return
      }

      const previewRequest = new main.SpatialPreviewRequestDTO({
        viewport: undefined,
        limit: 1000,
        maxVertices: 50000,
        simplify: false,
      })
      const preview: SpatialPreview = await LoadSpatialPreview(datasetName, previewRequest)

      setMapLayers((layers) =>
        layers.map((layer) =>
          layer.datasetName === datasetName
            ? {
                ...layer,
                kind: summary.kind,
                style: createDefaultLayerStyle(summary.kind),
                loading: false,
                error: null,
                summary,
                preview,
              }
            : layer,
        ),
      )
    } catch (err) {
      setMapLayers((layers) =>
        layers.map((layer) =>
          layer.datasetName === datasetName
            ? { ...layer, loading: false, error: err instanceof Error ? err.message : '加载空间图层失败' }
            : layer,
        ),
      )
    }
  }, [])

  const loadDataset = useCallback(async (datasetName: string, page: number = 1) => {
    try {
      setLoading(true)
      setError(null)
      setSelectedMapFeature(null)
      setSelectedFeatureAttributes(null)

      await loadTableDataset(datasetName, page)
      await addDatasetToMap(datasetName)

      setLoading(false)
    } catch (err) {
      setError(err instanceof Error ? err.message : '加载数据集失败')
      setLoading(false)
    }
  }, [addDatasetToMap, loadTableDataset])

  const setMapLayerVisible = useCallback((datasetName: string, visible: boolean) => {
    setMapLayers((layers) =>
      layers.map((layer) => (layer.datasetName === datasetName ? { ...layer, visible } : layer)),
    )
  }, [])

  const removeMapLayer = useCallback((datasetName: string) => {
    setMapLayers((layers) => layers.filter((layer) => layer.datasetName !== datasetName))
    setSelectedMapFeature((selection) => (selection?.datasetName === datasetName ? null : selection))
    setSelectedFeatureAttributes((attributes) => (attributes?.datasetName === datasetName ? null : attributes))
  }, [])

  const selectFeature = useCallback(async (datasetName: string, featureID: number) => {
    try {
      setError(null)
      setSelectedMapFeature({ datasetName, featureID })
      const attributes: FeatureAttributes = await GetFeatureAttributes(datasetName, featureID)
      setSelectedFeatureAttributes(attributes)
      if (activeTableDataset !== datasetName) {
        await loadTableDataset(datasetName, 1)
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : '查询要素属性失败')
    }
  }, [activeTableDataset, loadTableDataset])

  const loadCurrentFile = useCallback(async () => {
    try {
      const path = await GetCurrentFile()
      if (path) {
        setCurrentFile(path)
        const dsList: DatasetInfo[] = await ListDatasets()
        setDatasets(dsList)
        setSelectedDataset(null)
        setActiveTableDataset(null)
        setPageData(null)
        setMapLayers([])
        setSelectedMapFeature(null)
        setSelectedFeatureAttributes(null)
      }
    } catch {
      // Ignore errors when loading current file on startup
    }
  }, [])

  return {
    currentFile,
    datasets,
    selectedDataset,
    activeTableDataset,
    pageData,
    mapLayers,
    selectedMapFeature,
    selectedFeatureAttributes,
    loading,
    error,
    openFileDialog,
    closeFile,
    loadTableDataset,
    addDatasetToMap,
    loadDataset,
    setMapLayerVisible,
    removeMapLayer,
    selectFeature,
    loadCurrentFile,
  }
}
