import React from 'react'
import {
  DataGrid,
  GridColDef,
  GridPaginationModel,
} from '@mui/x-data-grid'
import {
  Paper,
  Box,
  Typography,
  Pagination,
  Stack,
} from '@mui/material'
import type { PageData } from '../types'

interface DataTableProps {
  pageData: PageData | null
  datasetName: string | null
  onPageChange: (page: number) => void
}

export const DataTable: React.FC<DataTableProps> = ({
  pageData,
  datasetName,
  onPageChange,
}) => {
  if (!pageData || !datasetName) {
    return (
      <Paper
        elevation={0}
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
      </Paper>
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
    const rowData: Record<string, string> = { id: rowIndex.toString() }
    row.forEach((cell, cellIndex) => {
      rowData[`col${cellIndex}`] = cell
    })
    return rowData
  })

  const handlePaginationChange = (_: React.ChangeEvent<unknown>, page: number) => {
    onPageChange(page)
  }

  return (
    <Paper elevation={0} sx={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <Box sx={{ p: 2, borderBottom: 1, borderColor: 'divider' }}>
        <Typography variant="h6" component="div">
          {datasetName}
        </Typography>
        <Typography variant="caption" color="text.secondary">
          第 {pageData.currentPage} / {pageData.totalPages} 页 · 共 {rows.length} 条记录
        </Typography>
      </Box>

      <Box sx={{ flex: 1, overflow: 'auto' }}>
        <DataGrid
          rows={rows}
          columns={columns}
          hideFooterPagination
          hideFooter
          disableRowSelectionOnClick
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

      <Box sx={{ p: 1, borderTop: 1, borderColor: 'divider' }}>
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
    </Paper>
  )
}
