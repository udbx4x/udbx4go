import React, { useEffect } from 'react'
import {
  ThemeProvider,
  createTheme,
  CssBaseline,
  Box,
  Alert,
  Snackbar,
} from '@mui/material'
import { useUDBX } from './hooks/useUDBX'
import { DatasetList } from './components/DatasetList'
import { DataTable } from './components/DataTable'
import { SpatialPreviewPanel } from './components/SpatialPreviewPanel'
import { StatusBar } from './components/StatusBar'

const theme = createTheme({
  palette: {
    mode: 'light',
  },
})

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
    <ThemeProvider theme={theme}>
      <CssBaseline />
      <Box sx={{ height: '100vh', display: 'flex', flexDirection: 'column' }}>
        {/* Menu Bar */}
        <Box sx={{ p: 1, borderBottom: 1, borderColor: 'divider', bgcolor: 'background.paper' }}>
          <Box component="nav" sx={{ display: 'flex', gap: 2 }}>
            <button onClick={handleOpenFile} style={{ padding: '6px 16px' }}>
              打开文件
            </button>
            <button onClick={handleCloseFile} style={{ padding: '6px 16px' }} disabled={!currentFile}>
              关闭文件
            </button>
          </Box>
        </Box>

        {/* Main Content */}
        <Box sx={{ flex: 1, display: 'flex', overflow: 'hidden' }}>
          {/* Left Sidebar - Dataset List */}
          <Box sx={{ width: 280, flexShrink: 0, borderRight: 1, borderColor: 'divider' }}>
            <DatasetList
              datasets={datasets}
              selectedDataset={selectedDataset}
              onSelectDataset={handleSelectDataset}
            />
          </Box>

          {/* Right Content - Map and Data Table */}
          <Box
            sx={{
              flex: 1,
              overflow: 'hidden',
              display: 'grid',
              gridTemplateRows: 'minmax(320px, 1fr) minmax(220px, 0.75fr)',
            }}
          >
            <Box sx={{ overflow: 'hidden', borderBottom: 1, borderColor: 'divider' }}>
              <SpatialPreviewPanel
                layers={mapLayers}
                selectedFeature={selectedMapFeature}
                selectedFeatureAttributes={selectedFeatureAttributes}
                onFeatureSelect={selectFeature}
                onLayerVisibleChange={setMapLayerVisible}
                onRemoveLayer={removeMapLayer}
              />
            </Box>
            <Box sx={{ overflow: 'hidden' }}>
              <DataTable
                pageData={pageData}
                datasetName={activeTableDataset}
                selectedFeature={selectedMapFeature}
                onFeatureSelect={selectFeature}
                onPageChange={handlePageChange}
              />
            </Box>
          </Box>
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
