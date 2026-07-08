import React from 'react'
import {
  DataGrid,
  GridColDef,
  GridRowSelectionModel,
} from '@mui/x-data-grid'
import {
  Box,
  Typography,
  Pagination,
  Stack,
} from '@mui/material'
import type { PageData } from '../types'

interface DataTableProps {
  pageData: PageData | null
  datasetName: string | null
  selectedFeature: { datasetName: string; featureID: number } | null
  onFeatureSelect: (datasetName: string, featureID: number) => void
  onPageChange: (page: number) => void
}

export const DataTable: React.FC<DataTableProps> = ({
  pageData,
  datasetName,
  selectedFeature,
  onFeatureSelect,
  onPageChange,
}) => {
  if (!pageData || !datasetName) {
    return (
      <Box
        sx={{
          height: '100%',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        }}
      >
        <Typography color="text.secondary">
          请从左侧选择一个数据集
        </Typography>
      </Box>
    )
  }

  // Build columns from pageData.columns
  const columns: GridColDef[] = pageData.columns.map((col, index) => ({
    field: `col${index}`,
    headerName: col,
    flex: index === 0 ? 0.5 : 1,
    minWidth: index === 0 ? 60 : 100,
  }))

  // Build rows from pageData.rows
  const rows = pageData.rows.map((row, rowIndex) => {
    const rowData: Record<string, string> = { id: row[0] || rowIndex.toString() }
    row.forEach((cell, cellIndex) => {
      rowData[`col${cellIndex}`] = cell
    })
    return rowData
  })

  const handlePaginationChange = (_: React.ChangeEvent<unknown>, page: number) => {
    onPageChange(page)
  }

  const rowSelectionModel: GridRowSelectionModel =
    selectedFeature?.datasetName === datasetName
      ? { type: 'include', ids: new Set([selectedFeature.featureID.toString()]) }
      : { type: 'include', ids: new Set() }

  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Box sx={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
        <DataGrid
          rows={rows}
          columns={columns}
          hideFooterPagination
          hideFooter
          onRowClick={(params) => {
            if (!datasetName) {
              return
            }
            const featureID = Number(params.id)
            if (Number.isFinite(featureID)) {
              onFeatureSelect(datasetName, featureID)
            }
          }}
          rowSelectionModel={rowSelectionModel}
          density="compact"
          sx={{
            border: 'none',
            '& .MuiDataGrid-cell': {
              fontSize: '0.875rem',
            },
            '& .MuiDataGrid-columnHeader': {
              fontWeight: 'bold',
            },
          }}
        />
      </Box>

      <Box sx={{ p: 0.75, borderTop: 1, borderColor: 'divider' }}>
        <Stack direction="row" justifyContent="center">
          <Pagination
            count={pageData.totalPages}
            page={pageData.currentPage}
            onChange={handlePaginationChange}
            color="primary"
            showFirstButton
            showLastButton
          />
        </Stack>
      </Box>
    </Box>
  )
}
