import React, { useEffect, useRef } from 'react'
import { Alert, Box, Paper, Stack, Typography } from '@mui/material'
import { OpenLayersSpatialRendererAdapter } from '../spatial/OpenLayersSpatialRendererAdapter'
import { MapLayerList } from './MapLayerList'
import type { FeatureAttributes, MapLayerState, SelectedMapFeature } from '../types'

interface SpatialPreviewPanelProps {
  layers: MapLayerState[]
  selectedFeature: SelectedMapFeature | null
  selectedFeatureAttributes: FeatureAttributes | null
  onFeatureSelect: (datasetName: string, featureID: number) => void
  onLayerVisibleChange: (datasetName: string, visible: boolean) => void
  onRemoveLayer: (datasetName: string) => void
}

export const SpatialPreviewPanel: React.FC<SpatialPreviewPanelProps> = ({
  layers,
  selectedFeature,
  selectedFeatureAttributes,
  onFeatureSelect,
  onLayerVisibleChange,
  onRemoveLayer,
}) => {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const adapterRef = useRef<OpenLayersSpatialRendererAdapter | null>(null)

  useEffect(() => {
    if (!containerRef.current) {
      return
    }

    const adapter = new OpenLayersSpatialRendererAdapter()
    adapter.mount(containerRef.current)
    adapter.onFeatureClick(onFeatureSelect)
    adapterRef.current = adapter

    return () => {
      adapter.destroy()
      adapterRef.current = null
    }
  }, [onFeatureSelect])

  useEffect(() => {
    const adapter = adapterRef.current
    if (!adapter) {
      return
    }

    layers.forEach((layer) => {
      if (layer.preview) {
        adapter.setLayer(layer)
        adapter.setLayerVisible(layer.datasetName, layer.visible)
      } else {
        adapter.removeLayer(layer.datasetName)
      }
    })
    adapter.fitAllVisibleLayers()
  }, [layers])

  useEffect(() => {
    adapterRef.current?.setSelection(selectedFeature)
  }, [selectedFeature])

  return (
    <Paper elevation={0} sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Box sx={{ flex: 1, minHeight: 280, position: 'relative' }}>
        <Box ref={containerRef} sx={{ position: 'absolute', inset: 0 }} />
        {layers.length === 0 && (
          <Box
            sx={{
              position: 'absolute',
              inset: 0,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              pointerEvents: 'none',
            }}
          >
            <Typography color="text.secondary">选择空间数据集后加入地图预览</Typography>
          </Box>
        )}
      </Box>
      <MapLayerList
        layers={layers}
        onVisibleChange={onLayerVisibleChange}
        onRemoveLayer={onRemoveLayer}
        onFitAll={() => adapterRef.current?.fitAllVisibleLayers()}
      />
      <Box sx={{ p: 1.5, borderTop: 1, borderColor: 'divider', minHeight: 72 }}>
        {layers.some((layer) => layer.error) && (
          <Alert severity="warning" sx={{ mb: 1 }}>
            部分数据集无法加入地图，请查看图层列表。
          </Alert>
        )}
        {selectedFeatureAttributes ? (
          <Stack spacing={0.5}>
            <Typography variant="body2">
              {selectedFeatureAttributes.datasetName} · SmID {selectedFeatureAttributes.id} ·{' '}
              {selectedFeatureAttributes.geometryType}
            </Typography>
            <Typography variant="caption" color="text.secondary" noWrap>
              {Object.entries(selectedFeatureAttributes.properties)
                .slice(0, 4)
                .map(([key, value]) => `${key}: ${value}`)
                .join(' · ') || '无用户属性'}
            </Typography>
          </Stack>
        ) : (
          <Typography variant="body2" color="text.secondary">
            点击地图要素或属性表行查看属性摘要
          </Typography>
        )}
      </Box>
    </Paper>
  )
}
