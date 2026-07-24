import { useCallback, useEffect, useRef, useState } from 'react'
import {
  OpenFileDialog,
  OpenUDBXFile,
  CloseUDBXFile,
  ListDatasets,
  LoadDatasetPage,
  GetCurrentFileInfo,
  GetDatasetSpatialSummary,
  LoadSpatialPreview,
  GetFeatureAttributes,
} from '../../wailsjs/go/main/App'
import { main } from '../../wailsjs/go/models'
import { createDefaultLayerStyle } from '../spatial/layerStyle'
import { isLocatableFeature, isValidBounds } from '../spatial/featureLocation'
import {
  isDegradedSpatialPreview,
  isSpatialPreviewDegradedReason,
} from '../spatial/spatialPreviewDegradation'
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

type TableDatasetCommitGuard = () => boolean

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
  const selectionRequestTokenRef = useRef(0)
  const tableRequestIDRef = useRef(0)
  const activeTableRequestIDRef = useRef<number | null>(null)
  const fileGenerationRef = useRef(0)
  const lastViewportRef = useRef<BoundingBox | null>(null)
  const layerLoadTokensRef = useRef(new Map<string, symbol>())
  const optionsRef = useRef(options)

  optionsRef.current = options

  const setMapLayers = useCallback((update: (layers: MapLayerState[]) => MapLayerState[]) => {
    const next = update(mapLayersRef.current)
    mapLayersRef.current = next
    setMapLayersState(next)
  }, [])

  const setSelectedMapFeature = useCallback((selection: SelectedMapFeature | null) => {
    selectionRequestTokenRef.current += 1
    selectedMapFeatureRef.current = selection
    setSelectedMapFeatureState(selection)
    setSelectedFeatureAttributes(null)
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
        const nextPreview = withViewportFeatureCount(preview)
        setMapLayers((layers) => layers.map((layer) =>
          layer.datasetName === datasetName
            ? {
                ...layer,
                preview: nextPreview,
                queryStatus: isDegradedSpatialPreview(nextPreview) ? 'degraded' : 'ready',
                queryError: null,
                lastQueriedBounds: nextPreview.queriedBounds,
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

  const scheduleViewport = useCallback((viewport: BoundingBox) => {
    if (!isValidBounds(viewport)) {
      return
    }
    lastViewportRef.current = viewport
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

  const resetFileState = useCallback((fileGeneration?: number) => {
    coordinatorRef.current?.invalidateAll()
    layerLoadTokensRef.current.clear()
    fileGenerationRef.current = fileGeneration ?? fileGenerationRef.current + 1
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
      resetFileState(fileInfo.fileGeneration)
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

  const loadTableDataset = useCallback(async (
    datasetName: string,
    page = 1,
    shouldCommit: TableDatasetCommitGuard = () => true,
  ) => {
    const requestID = tableRequestIDRef.current + 1
    tableRequestIDRef.current = requestID
    activeTableRequestIDRef.current = requestID
    try {
      setLoading(true)
      setError(null)
      const data: PageData = await LoadDatasetPage(datasetName, page)
      if (!shouldCommit() || activeTableRequestIDRef.current !== requestID) {
        return
      }
      setSelectedDataset(datasetName)
      setActiveTableDataset(datasetName)
      setPageData(data)
    } catch (err) {
      if (!shouldCommit() || activeTableRequestIDRef.current !== requestID) {
        return
      }
      setError(errorMessage(err, '加载属性表失败'))
      throw err
    } finally {
      if (activeTableRequestIDRef.current === requestID) {
        activeTableRequestIDRef.current = null
        setLoading(false)
      }
    }
  }, [])

  const addDatasetToMap = useCallback(async (datasetName: string) => {
    if (mapLayersRef.current.some((layer) => layer.datasetName === datasetName)) {
      return
    }
    const loadToken = Symbol(datasetName)
    layerLoadTokensRef.current.set(datasetName, loadToken)
    setMapLayers((layers) => [...layers, createPendingLayer(datasetName)])

    try {
      const summary: SpatialSummary = await GetDatasetSpatialSummary(datasetName)
      if (!isCurrentLayerLoad(layerLoadTokensRef.current, datasetName, loadToken)) {
        return
      }
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
        if (lastViewportRef.current) {
          scheduleViewport(lastViewportRef.current)
        }
        return
      }

      const preview = withSummaryDegradation(
        await loadBoundedPreview(datasetName, optionsRef.current),
        summary,
      )
      if (!isCurrentLayerLoad(layerLoadTokensRef.current, datasetName, loadToken)) {
        return
      }
      setMapLayers((layers) => layers.map((layer) => layer.datasetName === datasetName
        ? {
            ...layer,
            preview,
            queryStatus: stableQueryStatus(preview),
            queryError: null,
          }
        : layer,
      ))
    } catch (err) {
      if (!isCurrentLayerLoad(layerLoadTokensRef.current, datasetName, loadToken)) {
        return
      }
      setMapLayers((layers) => layers.map((layer) => layer.datasetName === datasetName
        ? { ...layer, loading: false, error: errorMessage(err, '加载空间图层失败') }
        : layer,
      ))
    }
  }, [scheduleViewport, setMapLayers])

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
    scheduleViewport(viewport)
  }, [scheduleViewport])

  const setMapLayerVisible = useCallback((datasetName: string, visible: boolean) => {
    if (!visible) {
      coordinatorRef.current?.invalidateLayer(datasetName)
    }
    setMapLayers((layers) => layers.map((layer) =>
      layer.datasetName === datasetName
        ? {
            ...layer,
            visible,
            queryStatus: !visible && layer.queryStatus === 'loading'
              ? stableQueryStatus(layer.preview)
              : layer.queryStatus,
            queryError: !visible && layer.queryStatus === 'loading' ? null : layer.queryError,
          }
        : layer,
    ))
    if (visible && lastViewportRef.current) {
      scheduleViewport(lastViewportRef.current)
    }
  }, [scheduleViewport, setMapLayers])

  const removeMapLayer = useCallback((datasetName: string) => {
    coordinatorRef.current?.invalidateLayer(datasetName)
    layerLoadTokensRef.current.delete(datasetName)
    setMapLayers((layers) => layers.filter((layer) => layer.datasetName !== datasetName))
    if (selectedMapFeatureRef.current?.datasetName === datasetName) {
      setSelectedMapFeature(null)
      setSelectedFeatureAttributes(null)
      setSelectionLocationError(null)
    }
  }, [setMapLayers, setSelectedMapFeature])

  const selectFeature = useCallback(async (datasetName: string, featureID: number) => {
    setSelectedMapFeature({ datasetName, featureID })
    const requestToken = selectionRequestTokenRef.current
    setSelectionLocationError(null)
    try {
      setError(null)
      const attributes: FeatureAttributes = await GetFeatureAttributes(datasetName, featureID)
      if (!selectionRequestMatches(requestToken, selectionRequestTokenRef.current, selectedMapFeatureRef.current, datasetName, featureID)) {
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
        await loadTableDataset(datasetName, 1, () => selectionRequestMatches(
          requestToken,
          selectionRequestTokenRef.current,
          selectedMapFeatureRef.current,
          datasetName,
          featureID,
        ))
      }
    } catch (err) {
      if (!selectionRequestMatches(requestToken, selectionRequestTokenRef.current, selectedMapFeatureRef.current, datasetName, featureID)) {
        return
      }
      setSelectedFeatureAttributes(null)
      setSelectionLocationError('定位失败')
      setError(errorMessage(err, '查询要素属性失败'))
    }
  }, [activeTableDataset, loadTableDataset, setSelectedMapFeature])

  const loadCurrentFile = useCallback(async () => {
    try {
      const fileInfo: FileInfo | null = await GetCurrentFileInfo()
      if (fileInfo) {
        resetFileState(fileInfo.fileGeneration)
        setCurrentFile(fileInfo.path)
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
  if (error instanceof Error) {
    return error.message
  }
  if (typeof error === 'string' && error.trim()) {
    return error
  }
  return fallback
}

function selectionMatches(
  selection: SelectedMapFeature | null,
  datasetName: string,
  featureID: number,
): boolean {
  return selection?.datasetName === datasetName && selection.featureID === featureID
}

function selectionRequestMatches(
  requestToken: number,
  currentToken: number,
  selection: SelectedMapFeature | null,
  datasetName: string,
  featureID: number,
): boolean {
  return requestToken === currentToken && selectionMatches(selection, datasetName, featureID)
}

function stableQueryStatus(preview: SpatialPreview | null): MapLayerState['queryStatus'] {
  if (!preview) {
    return 'idle'
  }
  return isDegradedSpatialPreview(preview) ? 'degraded' : 'ready'
}

function withSummaryDegradation(
  preview: SpatialPreview,
  summary: SpatialSummary,
): SpatialPreview {
  if (
    preview.strategy !== 'bounded_sample' ||
    preview.queriedBounds === undefined ||
    preview.degradedReason !== undefined ||
    !isSpatialPreviewDegradedReason(summary.queryDiagnosticReason)
  ) {
    return preview
  }
  return { ...preview, degradedReason: summary.queryDiagnosticReason }
}

function isCurrentLayerLoad(
  tokens: Map<string, symbol>,
  datasetName: string,
  token: symbol,
): boolean {
  return tokens.get(datasetName) === token
}

function withViewportFeatureCount(preview: SpatialPreview): SpatialPreview {
  if (!preview.queriedBounds) {
    return preview
  }
  return {
    ...preview,
    viewportFeatureCount: preview.features.filter((feature) =>
      isValidBounds(feature.bbox) && boundsIntersect(feature.bbox, preview.queriedBounds!),
    ).length,
  }
}

function boundsIntersect(feature: BoundingBox, query: BoundingBox): boolean {
  return feature.maxX >= query.minX &&
    feature.minX <= query.maxX &&
    feature.maxY >= query.minY &&
    feature.minY <= query.maxY
}
