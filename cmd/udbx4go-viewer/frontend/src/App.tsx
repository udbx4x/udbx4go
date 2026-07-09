import React, { useEffect } from 'react'
import {
  ThemeProvider,
  CssBaseline,
  Alert,
  Snackbar,
} from '@mui/material'
import { useUDBX } from './hooks/useUDBX'
import { useViewerSettings } from './hooks/useViewerSettings'
import { DatasetExplorer } from './components/DatasetExplorer'
import { AttributeTableDrawer } from './components/AttributeTableDrawer'
import { MapWorkspace } from './components/MapWorkspace'
import { AppShell } from './components/AppShell'
import { TopToolbar } from './components/TopToolbar'
import { InspectorPanel } from './components/InspectorPanel'
import { SettingsDialog } from './components/SettingsDialog'
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
  const {
    settings,
    loading: settingsLoading,
    error: settingsError,
    saveSettings,
    resetSettings,
  } = useViewerSettings()

  const [errorOpen, setErrorOpen] = React.useState(false)
  const [tableOpen, setTableOpen] = React.useState(true)
  const [settingsOpen, setSettingsOpen] = React.useState(false)
  const [settingsSaving, setSettingsSaving] = React.useState(false)
  const displayError = error || settingsError

  useEffect(() => {
    if (displayError) {
      setErrorOpen(true)
    }
  }, [displayError])

  useEffect(() => {
    setTableOpen(settings.table.defaultOpen)
  }, [settings.table.defaultOpen])

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
      <AppShell
        toolbar={
          <TopToolbar
            currentFile={currentFile}
            loading={loading}
            tableOpen={tableOpen}
            onOpenFile={handleOpenFile}
            onCloseFile={handleCloseFile}
            onToggleTable={() => setTableOpen((open) => !open)}
            onOpenSettings={() => setSettingsOpen(true)}
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

      <SettingsDialog
        open={settingsOpen}
        settings={settings}
        disabled={settingsLoading || settingsSaving}
        onClose={() => setSettingsOpen(false)}
        onSave={async (nextSettings) => {
          setSettingsSaving(true)
          try {
            await saveSettings(nextSettings)
            setSettingsOpen(false)
          } finally {
            setSettingsSaving(false)
          }
        }}
        onReset={async () => {
          setSettingsSaving(true)
          try {
            await resetSettings()
          } finally {
            setSettingsSaving(false)
          }
        }}
      />

      <Snackbar
        open={errorOpen}
        autoHideDuration={6000}
        onClose={() => setErrorOpen(false)}
        anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
      >
        <Alert severity="error" onClose={() => setErrorOpen(false)}>
          {displayError}
        </Alert>
      </Snackbar>
    </ThemeProvider>
  )
}

export default App
