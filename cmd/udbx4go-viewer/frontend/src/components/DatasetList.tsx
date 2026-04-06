import React from 'react'
import {
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Paper,
  Typography,
  Box,
} from '@mui/material'
import {
  LocationOn as PointIcon,
  ShowChart as LineIcon,
  CropDin as RegionIcon,
  TableChart as TableIcon,
  Help as UnknownIcon,
} from '@mui/icons-material'
import type { DatasetInfo } from '../types'

interface DatasetListProps {
  datasets: DatasetInfo[]
  selectedDataset: string | null
  onSelectDataset: (name: string) => void
}

const getIcon = (iconType: string) => {
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

export const DatasetList: React.FC<DatasetListProps> = ({
  datasets,
  selectedDataset,
  onSelectDataset,
}) => {
  return (
    <Paper elevation={0} sx={{ height: '100%', overflow: 'auto' }}>
      <Box sx={{ p: 2, borderBottom: 1, borderColor: 'divider' }}>
        <Typography variant="h6" component="div">
          数据集列表
        </Typography>
        <Typography variant="caption" color="text.secondary">
          共 {datasets.length} 个数据集
        </Typography>
      </Box>
      <List sx={{ p: 0 }}>
        {datasets.map((dataset) => (
          <ListItem key={dataset.name} disablePadding>
            <ListItemButton
              selected={selectedDataset === dataset.name}
              onClick={() => onSelectDataset(dataset.name)}
            >
              <ListItemIcon>{getIcon(dataset.iconType)}</ListItemIcon>
              <ListItemText
                primary={dataset.name}
                secondary={`${dataset.kind} · ${dataset.objectCount} 条`}
              />
            </ListItemButton>
          </ListItem>
        ))}
      </List>
    </Paper>
  )
}
