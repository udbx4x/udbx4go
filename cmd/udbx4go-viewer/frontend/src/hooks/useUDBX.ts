import { useCallback, useEffect, useRef, useState } from 'react'
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
import { createDefaultLayerStyle } from '../spatial/layerStyle'
import { isLocatableFeature } from '../spatial/featureLocation'
import {
  ViewportQueryCoordinator,
  type ViewportQueryJob,
} from '../spatial/ViewportQueryCoordinator'
import type {
  BoundingBox,
  DatasetInfo,
  PageData,
  FileInfo,
  SpatialSummary,
  SpatialPreview,
  FeatureAttributes,
  MapLayerState,
  SelectedMapFeature,
} from '../types'

interface UseUDBXOptions {
  spatialPreviewFeatureLimit: number
  spatialPreviewVertexBudget: number
}

export function useUDBX(options: UseUDBXOptions) {
  const [currentFile, setCurrentFile] = useState<string | null>(null)
  const [datasets, setDatasets] = useState<DatasetInfo[]>([])
  const [selectedDataset, setSelectedDataset] = useState<string | null>(null)
  const [activeTableDataset, setActiveTableDataset] = useState<string | null>(null)
  const [pageData, setPageData] = useState<PageData | null>(null)
  const [mapLayers, setMapLayersState] = useState<MapLayerState[]>([])
  const [selectedMapFeature, setSelectedMapFeatureState] = useState<SelectedMapFeature | null>(null)
  const [selectedFeatureAttributes, setSelectedFeatureAttributes] = useState<FeatureAttributes | null>(null)
  const [selectionLocationError, setSelectionLocationError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const mapLayersRef = useRef<MapLayerState[]>([])
  const selectedMapFeatureRef = useRef<SelectedMapFeature | null>(null)
  const fileGenerationRef = useRef(0)
  const optionsRef = useRef(options)

  optionsRef.current = options

  const setMapLayers = useCallback((update: (layers: MapLayerState[]) => MapLayerState[]) => {
    setMapLayersState((layers) => {
      const next = update(layers)
      mapLayersRef.current = next
      return next
    })
  }, [])

  const setSelectedMapFeature = useCallback((selection: SelectedMapFeature | null) => {
    selectedMapFeatureRef.current = selection
    setSelectedMapFeatureState(selection)
  }, [])

  const coordinatorRef = useRef<ViewportQueryCoordinator | null>(null)
  if (!coordinatorRef.current) {
    coordinatorRef.current = new ViewportQueryCoordinator({
      loadPreview: (job) => loadViewportPreview(job, optionsRef.current),
      applyLoading: (datasetName) => {
        setMapLayers((layers) => layers.map((layer) =>
          layer.datasetName === datasetName
            ? { ...layer, queryStatus: 'loading', queryError: null }
            : layer,
        ))
      },
      applyPreview: (datasetName, preview) => {
        setMapLayers((layers) => layers.map((layer) =>
          layer.datasetName === datasetName
            ? {
                ...layer,
                preview,
                queryStatus: preview.degradedReason || preview.strategy === 'bounded_sample' ? 'degraded' : 'ready',
                queryError: null,
                lastQueriedBounds: preview.queriedBounds,
              }
            : layer,
        ))
      },
      applyError: (datasetName, queryError) => {
        setMapLayers((layers) => layers.map((layer) =>
          layer.datasetName === datasetName
            ? { ...layer, queryStatus: 'error', queryError }
            : layer,
        ))
      },
      getFileGeneration: () => fileGenerationRef.current,
      getLayer: (datasetName) => mapLayersRef.current.find((layer) => layer.datasetName === datasetName),
    })
  }

  useEffect(() => () => coordinatorRef.current?.invalidateAll(), [])

  const resetFileState = useCallback(() => {
    coordinatorRef.current?.invalidateAll()
    fileGenerationRef.current += 1
    setSelectedDataset(null)
    setActiveTableDataset(null)
    setPageData(null)
    setMapLayers(() => [])
    setSelectedMapFeature(null)
    setSelectedFeatureAttributes(null)
    setSelectionLocationError(null)
  }, [setMapLayers, setSelectedMapFeature])

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
      resetFileState()
      setCurrentFile(fileInfo.path)
      setDatasets(await ListDatasets())
      setLoading(false)
      return true
    } catch (err) {
      setError(errorMessage(err, '打开文件失败'))
      setLoading(false)
      return false
    }
  }, [resetFileState])

  const closeFile = useCallback(async () => {
    try {
      await CloseUDBXFile()
      resetFileState()
      setCurrentFile(null)
      setDatasets([])
      setError(null)
    } catch (err) {
      setError(errorMessage(err, '关闭文件失败'))
    }
  }, [resetFileState])

  const loadTableDataset = useCallback(async (datasetName: string, page = 1) => {
    try {
      setLoading(true)
      setError(null)
      const data: PageData = await LoadDatasetPage(datasetName, page)
      setSelectedDataset(datasetName)
      setActiveTableDataset(datasetName)
      setPageData(data)
      setLoading(false)
    } catch (err) {
      setError(errorMessage(err, '加载属性表失败'))
      setLoading(false)
      throw err
    }
  }, [])

  const addDatasetToMap = useCallback(async (datasetName: string) => {
    if (mapLayersRef.current.some((layer) => layer.datasetName === datasetName)) {
      return
    }
    setMapLayers((layers) => [...layers, createPendingLayer(datasetName)])

    try {
      const summary: SpatialSummary = await GetDatasetSpatialSummary(datasetName)
      if (!summary.previewSupported) {
        setMapLayers((layers) => layers.map((layer) => layer.datasetName === datasetName
          ? {
              ...layer,
              kind: summary.kind,
              style: createDefaultLayerStyle(summary.kind),
              loading: false,
              error: summary.unsupportedReason || '该数据集不支持空间预览',
              summary,
            }
          : layer,
        ))
        return
      }

      setMapLayers((layers) => layers.map((layer) => layer.datasetName === datasetName
        ? {
            ...layer,
            kind: summary.kind,
            style: createDefaultLayerStyle(summary.kind),
            loading: false,
            error: null,
            summary,
          }
        : layer,
      ))

      if (summary.viewportQuerySupported) {
        return
      }

      const preview = await loadBoundedPreview(datasetName, optionsRef.current)
      setMapLayers((layers) => layers.map((layer) => layer.datasetName === datasetName
        ? {
            ...layer,
            preview,
            queryStatus: preview.degradedReason || preview.strategy === 'bounded_sample' ? 'degraded' : 'ready',
            queryError: null,
          }
        : layer,
      ))
    } catch (err) {
      setMapLayers((layers) => layers.map((layer) => layer.datasetName === datasetName
        ? { ...layer, loading: false, error: errorMessage(err, '加载空间图层失败') }
        : layer,
      ))
    }
  }, [setMapLayers])

  const loadDataset = useCallback(async (datasetName: string, page = 1) => {
    try {
      setLoading(true)
      setError(null)
      setSelectedMapFeature(null)
      setSelectedFeatureAttributes(null)
      setSelectionLocationError(null)
      await loadTableDataset(datasetName, page)
      await addDatasetToMap(datasetName)
      setLoading(false)
    } catch (err) {
      setError(errorMessage(err, '加载数据集失败'))
      setLoading(false)
    }
  }, [addDatasetToMap, loadTableDataset, setSelectedMapFeature])

  const queryViewport = useCallback((viewport: BoundingBox) => {
    const layers = mapLayersRef.current
      .filter((layer) => layer.visible && layer.summary?.viewportQuerySupported)
      .map((layer) => ({
        datasetName: layer.datasetName,
        visible: layer.visible,
        requiredIds: selectedMapFeatureRef.current?.datasetName === layer.datasetName
          ? [selectedMapFeatureRef.current.featureID]
          : [],
      }))
    coordinatorRef.current?.scheduleViewport(viewport, layers, fileGenerationRef.current)
  }, [])

  const setMapLayerVisible = useCallback((datasetName: string, visible: boolean) => {
    if (!visible) {
      coordinatorRef.current?.invalidateLayer(datasetName)
    }
    setMapLayers((layers) => layers.map((layer) =>
      layer.datasetName === datasetName ? { ...layer, visible } : layer,
    ))
  }, [setMapLayers])

  const removeMapLayer = useCallback((datasetName: string) => {
    coordinatorRef.current?.invalidateLayer(datasetName)
    setMapLayers((layers) => layers.filter((layer) => layer.datasetName !== datasetName))
    if (selectedMapFeatureRef.current?.datasetName === datasetName) {
      setSelectedMapFeature(null)
      setSelectedFeatureAttributes(null)
      setSelectionLocationError(null)
    }
  }, [setMapLayers, setSelectedMapFeature])

  const selectFeature = useCallback(async (datasetName: string, featureID: number) => {
    try {
      setError(null)
      setSelectedMapFeature({ datasetName, featureID })
      setSelectionLocationError(null)
      const attributes: FeatureAttributes = await GetFeatureAttributes(datasetName, featureID)
      if (!selectionMatches(selectedMapFeatureRef.current, datasetName, featureID)) {
        return
      }
      if (attributes.datasetName !== datasetName || attributes.id !== featureID) {
        setSelectionLocationError('定位失败')
        return
      }
      setSelectedFeatureAttributes(attributes)
      if (!isLocatableFeature(attributes)) {
        setSelectionLocationError('定位失败')
      }
      if (activeTableDataset !== datasetName) {
        await loadTableDataset(datasetName, 1)
      }
    } catch (err) {
      if (!selectionMatches(selectedMapFeatureRef.current, datasetName, featureID)) {
        return
      }
      setSelectedFeatureAttributes(null)
      setSelectionLocationError('定位失败')
      setError(errorMessage(err, '查询要素属性失败'))
    }
  }, [activeTableDataset, loadTableDataset, setSelectedMapFeature])

  const loadCurrentFile = useCallback(async () => {
    try {
      const path = await GetCurrentFile()
      if (path) {
        resetFileState()
        setCurrentFile(path)
        setDatasets(await ListDatasets())
      }
    } catch {
      // Startup restoration is optional.
    }
  }, [resetFileState])

  return {
    currentFile,
    datasets,
    selectedDataset,
    activeTableDataset,
    pageData,
    mapLayers,
    selectedMapFeature,
    selectedFeatureAttributes,
    selectionLocationError,
    loading,
    error,
    openFileDialog,
    closeFile,
    loadTableDataset,
    addDatasetToMap,
    loadDataset,
    queryViewport,
    setMapLayerVisible,
    removeMapLayer,
    selectFeature,
    loadCurrentFile,
  }
}

