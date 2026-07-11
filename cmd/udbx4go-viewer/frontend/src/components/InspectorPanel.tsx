import React from 'react'
import { Box, Divider, Tab, Tabs, Typography } from '@mui/material'
import { FeaturePanel } from './FeaturePanel'
import { LayerPanel } from './LayerPanel'
import type { FeatureAttributes, MapLayerState } from '../types'

type InspectorTab = 'layers' | 'attributes' | 'styles'

interface InspectorPanelProps {
  layers: MapLayerState[]
  showPreviewStats: boolean
  selectedFeatureAttributes: FeatureAttributes | null
  onLayerVisibleChange: (datasetName: string, visible: boolean) => void
  onRemoveLayer: (datasetName: string) => void
}

export const InspectorPanel: React.FC<InspectorPanelProps> = ({
  layers,
  showPreviewStats,
  selectedFeatureAttributes,
  onLayerVisibleChange,
  onRemoveLayer,
}) => {
  const [selectedTab, setSelectedTab] = React.useState<InspectorTab>('layers')

  React.useEffect(() => {
    if (selectedFeatureAttributes) {
      setSelectedTab('attributes')
    }
  }, [selectedFeatureAttributes])

  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', minHeight: 0, bgcolor: 'background.paper' }}>
      <Tabs
        aria-label="检查器视图"
        value={selectedTab}
        onChange={(_, value: InspectorTab) => setSelectedTab(value)}
        variant="fullWidth"
        sx={{ minHeight: 44, flexShrink: 0 }}
      >
        <Tab value="layers" label="图层" sx={{ minHeight: 44 }} />
        <Tab value="attributes" label="属性" sx={{ minHeight: 44 }} />
        <Tab value="styles" label="样式" sx={{ minHeight: 44 }} />
      </Tabs>
      <Divider />

      <Box sx={{ flex: 1, minHeight: 0, overflow: 'hidden' }}>
        {selectedTab === 'layers' && (
          <LayerPanel
            layers={layers}
            showPreviewStats={showPreviewStats}
            onVisibleChange={onLayerVisibleChange}
            onRemoveLayer={onRemoveLayer}
          />
        )}
        {selectedTab === 'attributes' && <FeaturePanel attributes={selectedFeatureAttributes} />}
        {selectedTab === 'styles' && (
          <Box sx={{ px: 2, py: 3 }}>
            <Typography variant="body2" color="text.secondary">
              样式设置将在后续版本支持
            </Typography>
          </Box>
        )}
      </Box>
    </Box>
  )
}
