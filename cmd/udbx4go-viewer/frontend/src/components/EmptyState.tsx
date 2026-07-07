import React from 'react'
import { Box, Typography } from '@mui/material'

interface EmptyStateProps {
  title: string
  description?: string
}

export const EmptyState: React.FC<EmptyStateProps> = ({ title, description }) => {
  return (
    <Box
      sx={{
        width: '100%',
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        px: 3,
        textAlign: 'center',
        pointerEvents: 'none',
      }}
    >
      <Typography variant="body2" color="text.secondary">
        {title}
      </Typography>
      {description && (
        <Typography variant="caption" color="text.secondary" sx={{ mt: 0.5 }}>
          {description}
        </Typography>
      )}
    </Box>
  )
}