function createPendingLayer(datasetName: string): MapLayerState {
  return {
    datasetName,
    kind: 'unknown',
    visible: true,
    style: createDefaultLayerStyle('unknown'),
    loading: true,
    error: null,
    summary: null,
    preview: null,
    queryStatus: 'idle',
    queryError: null,
  }
}

async function loadViewportPreview(job: ViewportQueryJob, options: UseUDBXOptions): Promise<SpatialPreview> {
  return LoadSpatialPreview(job.datasetName, new main.SpatialPreviewRequestDTO({
    viewport: job.bounds,
    limit: options.spatialPreviewFeatureLimit,
    maxVertices: options.spatialPreviewVertexBudget,
    simplify: false,
    requiredIds: job.requiredIds,
  }))
}

async function loadBoundedPreview(datasetName: string, options: UseUDBXOptions): Promise<SpatialPreview> {
  return LoadSpatialPreview(datasetName, new main.SpatialPreviewRequestDTO({
    viewport: undefined,
    limit: options.spatialPreviewFeatureLimit,
    maxVertices: options.spatialPreviewVertexBudget,
    simplify: false,
  }))
}

function errorMessage(error: unknown, fallback: string): string {
  return error instanceof Error ? error.message : fallback
}

function selectionMatches(
  selection: SelectedMapFeature | null,
  datasetName: string,
  featureID: number,
): boolean {
  return selection?.datasetName === datasetName && selection.featureID === featureID
}
