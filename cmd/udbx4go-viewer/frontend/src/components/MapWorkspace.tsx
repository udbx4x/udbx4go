import React, { useEffect, useRef } from 'react'
import { Box, IconButton, Paper, Tooltip, Typography } from '@mui/material'
import { CenterFocusStrong as FitIcon } from '@mui/icons-material'
import { viewerColors } from '../theme/viewerTheme'
import { OpenLayersSpatialRendererAdapter } from '../spatial/OpenLayersSpatialRendererAdapter'
import { featureGeometryKind, isValidBounds } from '../spatial/featureLocation'
import type { BoundingBox, FeatureAttributes, MapLayerState, SelectedMapFeature } from '../types'
import { EmptyState } from './EmptyState'

interface MapWorkspaceProps {
  layers: MapLayerState[]
  selectedFeature: SelectedMapFeature | null
  selectedFeatureAttributes?: FeatureAttributes | null
  selectionLocationError?: string | null
  autoFitOnLayerChange: boolean
  zoomToSelectedFeature: boolean
  onViewportChange: (viewport: BoundingBox) => void
  onFeatureSelect: (datasetName: string, featureID: number) => void
}

export const MapWorkspace: React.FC<MapWorkspaceProps> = ({
  layers,
  selectedFeature,
  selectedFeatureAttributes,
  selectionLocationError,
  autoFitOnLayerChange,
  zoomToSelectedFeature,
  onViewportChange,
  onFeatureSelect,
}) => {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const adapterRef = useRef<OpenLayersSpatialRendererAdapter | null>(null)
  const renderedLayerNamesRef = useRef<Set<string>>(new Set())
  const appliedLayerStateRef = useRef<Map<string, { preview: MapLayerState['preview']; visible: boolean }>>(new Map())
  const layerPreviewReadyRef = useRef<Map<string, boolean>>(new Map())
  const hasSyncedLayersRef = useRef(false)
  const selectedFeatureRef = useRef<SelectedMapFeature | null>(selectedFeature)
  const zoomToSelectedFeatureRef = useRef(zoomToSelectedFeature)
  const featureSelectHandlerRef = useRef(onFeatureSelect)
  const viewportChangeHandlerRef = useRef(onViewportChange)
  const fittedSummaryLayersRef = useRef<Set<string>>(new Set())
  const viewportBootstrapLayersRef = useRef<Set<string>>(new Set())
  const currentViewportRef = useRef<BoundingBox | null>(null)
  const sampledLayerCount = layers.filter((layer) => layer.preview?.sampled).length

  useEffect(() => {
    featureSelectHandlerRef.current = onFeatureSelect
  }, [onFeatureSelect])

  useEffect(() => {
    viewportChangeHandlerRef.current = onViewportChange
  }, [onViewportChange])

  useEffect(() => {
    if (!containerRef.current) {
      return
    }

    const adapter = new OpenLayersSpatialRendererAdapter()
    const unsubscribeViewport = adapter.onViewportChange((viewport) => {
      currentViewportRef.current = viewport
      viewportChangeHandlerRef.current(viewport)
    })
    adapter.mount(containerRef.current)
    if (!autoFitOnLayerChange) {
      const currentViewport = adapter.getViewport()
      if (currentViewport) {
        currentViewportRef.current = currentViewport
        viewportChangeHandlerRef.current(currentViewport)
      }
    }
    adapter.onFeatureClick((datasetName, featureID) => {
      featureSelectHandlerRef.current(datasetName, featureID)
    })
    adapterRef.current = adapter

    return () => {
      unsubscribeViewport()
      adapter.destroy()
      adapterRef.current = null
      renderedLayerNamesRef.current.clear()
      appliedLayerStateRef.current.clear()
      layerPreviewReadyRef.current.clear()
      hasSyncedLayersRef.current = false
      fittedSummaryLayersRef.current.clear()
      viewportBootstrapLayersRef.current.clear()
      currentViewportRef.current = null
    }
  }, [])

  useEffect(() => {
    const adapter = adapterRef.current
    if (!adapter) {
      return
    }

    const nextLayerNames = new Set(layers.map((layer) => layer.datasetName))
    renderedLayerNamesRef.current.forEach((datasetName) => {
      if (!nextLayerNames.has(datasetName)) {
        adapter.removeLayer(datasetName)
        appliedLayerStateRef.current.delete(datasetName)
        fittedSummaryLayersRef.current.delete(datasetName)
        viewportBootstrapLayersRef.current.delete(datasetName)
      }
    })

    const previousPreviewReady = layerPreviewReadyRef.current
    const nextPreviewReady = new Map<string, boolean>()
    const currentSelectedFeature = selectedFeatureRef.current
    const shouldZoomToSelectedFeature = zoomToSelectedFeatureRef.current
    const selectedLayer = currentSelectedFeature
      ? layers.find((layer) => layer.datasetName === currentSelectedFeature.datasetName)
      : undefined
    const shouldFitSelectedAfterLayerSync =
      Boolean(
        hasSyncedLayersRef.current &&
          currentSelectedFeature &&
          shouldZoomToSelectedFeature &&
          selectedLayer?.preview &&
          previousPreviewReady.get(currentSelectedFeature.datasetName) !== true,
      )

    layers.forEach((layer) => {
      nextPreviewReady.set(layer.datasetName, Boolean(layer.preview))
      const applied = appliedLayerStateRef.current.get(layer.datasetName)
      if (layer.preview) {
        if (applied?.preview !== layer.preview) {
          adapter.setLayer(layer)
        } else if (applied.visible !== layer.visible) {
          adapter.setLayerVisible(layer.datasetName, layer.visible)
        }
        appliedLayerStateRef.current.set(layer.datasetName, {
          preview: layer.preview,
          visible: layer.visible,
        })
      } else {
        if (applied?.preview) {
          adapter.removeLayer(layer.datasetName)
        }
        appliedLayerStateRef.current.set(layer.datasetName, {
          preview: null,
          visible: layer.visible,
        })
      }
    })

    renderedLayerNamesRef.current = nextLayerNames
    layerPreviewReadyRef.current = nextPreviewReady
    hasSyncedLayersRef.current = true
    const newViewportLayers = layers.filter((layer) =>
      layer.visible &&
      layer.summary?.viewportQuerySupported &&
      !viewportBootstrapLayersRef.current.has(layer.datasetName),
    )
    newViewportLayers.forEach((layer) => viewportBootstrapLayersRef.current.add(layer.datasetName))
    if (!autoFitOnLayerChange && newViewportLayers.length > 0 && !currentViewportRef.current) {
      const currentViewport = adapter.getViewport()
      if (currentViewport) {
        currentViewportRef.current = currentViewport
        viewportChangeHandlerRef.current(currentViewport)
      }
    }
    const layerToFit = layers.find((layer) =>
      layer.visible &&
      layer.summary?.viewportQuerySupported &&
      layer.summary.extent &&
      !fittedSummaryLayersRef.current.has(layer.datasetName),
    )
    if (autoFitOnLayerChange && layerToFit?.summary?.extent) {
      fittedSummaryLayersRef.current.add(layerToFit.datasetName)
      adapter.fitBounds(layerToFit.summary.extent, geometryKindFromDatasetKind(layerToFit.kind))
    } else if (
      autoFitOnLayerChange &&
      layers.some((layer) =>
        !layer.summary?.viewportQuerySupported &&
        Boolean(layer.preview) &&
        previousPreviewReady.get(layer.datasetName) !== true,
      )
    ) {
      adapter.fitAllVisibleLayers()
    }
    if (shouldFitSelectedAfterLayerSync && currentSelectedFeature && selectedFeatureAttributes === undefined) {
      adapter.fitFeature(currentSelectedFeature.datasetName, currentSelectedFeature.featureID)
    }
    if (currentSelectedFeature) {
      adapter.setSelection(currentSelectedFeature)
    }
  }, [autoFitOnLayerChange, layers, selectedFeatureAttributes])

  useEffect(() => {
    selectedFeatureRef.current = selectedFeature
    zoomToSelectedFeatureRef.current = zoomToSelectedFeature

    const adapter = adapterRef.current
    if (!adapter) {
      return
    }

    adapter.setSelection(selectedFeature)
    if (selectedFeature && zoomToSelectedFeature && selectedFeatureAttributes === undefined) {
      adapter.fitFeature(selectedFeature.datasetName, selectedFeature.featureID)
    }
  }, [selectedFeature, selectedFeatureAttributes, zoomToSelectedFeature])

  useEffect(() => {
    const adapter = adapterRef.current
    if (!adapter || !selectedFeature || !selectedFeatureAttributes || !zoomToSelectedFeature) {
      return
    }
    if (
      selectedFeature.datasetName !== selectedFeatureAttributes.datasetName ||
      selectedFeature.featureID !== selectedFeatureAttributes.id
    ) {
      return
    }
    const geometryKind = featureGeometryKind(selectedFeatureAttributes.geometryType)
    if (!geometryKind || !isValidBounds(selectedFeatureAttributes.bbox)) {
      return
    }
    adapter.fitBounds(selectedFeatureAttributes.bbox, geometryKind)
  }, [selectedFeature, selectedFeatureAttributes, zoomToSelectedFeature])

  return (
    <Paper elevation={0} sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Box sx={{ flex: 1, minHeight: 0, position: 'relative', bgcolor: viewerColors.mapBg }}>
        <Box ref={containerRef} sx={{ position: 'absolute', inset: 0 }} />
        {layers.length === 0 && (
          <Box sx={{ position: 'absolute', inset: 0 }}>
            <EmptyState title="从左侧选择空间数据集加入地图" />
          </Box>
        )}
        {sampledLayerCount > 0 && (
          <Box
            sx={{
              position: 'absolute',
              left: 12,
              bottom: 12,
              px: 1.25,
              py: 0.75,
              bgcolor: 'background.paper',
              border: 1,
              borderColor: 'warning.light',
              boxShadow: 1,
              borderRadius: 1,
              pointerEvents: 'none',
            }}
          >
            <Typography variant="caption" color="warning.dark">
              部分图层为采样预览
            </Typography>
          </Box>
        )}
        {selectionLocationError && (
          <Box
            sx={{
              position: 'absolute',
              left: 12,
              top: 12,
              px: 1.25,
              py: 0.75,
              bgcolor: 'background.paper',
              border: 1,
              borderColor: 'error.light',
              boxShadow: 1,
              borderRadius: 1,
              pointerEvents: 'none',
            }}
          >
            <Typography variant="caption" color="error.main">
              {selectionLocationError}
            </Typography>
          </Box>
        )}
        <Box sx={{ position: 'absolute', top: 12, right: 12 }}>
          <Tooltip title="适配全部可见图层">
            <span>
              <IconButton
                aria-label="适配全部可见图层"
                size="small"
                disabled={layers.length === 0}
                onClick={() => adapterRef.current?.fitAllVisibleLayers()}
                sx={{
                  bgcolor: 'background.paper',
                  border: 1,
                  borderColor: 'divider',
                  boxShadow: 1,
                  '&:hover': {
                    bgcolor: 'background.paper',
                  },
                }}
              >
                <FitIcon fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
        </Box>
      </Box>
    </Paper>
  )
}

function geometryKindFromDatasetKind(kind: string): 'point' | 'line' | 'polygon' {
  if (kind === 'line' || kind === 'lineZ') {
    return 'line'
  }
  if (kind === 'region' || kind === 'regionZ') {
    return 'polygon'
  }
  return 'point'
}
