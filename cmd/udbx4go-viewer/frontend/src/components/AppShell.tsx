import React from 'react'
import { Box } from '@mui/material'
import { viewerLayout } from '../theme/viewerTheme'

interface AppShellProps {
  toolbar: React.ReactNode
  datasetExplorer: React.ReactNode
  mapWorkspace: React.ReactNode
  inspector: React.ReactNode
  tableDrawer: React.ReactNode
}

export const AppShell: React.FC<AppShellProps> = ({
  toolbar,
  datasetExplorer,
  mapWorkspace,
  inspector,
  tableDrawer,
}) => {
  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', bgcolor: 'background.default' }}>
      {toolbar}
      <Box
        sx={{
          flex: 1,
          minHeight: 0,
          display: 'grid',
          gridTemplateColumns: `${viewerLayout.datasetPanelWidth}px minmax(360px, 1fr) ${viewerLayout.inspectorWidth}px`,
          gridTemplateRows: 'minmax(320px, 1fr) auto',
          overflow: 'hidden',
        }}
      >
        <Box sx={{ gridRow: '1 / span 2', minHeight: 0, overflow: 'hidden', borderRight: 1, borderColor: 'divider' }}>
          {datasetExplorer}
        </Box>
        <Box sx={{ minHeight: 0, overflow: 'hidden', borderBottom: 1, borderColor: 'divider' }}>
          {mapWorkspace}
        </Box>
        <Box
          sx={{
            gridColumn: 3,
            gridRow: '1 / span 2',
            minHeight: 0,
            overflow: 'hidden',
            borderLeft: 1,
            borderColor: 'divider',
          }}
        >
          {inspector}
        </Box>
        <Box sx={{ gridColumn: 2, gridRow: 2, minHeight: 0, overflow: 'hidden' }}>
          {tableDrawer}
        </Box>
      </Box>
    </Box>
  )
}
