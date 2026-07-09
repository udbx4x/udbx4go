import { ThemeProvider } from '@mui/material/styles'
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { defaultViewerSettings } from '../settings/viewerSettings'
import type { ViewerSettings } from '../settings/viewerSettings'
import { viewerTheme } from '../theme/viewerTheme'
import { SettingsDialog } from './SettingsDialog'

function renderDialog(props: Partial<React.ComponentProps<typeof SettingsDialog>> = {}) {
  return render(
    <ThemeProvider theme={viewerTheme}>
      <SettingsDialog
        open
        settings={defaultViewerSettings}
        disabled={false}
        onClose={vi.fn()}
        onSave={vi.fn()}
        onReset={vi.fn()}
        {...props}
      />
    </ThemeProvider>,
  )
}

describe('SettingsDialog', () => {
  it('展示空间预览设置并保存草稿修改', async () => {
    const onSave = vi.fn()

    renderDialog({ onSave })

    expect(screen.getByRole('tab', { name: '空间预览' })).toHaveAttribute('aria-selected', 'true')

    const featureLimit = screen.getByLabelText('空间预览要素上限')
    const vertexBudget = screen.getByLabelText('空间预览顶点预算')
    const autoFit = screen.getByRole('switch', { name: '加载图层后自动适配范围' })

    fireEvent.change(featureLimit, { target: { value: '250' } })
    fireEvent.change(vertexBudget, { target: { value: '5000' } })
    fireEvent.click(autoFit)
    fireEvent.click(screen.getByRole('button', { name: '保存' }))

    expect(onSave).toHaveBeenCalledWith({
      ...defaultViewerSettings,
      spatialPreview: {
        featureLimit: 250,
        vertexBudget: 5000,
        autoFitOnLayerChange: false,
      },
    } satisfies ViewerSettings)
  })

  it('恢复默认前确认，确认后调用 onReset', async () => {
    const onReset = vi.fn()
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(true)

    renderDialog({ onReset })

    fireEvent.click(screen.getByRole('button', { name: '恢复默认' }))

    expect(confirmSpy).toHaveBeenCalledWith('确定要恢复默认设置吗？')
    expect(onReset).toHaveBeenCalledTimes(1)

    confirmSpy.mockRestore()
  })

  it('disabled 为 true 时禁用恢复默认和保存按钮', () => {
    renderDialog({ disabled: true })

    expect(screen.getByRole('button', { name: '恢复默认' })).toBeDisabled()
    expect(screen.getByRole('button', { name: '保存' })).toBeDisabled()
  })
})
