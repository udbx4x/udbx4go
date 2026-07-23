import React from 'react'
import {
  Box,
  Checkbox,
  IconButton,
  List,
  ListItemIcon,
  ListItem,
  ListItemText,
  Menu,
  MenuItem,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  DeleteOutline as DeleteOutlineIcon,
  MoreVert as MoreVertIcon,
  Tune as TuneIcon,
  WarningAmber as WarningAmberIcon,
} from '@mui/icons-material'
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
  if (layer.error || layer.queryStatus === 'error') {
    return layer.error || layer.queryError || '当前范围加载失败'
  }
  const degradedStatus = getDegradedStatus(layer)
  if (degradedStatus) {
    return degradedStatus
  }
  if (layer.preview?.hasMore) {
    return `当前范围 ${layer.preview.viewportFeatureCount ?? layer.preview.features.length}+ 个对象，请继续放大`
  }
  if (layer.queryStatus === 'loading') {
    return '加载当前范围'
  }
  if (layer.loading) {
    return '加载中'
  }

  return `${layer.kind} · ${layer.preview?.features.length ?? 0} 个预览要素`
}

function getDegradedStatus(layer: MapLayerState): string | null {
  if (layer.queryStatus !== 'degraded' || layer.preview?.strategy !== 'bounded_sample') {
    return null
  }

  switch (layer.preview.degradedReason) {
    case 'envelope_cache_budget_exceeded':
      return '缓存预算不足，显示有界预览'
    case 'spatial_index_unavailable':
      return '范围索引不可用，显示有界预览'
    default:
      return null
  }
}

export const LayerPanel: React.FC<LayerPanelProps> = ({
  layers,
  showPreviewStats,
  onVisibleChange,
  onRemoveLayer,
}) => {
  const [menuAnchorEl, setMenuAnchorEl] = React.useState<HTMLElement | null>(null)
  const [menuDatasetName, setMenuDatasetName] = React.useState<string | null>(null)

  const menuLayerExists = menuDatasetName
    ? layers.some((layer) => layer.datasetName === menuDatasetName)
    : false

  const closeMenu = () => {
    setMenuAnchorEl(null)
    setMenuDatasetName(null)
  }

  const openLayerMenu = (event: React.MouseEvent<HTMLElement>, layer: MapLayerState) => {
    setMenuAnchorEl(event.currentTarget)
    setMenuDatasetName(layer.datasetName)
  }

  const removeMenuLayer = () => {
    if (menuDatasetName && menuLayerExists) {
      onRemoveLayer(menuDatasetName)
    }
    closeMenu()
  }

  React.useEffect(() => {
    if (menuDatasetName && !menuLayerExists) {
      closeMenu()
    }
  }, [menuDatasetName, menuLayerExists])

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
              sx={{ pr: 6, py: 1 }}
              secondaryAction={
                <Tooltip title="更多操作">
                  <IconButton
                    edge="end"
                    size="small"
                    aria-label={`${layer.datasetName} 更多操作`}
                    aria-controls={menuDatasetName === layer.datasetName ? 'layer-actions-menu' : undefined}
                    aria-haspopup="menu"
                    aria-expanded={menuDatasetName === layer.datasetName ? 'true' : undefined}
                    onClick={(event) => openLayerMenu(event, layer)}
                  >
                    <MoreVertIcon fontSize="small" />
                  </IconButton>
                </Tooltip>
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
                    {layer.preview?.sampled && (
                      <Stack direction="row" spacing={0.5} sx={{ mt: 0.5, alignItems: 'center' }}>
                        <WarningAmberIcon fontSize="inherit" color="warning" />
                        <Typography variant="caption" color="warning.main">
                          {layer.preview.sampleReason || '已采样预览'}
                        </Typography>
                      </Stack>
                    )}
                    {showPreviewStats && layer.preview && (
                      <Typography variant="caption" color="text.secondary" sx={{ display: 'block', mt: 0.5 }}>
                        {formatPreviewStats(layer)}
                      </Typography>
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
      <Menu
        id="layer-actions-menu"
        anchorEl={menuAnchorEl}
        open={Boolean(menuAnchorEl)}
        onClose={closeMenu}
        MenuListProps={{
          'aria-label': menuDatasetName ? `${menuDatasetName} 图层操作` : '图层操作',
          dense: true,
        }}
      >
        <MenuItem disabled>
          <ListItemIcon>
            <TuneIcon fontSize="small" />
          </ListItemIcon>
          样式设置
        </MenuItem>
        <MenuItem onClick={removeMenuLayer}>
          <ListItemIcon>
            <DeleteOutlineIcon fontSize="small" />
          </ListItemIcon>
          移除图层
        </MenuItem>
      </Menu>
    </Box>
  )
}

function formatPreviewStats(layer: MapLayerState): string {
  const preview = layer.preview!
  const parts = [
    preview.strategy,
    `${preview.queryDurationMs} ms`,
    `要素 ${preview.features.length}`,
    `顶点 ${preview.estimatedVertexCount}`,
  ]
  if (preview.degradedReason) {
    parts.push(preview.degradedReason)
  }
  return parts.join(' · ')
}
