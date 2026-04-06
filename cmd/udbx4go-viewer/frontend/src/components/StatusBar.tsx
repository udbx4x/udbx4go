import React from 'react'
import { AppBar, Toolbar, Typography, Box, CircularProgress } from '@mui/material'
import { FolderOpen as FolderIcon } from '@mui/icons-material'

interface StatusBarProps {
  currentFile: string | null
  loading: boolean
}

export const StatusBar: React.FC<StatusBarProps> = ({ currentFile, loading }) => {
  const fileName = currentFile ? currentFile.split('/').pop() : '未打开文件'

  return (
    <AppBar position="static" color="default" elevation={0} sx={{ borderTop: 1, borderColor: 'divider' }}>
      <Toolbar variant="dense" sx={{ minHeight: 40 }}>
        <FolderIcon fontSize="small" sx={{ mr: 1, color: 'text.secondary' }} />
        <Typography variant="body2" color="text.secondary" sx={{ flexGrow: 1 }}>
          {fileName}
        </Typography>
        {loading && (
          <Box sx={{ display: 'flex', alignItems: 'center' }}>
            <CircularProgress size={16} sx={{ mr: 1 }} />
            <Typography variant="caption" color="text.secondary">
              加载中...
            </Typography>
          </Box>
        )}
      </Toolbar>
    </AppBar>
  )
}
