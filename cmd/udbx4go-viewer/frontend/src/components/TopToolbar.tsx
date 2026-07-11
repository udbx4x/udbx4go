import React from 'react'
import {
  Box,
  Button,
  CircularProgress,
  IconButton,
  Menu,
  MenuItem,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  MoreHoriz as MoreHorizIcon,
  Settings as SettingsIcon,
} from '@mui/icons-material'
import { viewerLayout } from '../theme/viewerTheme'

interface TopToolbarProps {
  currentFile: string | null
  loading: boolean
  onOpenFile: () => void
  onCloseFile: () => void
  onOpenSettings: () => void
}

const getFileName = (path: string | null) => {
  if (!path) {
    return '未打开文件'
  }
  return path.split(/[\\/]/).pop() || path
}

export const TopToolbar: React.FC<TopToolbarProps> = ({
  currentFile,
  loading,
  onOpenFile,
  onCloseFile,
  onOpenSettings,
}) => {
  const [fileMenuAnchor, setFileMenuAnchor] = React.useState<HTMLElement | null>(null)
  const fileName = getFileName(currentFile)
  const fileMenuOpen = Boolean(fileMenuAnchor)

  const handleOpenFileMenu = (event: React.MouseEvent<HTMLElement>) => {
    setFileMenuAnchor(event.currentTarget)
  }

  const handleCloseFileMenu = () => {
    setFileMenuAnchor(null)
  }

  const handleCloseFile = () => {
    handleCloseFileMenu()
    onCloseFile()
  }

  return (
    <Box
      component="header"
      sx={{
        height: viewerLayout.toolbarHeight,
        minHeight: viewerLayout.toolbarHeight,
        px: 1.5,
        display: 'flex',
        alignItems: 'center',
        gap: 2,
        borderBottom: 1,
        borderColor: 'divider',
        bgcolor: 'background.paper',
      }}
    >
      <Typography variant="subtitle1" component="h1" sx={{ whiteSpace: 'nowrap' }}>
        UDBX Viewer
      </Typography>

      <Typography
        variant="body2"
        color={currentFile ? 'text.primary' : 'text.secondary'}
        sx={{ minWidth: 0, flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
        title={fileName}
      >
        {fileName}
      </Typography>

      {loading && (
        <Stack direction="row" spacing={1} alignItems="center" aria-label="加载状态">
          <CircularProgress size={16} />
          <Typography variant="caption" color="text.secondary">
            加载中...
          </Typography>
        </Stack>
      )}

      <Stack direction="row" spacing={1} alignItems="center">
        <Button size="small" variant="contained" onClick={onOpenFile}>
          打开文件
        </Button>
        <Tooltip title="更多文件操作">
          <span>
            <IconButton
              size="small"
              aria-label="更多文件操作"
              disabled={!currentFile}
              onClick={handleOpenFileMenu}
            >
              <MoreHorizIcon fontSize="small" />
            </IconButton>
          </span>
        </Tooltip>
        <Menu
          anchorEl={fileMenuAnchor}
          open={fileMenuOpen}
          onClose={handleCloseFileMenu}
          anchorOrigin={{ vertical: 'bottom', horizontal: 'right' }}
          transformOrigin={{ vertical: 'top', horizontal: 'right' }}
        >
          <MenuItem onClick={handleCloseFile}>关闭文件</MenuItem>
        </Menu>
        <Tooltip title="设置">
          <IconButton size="small" aria-label="设置" onClick={onOpenSettings}>
            <SettingsIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Stack>
    </Box>
  )
}
