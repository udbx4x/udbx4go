import React from 'react'
import {
  DataGrid,
  GridColDef,
  GridRowSelectionModel,
} from '@mui/x-data-grid'
import {
  Box,
  Typography,
} from '@mui/material'
import type { PageData } from '../types'

interface DataTableProps {
  pageData: PageData | null
  datasetName: string | null
  selectedFeature: { datasetName: string; featureID: number } | null
  onFeatureSelect: (datasetName: string, featureID: number) => void
}

export const DataTable: React.FC<DataTableProps> = ({
  pageData,
  datasetName,
  selectedFeature,
  onFeatureSelect,
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

  const rowSelectionModel: GridRowSelectionModel =
    selectedFeature?.datasetName === datasetName
      ? { type: 'include', ids: new Set([selectedFeature.featureID.toString()]) }
      : { type: 'include', ids: new Set() }

  return (
    <Box sx={{ height: '100%', minHeight: 0, overflow: 'hidden' }}>
      <DataGrid
        rows={rows}
        columns={columns}
        hideFooterPagination
        hideFooter
        disableColumnMenu
        rowHeight={30}
        columnHeaderHeight={34}
        onRowClick={(params) => {
          const featureID = Number(params.id)
          if (Number.isFinite(featureID)) {
            onFeatureSelect(datasetName, featureID)
          }
        }}
        rowSelectionModel={rowSelectionModel}
        density="compact"
        sx={{
          border: 'none',
          fontSize: '0.8125rem',
          '& .MuiDataGrid-cell': {
            py: 0,
            px: 1,
            lineHeight: '30px',
          },
          '& .MuiDataGrid-columnHeader': {
            fontWeight: 700,
            px: 1,
          },
          '& .MuiDataGrid-row.Mui-selected': {
            bgcolor: 'primary.light',
          },
          '& .MuiDataGrid-row.Mui-selected:hover': {
            bgcolor: 'primary.light',
          },
        }}
      />
    </Box>
  )
}
