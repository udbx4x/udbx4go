import React, { useEffect } from 'react'
import {
  Box,
  Button,
  Dialog,
  DialogActions,
  DialogContent,
  DialogTitle,
  Divider,
  FormControlLabel,
  Stack,
  Switch,
  Tab,
  Tabs,
  TextField,
  Typography,
} from '@mui/material'
import type { ViewerSettings } from '../settings/viewerSettings'
import { viewerSettingsConstraints } from '../settings/viewerSettings'

interface SettingsDialogProps {
  open: boolean
  settings: ViewerSettings
  disabled: boolean
  onClose: () => void
  onSave: (settings: ViewerSettings) => void | Promise<void>
  onReset: () => void | Promise<void>
}

type SettingsTab = 'spatialPreview' | 'mapInteraction' | 'table' | 'advanced'

const cloneSettings = (settings: ViewerSettings): ViewerSettings => ({
  spatialPreview: { ...settings.spatialPreview },
  mapInteraction: { ...settings.mapInteraction },
  table: { ...settings.table },
  advanced: { ...settings.advanced },
})

const isIntegerInRange = (value: number, min: number, max: number) => (
  Number.isInteger(value) && value >= min && value <= max
)

const getRangeHelperText = (min: number, max: number) => `请输入 ${min} 到 ${max} 之间的整数`

