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
import { AttributeTableDrawer } from './components/AttributeTableDrawer'
import { MapWorkspace } from './components/MapWorkspace'
import { AppShell } from './components/AppShell'
import { TopToolbar } from './components/TopToolbar'
import { StatusBar } from './components/StatusBar'
import { InspectorPanel } from './components/InspectorPanel'
import { viewerTheme } from './theme/viewerTheme'

function App() {
  const {
    currentFile,
    datasets,
    selectedDataset,
    activeTableDataset,
    pageData,
    mapLayers,
    selectedMapFeature,
    selectedFeatureAttributes,
    loading,
    error,
    openFileDialog,
    closeFile,
    loadDataset,
    setMapLayerVisible,
    removeMapLayer,
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
            inspector={
              <InspectorPanel
                layers={mapLayers}
                selectedFeatureAttributes={selectedFeatureAttributes}
                onLayerVisibleChange={setMapLayerVisible}
                onRemoveLayer={removeMapLayer}
              />
            }
            tableDrawer={
              <AttributeTableDrawer
                open={tableOpen}
                pageData={pageData}
                datasetName={activeTableDataset}
                selectedFeature={selectedMapFeature}
                onToggleOpen={() => setTableOpen((open) => !open)}
                onFeatureSelect={selectFeature}
                onPageChange={handlePageChange}
              />
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
