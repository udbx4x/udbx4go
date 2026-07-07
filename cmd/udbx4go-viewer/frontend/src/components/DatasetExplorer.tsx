import React from 'react'
import {
  Box,
  Chip,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Paper,
  Typography,
} from '@mui/material'
import {
  CropDin as RegionIcon,
  Help as UnknownIcon,
  LocationOn as PointIcon,
  ShowChart as LineIcon,
  TableChart as TableIcon,
} from '@mui/icons-material'
import type { DatasetInfo, MapLayerState } from '../types'

interface DatasetExplorerProps {
  datasets: DatasetInfo[]
  selectedDataset: string | null
  activeTableDataset: string | null
  mapLayers: MapLayerState[]
  onSelectDataset: (name: string) => void
}

const getDatasetIcon = (iconType: string) => {
  switch (iconType) {
    case 'point':
      return <PointIcon color="primary" />
    case 'line':
      return <LineIcon color="success" />
    case 'region':
      return <RegionIcon color="warning" />
    case 'tabular':
      return <TableIcon color="action" />
    default:
      return <UnknownIcon color="disabled" />
  }
}

export const DatasetExplorer: React.FC<DatasetExplorerProps> = ({
  datasets,
  selectedDataset,
  activeTableDataset,
  mapLayers,
  onSelectDataset,
}) => {
  const mapLayerNames = new Set(mapLayers.map((layer) => layer.datasetName))

  return (
    <Paper elevation={0} sx={{ height: '100%', overflow: 'auto' }}>
      <Box sx={{ p: 2, borderBottom: 1, borderColor: 'divider' }}>
        <Typography variant="h6" component="div">
          数据集
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {datasets.length} 个数据集
        </Typography>
      </Box>

      {datasets.length === 0 ? (
        <Box sx={{ px: 2, py: 3 }}>
          <Typography variant="body2" color="text.secondary">
            打开 UDBX 文件后查看数据集
          </Typography>
        </Box>
      ) : (
        <List sx={{ p: 0 }}>
          {datasets.map((dataset) => {
            const isSelected =
              selectedDataset === dataset.name || activeTableDataset === dataset.name
            const isMapLayer = mapLayerNames.has(dataset.name)

            return (
              <ListItem key={dataset.name} disablePadding>
                <ListItemButton
                  selected={isSelected}
                  onClick={() => onSelectDataset(dataset.name)}
                >
                  <ListItemIcon>{getDatasetIcon(dataset.iconType)}</ListItemIcon>
                  <ListItemText
                    primary={dataset.name}
                    secondary={`${dataset.kind} · ${dataset.objectCount} 条`}
                  />
                  {isMapLayer && (
                    <Chip
                      label="已加入地图"
                      size="small"
                      color="primary"
                      variant="outlined"
                    />
                  )}
                </ListItemButton>
              </ListItem>
            )
          })}
        </List>
      )}
    </Paper>
  )
}
