import React from 'react'
import {
  Box,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  KeyboardArrowDown as CollapseTableIcon,
  KeyboardArrowUp as ExpandTableIcon,
  TableRows as TableRowsIcon,
} from '@mui/icons-material'
import { viewerLayout } from '../theme/viewerTheme'
import type { PageData } from '../types'
import { DataTable } from './DataTable'

interface AttributeTableDrawerProps {
  open: boolean
  pageData: PageData | null
  datasetName: string | null
  selectedFeature: { datasetName: string; featureID: number } | null
  onToggleOpen: () => void
  onFeatureSelect: (datasetName: string, featureID: number) => void
  onPageChange: (page: number) => void
}

export const AttributeTableDrawer: React.FC<AttributeTableDrawerProps> = ({
  open,
  pageData,
  datasetName,
  selectedFeature,
  onToggleOpen,
  onFeatureSelect,
  onPageChange,
}) => {
  const toggleLabel = open ? '收起属性表' : '展开属性表'
  const pageSummary = pageData
    ? `第 ${pageData.currentPage} / ${pageData.totalPages} 页 · 共 ${pageData.rows.length} 条记录`
    : '未选择数据集'

  return (
    <Box
      sx={{
        height: open ? viewerLayout.tableExpandedHeight : viewerLayout.tableCollapsedHeight,
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
          borderBottom: open ? 1 : 0,
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
        <Stack direction="row" spacing={0.5} alignItems="center">
          <Tooltip title={toggleLabel}>
            <IconButton size="small" aria-label={toggleLabel} onClick={onToggleOpen}>
              {open ? <CollapseTableIcon fontSize="small" /> : <ExpandTableIcon fontSize="small" />}
            </IconButton>
          </Tooltip>
        </Stack>
      </Box>

      {open && (
        <Box sx={{ flex: 1, minHeight: 0 }}>
          <DataTable
            pageData={pageData}
            datasetName={datasetName}
            selectedFeature={selectedFeature}
            onFeatureSelect={onFeatureSelect}
            onPageChange={onPageChange}
          />
        </Box>
      )}
    </Box>
  )
}
