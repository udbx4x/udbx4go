import React, { useEffect, useRef } from 'react'
import { Box, IconButton, Paper, Tooltip, Typography } from '@mui/material'
import { CenterFocusStrong as FitIcon } from '@mui/icons-material'
import { viewerColors } from '../theme/viewerTheme'
import { OpenLayersSpatialRendererAdapter } from '../spatial/OpenLayersSpatialRendererAdapter'
import type { MapLayerState, SelectedMapFeature } from '../types'
import { EmptyState } from './EmptyState'

interface MapWorkspaceProps {
  layers: MapLayerState[]
  selectedFeature: SelectedMapFeature | null
  autoFitOnLayerChange: boolean
  zoomToSelectedFeature: boolean
  onFeatureSelect: (datasetName: string, featureID: number) => void
}

export const MapWorkspace: React.FC<MapWorkspaceProps> = ({
  layers,
  selectedFeature,
  autoFitOnLayerChange,
  zoomToSelectedFeature,
  onFeatureSelect,
}) => {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const adapterRef = useRef<OpenLayersSpatialRendererAdapter | null>(null)
  const renderedLayerNamesRef = useRef<Set<string>>(new Set())
  const layerPreviewReadyRef = useRef<Map<string, boolean>>(new Map())
  const hasSyncedLayersRef = useRef(false)
  const selectedFeatureRef = useRef<SelectedMapFeature | null>(selectedFeature)
  const zoomToSelectedFeatureRef = useRef(zoomToSelectedFeature)
  const featureSelectHandlerRef = useRef(onFeatureSelect)
  const sampledLayerCount = layers.filter((layer) => layer.preview?.sampled).length

  useEffect(() => {
    featureSelectHandlerRef.current = onFeatureSelect
  }, [onFeatureSelect])

  useEffect(() => {
    if (!containerRef.current) {
      return
    }

    const adapter = new OpenLayersSpatialRendererAdapter()
    adapter.mount(containerRef.current)
    adapter.onFeatureClick((datasetName, featureID) => {
      featureSelectHandlerRef.current(datasetName, featureID)
    })
    adapterRef.current = adapter

    return () => {
      adapter.destroy()
      adapterRef.current = null
      renderedLayerNamesRef.current.clear()
      layerPreviewReadyRef.current.clear()
      hasSyncedLayersRef.current = false
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
      if (layer.preview) {
        adapter.setLayer(layer)
        adapter.setLayerVisible(layer.datasetName, layer.visible)
      } else {
        adapter.removeLayer(layer.datasetName)
      }
    })

    renderedLayerNamesRef.current = nextLayerNames
    layerPreviewReadyRef.current = nextPreviewReady
    hasSyncedLayersRef.current = true
    if (autoFitOnLayerChange) {
      adapter.fitAllVisibleLayers()
    }
    if (shouldFitSelectedAfterLayerSync && currentSelectedFeature) {
      adapter.fitFeature(currentSelectedFeature.datasetName, currentSelectedFeature.featureID)
    }
  }, [autoFitOnLayerChange, layers])

  useEffect(() => {
    selectedFeatureRef.current = selectedFeature
    zoomToSelectedFeatureRef.current = zoomToSelectedFeature

    const adapter = adapterRef.current
    if (!adapter) {
      return
    }

    adapter.setSelection(selectedFeature)
    if (selectedFeature && zoomToSelectedFeature) {
      adapter.fitFeature(selectedFeature.datasetName, selectedFeature.featureID)
    }
  }, [selectedFeature, zoomToSelectedFeature])

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
