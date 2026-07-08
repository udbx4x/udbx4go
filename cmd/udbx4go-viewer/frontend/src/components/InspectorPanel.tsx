import React from 'react'
import { Box, Divider } from '@mui/material'
import { FeaturePanel } from './FeaturePanel'
import { LayerPanel } from './LayerPanel'
import type { FeatureAttributes, MapLayerState } from '../types'

interface InspectorPanelProps {
  layers: MapLayerState[]
  selectedFeatureAttributes: FeatureAttributes | null
  onLayerVisibleChange: (datasetName: string, visible: boolean) => void
  onRemoveLayer: (datasetName: string) => void
}

export const InspectorPanel: React.FC<InspectorPanelProps> = ({
  layers,
  selectedFeatureAttributes,
  onLayerVisibleChange,
  onRemoveLayer,
}) => {
  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', minHeight: 0, bgcolor: 'background.paper' }}>
      <Box sx={{ flex: 1, minHeight: 0 }}>
        <LayerPanel
          layers={layers}
          onVisibleChange={onLayerVisibleChange}
          onRemoveLayer={onRemoveLayer}
        />
      </Box>
      <Divider />
      <Box sx={{ flex: 1, minHeight: 0 }}>
        <FeaturePanel attributes={selectedFeatureAttributes} />
      </Box>
    </Box>
  )
}
