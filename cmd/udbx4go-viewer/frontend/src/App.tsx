import React, { useEffect } from 'react'
import {
  ThemeProvider,
  CssBaseline,
  Box,
  Alert,
  Snackbar,
} from '@mui/material'
import { useUDBX } from './hooks/useUDBX'
import { DatasetExplorer } from './components/DatasetExplorer'
import { DataTable } from './components/DataTable'
import { MapWorkspace } from './components/MapWorkspace'
import { AppShell } from './components/AppShell'
import { TopToolbar } from './components/TopToolbar'
import { StatusBar } from './components/StatusBar'
import { viewerLayout, viewerTheme } from './theme/viewerTheme'

function App() {
  const {
    currentFile,
    datasets,
    selectedDataset,
    activeTableDataset,
    pageData,
    mapLayers,
    selectedMapFeature,
    loading,
    error,
    openFileDialog,
    closeFile,
    loadDataset,
    selectFeature,
  } = useUDBX()

  const [errorOpen, setErrorOpen] = React.useState(false)
  const [tableOpen, setTableOpen] = React.useState(true)

  useEffect(() => {
    if (error) {
      setErrorOpen(true)
    }
  }, [error])

  const handleOpenFile = async () => {
    await openFileDialog()
  }

  const handleCloseFile = async () => {
    await closeFile()
  }

  const handleSelectDataset = (name: string) => {
    loadDataset(name, 1)
  }

  const handlePageChange = (page: number) => {
    if (activeTableDataset) {
      loadDataset(activeTableDataset, page)
    }
  }

  return (
    <ThemeProvider theme={viewerTheme}>
      <CssBaseline />
      <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
        <Box sx={{ flex: 1, minHeight: 0 }}>
          <AppShell
            toolbar={
              <TopToolbar
                currentFile={currentFile}
                loading={loading}
                tableOpen={tableOpen}
                onOpenFile={handleOpenFile}
                onCloseFile={handleCloseFile}
                onToggleTable={() => setTableOpen((open) => !open)}
              />
            }
            datasetExplorer={
              <DatasetExplorer
                datasets={datasets}
                selectedDataset={selectedDataset}
                activeTableDataset={activeTableDataset}
                mapLayers={mapLayers}
                onSelectDataset={handleSelectDataset}
              />
            }
            mapWorkspace={
              <MapWorkspace
                layers={mapLayers}
                selectedFeature={selectedMapFeature}
                onFeatureSelect={selectFeature}
              />
            }
            inspector={<Box sx={{ height: '100%', bgcolor: 'background.paper' }} />}
            tableDrawer={
              <Box
                sx={{
                  height: tableOpen ? viewerLayout.tableExpandedHeight : viewerLayout.tableCollapsedHeight,
                  overflow: 'hidden',
                  bgcolor: 'background.paper',
                }}
              >
                {tableOpen && (
                  <DataTable
                    pageData={pageData}
                    datasetName={activeTableDataset}
                    selectedFeature={selectedMapFeature}
                    onFeatureSelect={selectFeature}
                    onPageChange={handlePageChange}
                  />
                )}
              </Box>
            }
          />
        </Box>

        {/* Status Bar */}
        <StatusBar currentFile={currentFile} loading={loading} />

        {/* Error Snackbar */}
        <Snackbar
          open={errorOpen}
          autoHideDuration={6000}
          onClose={() => setErrorOpen(false)}
          anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
        >
          <Alert severity="error" onClose={() => setErrorOpen(false)}>
            {error}
          </Alert>
        </Snackbar>
      </Box>
    </ThemeProvider>
  )
}

export default App
