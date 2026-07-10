import React from 'react'
import {
  Box,
  Checkbox,
  IconButton,
  List,
  ListItem,
  ListItemText,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import { Close as CloseIcon, Tune as TuneIcon } from '@mui/icons-material'
import type { MapLayerState } from '../types'

interface LayerPanelProps {
  layers: MapLayerState[]
  showPreviewStats: boolean
  onVisibleChange: (datasetName: string, visible: boolean) => void
  onRemoveLayer: (datasetName: string) => void
}

function getLayerColor(layer: MapLayerState): string {
  if (layer.kind === 'line' || layer.kind === 'lineZ') {
    return layer.style.line.strokeColor
  }

  if (layer.kind === 'region' || layer.kind === 'regionZ') {
    return layer.style.polygon.strokeColor
  }

  return layer.style.point.fillColor
}

function getLayerStatus(layer: MapLayerState): string {
  if (layer.loading) {
    return '加载中'
  }

  if (layer.error) {
    return layer.error
  }

  return `${layer.kind} · ${layer.preview?.features.length ?? 0} 个预览要素`
}

export const LayerPanel: React.FC<LayerPanelProps> = ({
  layers,
  showPreviewStats,
  onVisibleChange,
  onRemoveLayer,
}) => {
  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
      <Box sx={{ px: 2, py: 1.5, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <Typography variant="subtitle2">地图图层</Typography>
        <Typography variant="caption" color="text.secondary">
          {layers.length} 个图层
        </Typography>
      </Box>

      {layers.length === 0 ? (
        <Box sx={{ px: 2, py: 3 }}>
          <Typography variant="body2" color="text.secondary">
            选择空间数据集后加入地图
          </Typography>
        </Box>
      ) : (
        <List dense disablePadding sx={{ overflow: 'auto' }}>
          {layers.map((layer) => (
            <ListItem
              key={layer.datasetName}
              sx={{ pr: 9.5, py: 1 }}
              secondaryAction={
                <Stack direction="row" spacing={0.5}>
                  <Tooltip title="样式设置">
                    <span>
                      <IconButton size="small" disabled aria-label={`${layer.datasetName} 图层样式设置`}>
                        <TuneIcon fontSize="small" />
                      </IconButton>
                    </span>
                  </Tooltip>
                  <Tooltip title="从地图移除">
                    <IconButton
                      edge="end"
                      size="small"
                      aria-label={`从地图移除 ${layer.datasetName}`}
                      onClick={() => onRemoveLayer(layer.datasetName)}
                    >
                      <CloseIcon fontSize="small" />
                    </IconButton>
                  </Tooltip>
                </Stack>
              }
            >
              <Checkbox
                edge="start"
                size="small"
                checked={layer.visible}
                inputProps={{ 'aria-label': `切换 ${layer.datasetName} 图层可见性` }}
                onChange={(event) => onVisibleChange(layer.datasetName, event.target.checked)}
              />
              <Box
                aria-label={`${layer.datasetName} 图层样式色块`}
                sx={{
                  width: 14,
                  height: 14,
                  borderRadius: '50%',
                  bgcolor: getLayerColor(layer),
                  border: 1,
                  borderColor: 'divider',
                  mr: 1,
                  flexShrink: 0,
                }}
              />
              <ListItemText
                primary={layer.datasetName}
                secondary={
                  <Box component="div">
                    <Typography
                      component="span"
                      variant="body2"
                      color={layer.error ? 'error' : 'text.secondary'}
                      noWrap
                      title={getLayerStatus(layer)}
                      sx={{ display: 'block' }}
                    >
                      {getLayerStatus(layer)}
                    </Typography>
                    {showPreviewStats && layer.preview && (
                      <Stack direction="row" spacing={1} sx={{ mt: 0.5 }}>
                        <Typography variant="caption" color="text.secondary">
                          预览要素 {layer.preview.features.length}
                        </Typography>
                        <Typography variant="caption" color="text.secondary">
                          顶点 {layer.preview.estimatedVertexCount}
                        </Typography>
                        {layer.preview.sampled && (
                          <Typography variant="caption" color="warning.main">
                            {layer.preview.sampleReason || '已采样'}
                          </Typography>
                        )}
                      </Stack>
                    )}
                  </Box>
                }
                secondaryTypographyProps={{
                  component: 'div',
                }}
              />
            </ListItem>
          ))}
        </List>
      )}
    </Box>
  )
}
