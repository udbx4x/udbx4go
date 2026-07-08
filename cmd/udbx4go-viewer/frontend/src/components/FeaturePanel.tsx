import React from 'react'
import { Box, Stack, Typography } from '@mui/material'
import type { FeatureAttributes } from '../types'

interface FeaturePanelProps {
  attributes: FeatureAttributes | null
}

export const FeaturePanel: React.FC<FeaturePanelProps> = ({ attributes }) => {
  const entries = attributes ? Object.entries(attributes.properties).slice(0, 8) : []

  return (
    <Box sx={{ height: '100%', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
      <Box sx={{ px: 2, py: 1.5 }}>
        <Typography variant="subtitle2">要素属性</Typography>
      </Box>

      {!attributes ? (
        <Box sx={{ px: 2, py: 3 }}>
          <Typography variant="body2" color="text.secondary">
            点击地图要素或属性表行查看属性
          </Typography>
        </Box>
      ) : (
        <Box sx={{ px: 2, pb: 2, overflow: 'auto' }}>
          <Stack spacing={0.5} sx={{ mb: 1.5 }}>
            <Typography variant="body2" fontWeight={600} noWrap title={attributes.datasetName}>
              {attributes.datasetName}
            </Typography>
            <Typography variant="caption" color="text.secondary">
              <Box component="span">SmID {attributes.id}</Box>
              {' · '}
              {attributes.geometryType}
            </Typography>
          </Stack>

          <Stack spacing={1}>
            {entries.map(([key, value]) => (
              <Box key={key}>
                <Typography variant="caption" color="text.secondary" noWrap title={key}>
                  {key}
                </Typography>
                <Typography variant="body2" noWrap title={value}>
                  {value}
                </Typography>
              </Box>
            ))}
          </Stack>
        </Box>
      )}
    </Box>
  )
}
