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
import type { DatasetInfo } from './types'
import type { AttributeTableMode } from './components/AttributeTableDrawer'

const spatialDatasetKinds = new Set(['point', 'pointZ', 'line', 'lineZ', 'region', 'regionZ'])

const isUnknownDataset = (dataset: DatasetInfo) =>
  dataset.kind === 'unknown' || dataset.iconType === 'unknown'

const isSpatialDataset = (dataset: DatasetInfo) =>
  !isUnknownDataset(dataset) && spatialDatasetKinds.has(dataset.kind)

function App() {
  const {
    settings,
    loading: settingsLoading,
    error: settingsError,
    saveSettings,
    resetSettings,
  } = useViewerSettings()
  const udbx = useUDBX({
    spatialPreviewFeatureLimit: settings.spatialPreview.featureLimit,
    spatialPreviewVertexBudget: settings.spatialPreview.vertexBudget,
  })
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
    loadTableDataset,
    setMapLayerVisible,
    removeMapLayer,
    selectFeature,
  } = udbx

  const [errorOpen, setErrorOpen] = React.useState(false)
  const [tableMode, setTableMode] = React.useState<AttributeTableMode>('half')
  const [settingsOpen, setSettingsOpen] = React.useState(false)
  const [settingsSaving, setSettingsSaving] = React.useState(false)
  const settingsDefaultAppliedRef = React.useRef(false)
  const displayError = error || settingsError

  useEffect(() => {
    if (displayError) {
      setErrorOpen(true)
    }
  }, [displayError])

  useEffect(() => {
    if (!settingsLoading && !settingsDefaultAppliedRef.current) {
      setTableMode(settings.table.defaultOpen ? 'half' : 'collapsed')
      settingsDefaultAppliedRef.current = true
    }
  }, [settingsLoading, settings.table.defaultOpen])

  const handleOpenFile = async () => {
    await openFileDialog()
  }

  const handleCloseFile = async () => {
    await closeFile()
  }

  const handleSelectDataset = (name: string) => {
    const dataset = datasets.find((item) => item.name === name)

    if (dataset && isSpatialDataset(dataset)) {
      loadDataset(name, 1)
      return
    }

    loadTableDataset(name, 1)
  }

  const handlePageChange = (page: number) => {
    if (activeTableDataset) {
      loadTableDataset(activeTableDataset, page)
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
            onOpenFile={handleOpenFile}
            onCloseFile={handleCloseFile}
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
            autoFitOnLayerChange={settings.spatialPreview.autoFitOnLayerChange}
            zoomToSelectedFeature={settings.mapInteraction.zoomToSelectedFeature}
            onFeatureSelect={selectFeature}
          />
        }
        inspector={
          <InspectorPanel
            layers={mapLayers}
            showPreviewStats={settings.advanced.showPreviewStats}
            selectedFeatureAttributes={selectedFeatureAttributes}
            onLayerVisibleChange={setMapLayerVisible}
            onRemoveLayer={removeMapLayer}
          />
        }
        tableDrawer={
          <AttributeTableDrawer
            mode={tableMode}
            pageData={pageData}
            datasetName={activeTableDataset}
            selectedFeature={selectedMapFeature}
            onModeChange={setTableMode}
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
          } catch {
            // useViewerSettings exposes the error through settingsError.
          } finally {
            setSettingsSaving(false)
          }
        }}
        onReset={async () => {
          setSettingsSaving(true)
          try {
            await resetSettings()
          } catch {
            // useViewerSettings exposes the error through settingsError.
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
