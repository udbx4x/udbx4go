import React from 'react'
import {
  Box,
  Button,
  CircularProgress,
  IconButton,
  Stack,
  Tooltip,
  Typography,
} from '@mui/material'
import {
  Close as CloseIcon,
  KeyboardArrowDown as CollapseTableIcon,
  KeyboardArrowUp as ExpandTableIcon,
  Settings as SettingsIcon,
} from '@mui/icons-material'
import { viewerLayout } from '../theme/viewerTheme'

interface TopToolbarProps {
  currentFile: string | null
  loading: boolean
  tableOpen: boolean
  onOpenFile: () => void
  onCloseFile: () => void
  onToggleTable: () => void
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
  tableOpen,
  onOpenFile,
  onCloseFile,
  onToggleTable,
  onOpenSettings,
}) => {
  const tableToggleLabel = tableOpen ? '收起属性表' : '展开属性表'

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
      >
        {getFileName(currentFile)}
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
        <Button
          size="small"
          variant="outlined"
          color="inherit"
          startIcon={<CloseIcon fontSize="small" />}
          disabled={!currentFile}
          onClick={onCloseFile}
        >
          关闭文件
        </Button>
        <Tooltip title={tableToggleLabel}>
          <IconButton size="small" aria-label={tableToggleLabel} onClick={onToggleTable}>
            {tableOpen ? <CollapseTableIcon fontSize="small" /> : <ExpandTableIcon fontSize="small" />}
          </IconButton>
        </Tooltip>
        <Tooltip title="设置">
          <IconButton size="small" aria-label="设置" onClick={onOpenSettings}>
            <SettingsIcon fontSize="small" />
          </IconButton>
        </Tooltip>
      </Stack>
    </Box>
  )
}