export const SettingsDialog: React.FC<SettingsDialogProps> = ({
  open,
  settings,
  disabled,
  onClose,
  onSave,
  onReset,
}) => {
  const [tab, setTab] = React.useState<SettingsTab>('spatialPreview')
  const [draft, setDraft] = React.useState<ViewerSettings>(() => cloneSettings(settings))
  const featureLimitValid = isIntegerInRange(
    draft.spatialPreview.featureLimit,
    viewerSettingsConstraints.featureLimit.min,
    viewerSettingsConstraints.featureLimit.max,
  )
  const vertexBudgetValid = isIntegerInRange(
    draft.spatialPreview.vertexBudget,
    viewerSettingsConstraints.vertexBudget.min,
    viewerSettingsConstraints.vertexBudget.max,
  )
  const settingsValid = featureLimitValid && vertexBudgetValid

  useEffect(() => {
    if (open) {
      setDraft(cloneSettings(settings))
      setTab('spatialPreview')
    }
  }, [open, settings])

  const updateSpatialPreview = <Key extends keyof ViewerSettings['spatialPreview']>(
    key: Key,
    value: ViewerSettings['spatialPreview'][Key],
  ) => {
    if (disabled) {
      return
    }
    setDraft((current) => ({
      ...current,
      spatialPreview: {
        ...current.spatialPreview,
        [key]: value,
      },
    }))
  }

  const updateMapInteraction = <Key extends keyof ViewerSettings['mapInteraction']>(
    key: Key,
    value: ViewerSettings['mapInteraction'][Key],
  ) => {
    if (disabled) {
      return
    }
    setDraft((current) => ({
      ...current,
      mapInteraction: {
        ...current.mapInteraction,
        [key]: value,
      },
    }))
  }

  const updateTable = <Key extends keyof ViewerSettings['table']>(
    key: Key,
    value: ViewerSettings['table'][Key],
  ) => {
    if (disabled) {
      return
    }
    setDraft((current) => ({
      ...current,
      table: {
        ...current.table,
        [key]: value,
      },
    }))
  }

  const updateAdvanced = <Key extends keyof ViewerSettings['advanced']>(
    key: Key,
    value: ViewerSettings['advanced'][Key],
  ) => {
    if (disabled) {
      return
    }
    setDraft((current) => ({
      ...current,
      advanced: {
        ...current.advanced,
        [key]: value,
      },
    }))
  }

  const handleReset = () => {
    if (disabled) {
      return
    }
    if (window.confirm('确定要恢复默认设置吗？')) {
      void onReset()
    }
  }

  const handleClose = () => {
    if (!disabled) {
      onClose()
    }
  }

  const handleSave = () => {
    if (!disabled && settingsValid) {
      void onSave(draft)
    }
  }

  return (
    <Dialog open={open} onClose={handleClose} maxWidth="md" fullWidth>
      <DialogTitle>设置</DialogTitle>
      <DialogContent dividers sx={{ p: 0 }}>
        <Box sx={{ display: 'grid', gridTemplateColumns: '180px 1fr', minHeight: 360 }}>
          <Tabs
            orientation="vertical"
            value={tab}
            onChange={(_, nextTab: SettingsTab) => {
              if (!disabled) {
                setTab(nextTab)
              }
            }}
            aria-label="设置分类"
            sx={{ borderRight: 1, borderColor: 'divider', py: 1 }}
          >
            <Tab value="spatialPreview" label="空间预览" disabled={disabled} />
            <Tab value="mapInteraction" label="地图交互" disabled={disabled} />
            <Tab value="table" label="属性表" disabled={disabled} />
            <Tab value="advanced" label="高级" disabled={disabled} />
          </Tabs>

          <Box sx={{ p: 3 }}>
            {tab === 'spatialPreview' && (
              <Stack spacing={2.5}>
                <Typography variant="subtitle2">空间预览</Typography>
                <TextField
                  label="空间预览要素上限"
                  type="number"
                  size="small"
                  value={draft.spatialPreview.featureLimit}
                  onChange={(event) => updateSpatialPreview('featureLimit', Number(event.target.value))}
                  disabled={disabled}
                  error={!featureLimitValid}
                  helperText={!featureLimitValid
                    ? getRangeHelperText(
                      viewerSettingsConstraints.featureLimit.min,
                      viewerSettingsConstraints.featureLimit.max,
                    )
                    : undefined}
                  inputProps={{
                    min: viewerSettingsConstraints.featureLimit.min,
                    max: viewerSettingsConstraints.featureLimit.max,
                    step: 1,
                  }}
                  fullWidth
                />
                <TextField
                  label="空间预览顶点预算"
                  type="number"
                  size="small"
                  value={draft.spatialPreview.vertexBudget}
                  onChange={(event) => updateSpatialPreview('vertexBudget', Number(event.target.value))}
                  disabled={disabled}
                  error={!vertexBudgetValid}
                  helperText={!vertexBudgetValid
                    ? getRangeHelperText(
                      viewerSettingsConstraints.vertexBudget.min,
                      viewerSettingsConstraints.vertexBudget.max,
                    )
                    : undefined}
                  inputProps={{
                    min: viewerSettingsConstraints.vertexBudget.min,
                    max: viewerSettingsConstraints.vertexBudget.max,
                    step: 1,
                  }}
                  fullWidth
                />
                <FormControlLabel
                  control={
                    <Switch
                      checked={draft.spatialPreview.autoFitOnLayerChange}
                      disabled={disabled}
                      onChange={(event) => updateSpatialPreview('autoFitOnLayerChange', event.target.checked)}
                    />
                  }
                  label="加载图层后自动适配范围"
                />
              </Stack>
            )}

            {tab === 'mapInteraction' && (
              <Stack spacing={2.5}>
                <Typography variant="subtitle2">地图交互</Typography>
                <FormControlLabel
                  control={
                    <Switch
                      checked={draft.mapInteraction.zoomToSelectedFeature}
                      disabled={disabled}
                      onChange={(event) => updateMapInteraction('zoomToSelectedFeature', event.target.checked)}
                    />
                  }
                  label="选择要素时自动定位"
                />
              </Stack>
            )}

            {tab === 'table' && (
              <Stack spacing={2.5}>
                <Typography variant="subtitle2">属性表</Typography>
                <FormControlLabel
                  control={
                    <Switch
                      checked={draft.table.defaultOpen}
                      disabled={disabled}
                      onChange={(event) => updateTable('defaultOpen', event.target.checked)}
                    />
                  }
                  label="默认展开属性表"
                />
              </Stack>
            )}

            {tab === 'advanced' && (
              <Stack spacing={2.5}>
                <Typography variant="subtitle2">高级</Typography>
                <FormControlLabel
                  control={
                    <Switch
                      checked={draft.advanced.showPreviewStats}
                      disabled={disabled}
                      onChange={(event) => updateAdvanced('showPreviewStats', event.target.checked)}
                    />
                  }
                  label="显示空间预览统计"
                />
              </Stack>
            )}
          </Box>
        </Box>
      </DialogContent>
      <Divider />
      <DialogActions>
        <Button color="inherit" disabled={disabled} onClick={handleReset}>
          恢复默认
        </Button>
        <Box sx={{ flex: 1 }} />
        <Button color="inherit" disabled={disabled} onClick={onClose}>
          取消
        </Button>
        <Button variant="contained" disabled={disabled || !settingsValid} onClick={handleSave}>
          保存
        </Button>
      </DialogActions>
    </Dialog>
  )
}
