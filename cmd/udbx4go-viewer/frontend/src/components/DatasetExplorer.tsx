import React, { useMemo, useState } from 'react'
import {
  Box,
  Chip,
  InputAdornment,
  List,
  ListItem,
  ListItemButton,
  ListItemIcon,
  ListItemText,
  Paper,
  Stack,
  TextField,
  ToggleButton,
  ToggleButtonGroup,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  AddCircleOutline,
  CropDin as RegionIcon,
  HelpOutline as UnknownIcon,
  LocationOn as PointIcon,
  Search as SearchIcon,
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

type DatasetFilter = 'all' | 'spatial' | 'tabular' | 'unknown'

const spatialKinds = new Set(['point', 'pointZ', 'line', 'lineZ', 'region', 'regionZ'])

const isUnknownDataset = (dataset: DatasetInfo) =>
  dataset.kind === 'unknown' || dataset.iconType === 'unknown'

const isSpatialDataset = (dataset: DatasetInfo) =>
  !isUnknownDataset(dataset) && spatialKinds.has(dataset.kind)

const getDatasetCategory = (dataset: DatasetInfo): Exclude<DatasetFilter, 'all'> => {
  if (isUnknownDataset(dataset)) {
    return 'unknown'
  }

  if (dataset.kind === 'tabular') {
    return 'tabular'
  }

  return isSpatialDataset(dataset) ? 'spatial' : 'unknown'
}

const getDatasetSecondaryText = (dataset: DatasetInfo) =>
  isUnknownDataset(dataset)
    ? `未知类型 · ${dataset.objectCount} 条`
    : `${dataset.kind} · ${dataset.objectCount} 条`

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
  const [searchText, setSearchText] = useState('')
  const [typeFilter, setTypeFilter] = useState<DatasetFilter>('all')

  const mapLayerNames = new Set(mapLayers.map((layer) => layer.datasetName))
  const normalizedSearchText = searchText.trim().toLowerCase()
  const visibleDatasets = useMemo(
    () =>
      datasets.filter((dataset) => {
        const matchesSearch =
          normalizedSearchText.length === 0 ||
          dataset.name.toLowerCase().includes(normalizedSearchText)
        const matchesType =
          typeFilter === 'all' || getDatasetCategory(dataset) === typeFilter

        return matchesSearch && matchesType
      }),
    [datasets, normalizedSearchText, typeFilter],
  )

  const handleTypeFilterChange = (
    _event: React.MouseEvent<HTMLElement>,
    nextFilter: DatasetFilter | null,
  ) => {
    if (nextFilter !== null) {
      setTypeFilter(nextFilter)
    }
  }

  return (
    <Paper elevation={0} sx={{ height: '100%', overflow: 'auto' }}>
      <Box sx={{ p: 1.5, borderBottom: 1, borderColor: 'divider' }}>
        <Typography variant="h6" component="div">
          数据集
        </Typography>
        <Typography variant="caption" color="text.secondary">
          {datasets.length} 个数据集
        </Typography>
        {datasets.length > 0 && (
          <Stack spacing={1} sx={{ mt: 1.5 }}>
            <TextField
              placeholder="搜索数据集"
              value={searchText}
              onChange={(event) => setSearchText(event.target.value)}
              size="small"
              fullWidth
              inputProps={{ 'aria-label': '搜索数据集' }}
              InputProps={{
                startAdornment: (
                  <InputAdornment position="start">
                    <SearchIcon fontSize="small" />
                  </InputAdornment>
                ),
              }}
            />
            <ToggleButtonGroup
              value={typeFilter}
              exclusive
              onChange={handleTypeFilterChange}
              size="small"
              fullWidth
              aria-label="数据集类型过滤"
            >
              <ToggleButton value="all" aria-label="全部">
                全部
              </ToggleButton>
              <ToggleButton value="spatial" aria-label="空间">
                空间
              </ToggleButton>
              <ToggleButton value="tabular" aria-label="表格">
                表格
              </ToggleButton>
              <ToggleButton value="unknown" aria-label="未知">
                未知
              </ToggleButton>
            </ToggleButtonGroup>
          </Stack>
        )}
      </Box>

      {datasets.length === 0 ? (
        <Box sx={{ px: 2, py: 3 }}>
          <Typography variant="body2" color="text.secondary">
            打开 UDBX 文件后查看数据集
          </Typography>
        </Box>
      ) : visibleDatasets.length === 0 ? (
        <Box sx={{ px: 2, py: 3 }}>
          <Typography variant="body2" color="text.secondary">
            没有匹配的数据集
          </Typography>
        </Box>
      ) : (
        <List sx={{ p: 0 }}>
          {visibleDatasets.map((dataset) => {
            const isSelected =
              selectedDataset === dataset.name || activeTableDataset === dataset.name
            const isMapLayer = mapLayerNames.has(dataset.name)
            const isSpatial = isSpatialDataset(dataset)
            const isUnknown = isUnknownDataset(dataset)

            return (
              <ListItem key={dataset.name} disablePadding>
                <ListItemButton
                  selected={isSelected}
                  onClick={() => onSelectDataset(dataset.name)}
                  sx={{ minHeight: 56, gap: 1 }}
                >
                  <ListItemIcon sx={{ minWidth: 40 }}>
                    {getDatasetIcon(dataset.iconType)}
                  </ListItemIcon>
                  <ListItemText
                    primary={
                      <Typography
                        variant="body2"
                        component="span"
                        title={dataset.name}
                        sx={{
                          display: 'block',
                          overflow: 'hidden',
                          textOverflow: 'ellipsis',
                          whiteSpace: 'nowrap',
                        }}
                      >
                        {dataset.name}
                      </Typography>
                    }
                    secondary={
                      <Tooltip
                        title={isUnknown ? '无法识别的数据集类型，暂按未知类型展示' : ''}
                        describeChild
                      >
                        <Typography
                          variant="caption"
                          color="text.secondary"
                          component="span"
                        >
                          {getDatasetSecondaryText(dataset)}
                        </Typography>
                      </Tooltip>
                    }
                    sx={{ minWidth: 0, my: 0 }}
                  />
                  {isMapLayer && (
                    <Chip
                      label="已加入"
                      size="small"
                      variant="filled"
                      sx={{
                        height: 22,
                        bgcolor: 'action.selected',
                        color: 'text.secondary',
                        '& .MuiChip-label': { px: 0.75 },
                      }}
                    />
                  )}
                  {!isMapLayer && isSpatial && (
                    <Tooltip title="加入地图" describeChild>
                      <Box
                        component="span"
                        aria-hidden="true"
                        sx={{
                          display: 'inline-flex',
                          color: 'text.secondary',
                          opacity: 0.72,
                        }}
                      >
                        <AddCircleOutline fontSize="small" />
                      </Box>
                    </Tooltip>
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
