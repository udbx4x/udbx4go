import React from 'react'
import {
  Box,
  IconButton,
  Pagination,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  KeyboardArrowDown as CollapseTableIcon,
  TableRows as HalfTableIcon,
  UnfoldMore as FullTableIcon,
  TableRows as TableRowsIcon,
} from '@mui/icons-material'
import { viewerLayout } from '../theme/viewerTheme'
import type { PageData } from '../types'
import { DataTable } from './DataTable'

export type AttributeTableMode = 'collapsed' | 'half' | 'full'

interface AttributeTableDrawerProps {
  mode: AttributeTableMode
  pageData: PageData | null
  datasetName: string | null
  selectedFeature: { datasetName: string; featureID: number } | null
  onModeChange: (mode: AttributeTableMode) => void
  onFeatureSelect: (datasetName: string, featureID: number) => void
  onPageChange: (page: number) => void
}

const tableFullHeight = `${viewerLayout.tableFullMaxHeightRatio * 100}vh`

const getDrawerHeight = (mode: AttributeTableMode) => {
  if (mode === 'collapsed') {
    return viewerLayout.tableCollapsedHeight
  }

  if (mode === 'full') {
    return tableFullHeight
  }

  return viewerLayout.tableHalfHeight
}

export const AttributeTableDrawer: React.FC<AttributeTableDrawerProps> = ({
  mode,
  pageData,
  datasetName,
  selectedFeature,
  onModeChange,
  onFeatureSelect,
  onPageChange,
}) => {
  const isCollapsed = mode === 'collapsed'
  const pageSummary = pageData
    ? `第 ${pageData.currentPage} / ${pageData.totalPages} 页 · 本页 ${pageData.rows.length} 条记录`
    : '未选择数据集'

  return (
    <Box
      sx={{
        height: getDrawerHeight(mode),
        maxHeight: mode === 'full' ? tableFullHeight : undefined,
        overflow: 'hidden',
        bgcolor: 'background.paper',
        borderTop: 1,
        borderColor: 'divider',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      <Box
        component="header"
        sx={{
          height: viewerLayout.tableCollapsedHeight,
          minHeight: viewerLayout.tableCollapsedHeight,
          px: 1.5,
          display: 'flex',
          alignItems: 'center',
          gap: 1,
          borderBottom: isCollapsed ? 0 : 1,
          borderColor: 'divider',
        }}
      >
        <TableRowsIcon fontSize="small" color="action" />
        <Typography
          variant="subtitle2"
          component="h2"
          sx={{ minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
        >
          {datasetName || '属性表'}
        </Typography>
        <Typography
          variant="caption"
          color="text.secondary"
          sx={{ minWidth: 0, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
        >
          {pageSummary}
        </Typography>

        {pageData && !isCollapsed && (
          <Pagination
            aria-label="属性表分页"
            count={pageData.totalPages}
            page={pageData.currentPage}
            onChange={(_, page) => onPageChange(page)}
            color="primary"
            size="small"
            siblingCount={0}
            showFirstButton
            showLastButton
          />
        )}

        <Stack direction="row" spacing={0.5} alignItems="center">
          <Tooltip title="折叠属性表">
            <span>
              <IconButton
                size="small"
                aria-label="折叠属性表"
                disabled={mode === 'collapsed'}
                onClick={() => onModeChange('collapsed')}
              >
                <CollapseTableIcon fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title="半展开属性表">
            <span>
              <IconButton
                size="small"
                aria-label="半展开属性表"
                disabled={mode === 'half'}
                onClick={() => onModeChange('half')}
              >
                <HalfTableIcon fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
          <Tooltip title="全展开属性表">
            <span>
              <IconButton
                size="small"
                aria-label="全展开属性表"
                disabled={mode === 'full'}
                onClick={() => onModeChange('full')}
              >
                <FullTableIcon fontSize="small" />
              </IconButton>
            </span>
          </Tooltip>
        </Stack>
      </Box>

      {!isCollapsed && (
        <Box sx={{ flex: 1, minHeight: 0 }}>
          <DataTable
            pageData={pageData}
            datasetName={datasetName}
            selectedFeature={selectedFeature}
            onFeatureSelect={onFeatureSelect}
          />
        </Box>
      )}
    </Box>
  )
}
