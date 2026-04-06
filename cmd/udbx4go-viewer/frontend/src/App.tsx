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
    pageData,
    loading,
    error,
    openFileDialog,
    closeFile,
    loadDataset,
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
    if (selectedDataset) {
      loadDataset(selectedDataset, page)
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

          {/* Right Content - Data Table */}
          <Box sx={{ flex: 1, overflow: 'hidden' }}>
            <DataTable
              pageData={pageData}
              datasetName={selectedDataset}
              onPageChange={handlePageChange}
            />
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
