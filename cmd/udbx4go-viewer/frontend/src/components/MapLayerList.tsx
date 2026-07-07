import React from 'react'
import { Box, Checkbox, IconButton, List, ListItem, ListItemText, Tooltip, Typography } from '@mui/material'
import { Close as CloseIcon, CenterFocusStrong as FitIcon } from '@mui/icons-material'
import type { MapLayerState } from '../types'

interface MapLayerListProps {
  layers: MapLayerState[]
  onVisibleChange: (datasetName: string, visible: boolean) => void
  onRemoveLayer: (datasetName: string) => void
  onFitAll: () => void
}

export const MapLayerList: React.FC<MapLayerListProps> = ({
  layers,
  onVisibleChange,
  onRemoveLayer,
  onFitAll,
}) => {
  return (
    <Box sx={{ borderTop: 1, borderColor: 'divider' }}>
      <Box sx={{ px: 1.5, py: 1, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Typography variant="subtitle2">地图图层</Typography>
        <Tooltip title="适配全部可见图层">
          <span>
            <IconButton size="small" onClick={onFitAll} disabled={layers.length === 0}>
              <FitIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
      </Box>
      <List dense disablePadding>
        {layers.map((layer) => (
          <ListItem
            key={layer.datasetName}
            secondaryAction={
              <Tooltip title="从地图移除">
                <IconButton edge="end" size="small" onClick={() => onRemoveLayer(layer.datasetName)}>
                  <CloseIcon fontSize="small" />
                </IconButton>
              </Tooltip>
            }
          >
            <Checkbox
              edge="start"
              size="small"
              checked={layer.visible}
              onChange={(event) => onVisibleChange(layer.datasetName, event.target.checked)}
            />
            <Box
              aria-label={`${layer.datasetName} 图层样式色块`}
              sx={{
                width: 14,
                height: 14,
                borderRadius: '50%',
                bgcolor: layer.style.point.fillColor,
                border: `1px solid ${layer.style.line.strokeColor}`,
                mr: 1,
                flexShrink: 0,
              }}
            />
            <ListItemText
              primary={layer.datasetName}
              secondary={
                layer.loading
                  ? '加载中'
                  : layer.error || `${layer.kind} · ${layer.preview?.features.length ?? 0} 个预览要素`
              }
            />
          </ListItem>
        ))}
      </List>
    </Box>
  )
}
