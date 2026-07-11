import { ThemeProvider } from '@mui/material/styles'
import { fireEvent, render, screen } from '@testing-library/react'
import { beforeAll, describe, expect, it, vi } from 'vitest'
import { viewerTheme } from '../theme/viewerTheme'

declare global {
  interface Window {
    __vite_plugin_react_preamble_installed__?: boolean
  }
}

type TopToolbarProps = {
  currentFile: string | null
  loading: boolean
  onOpenFile: () => void
  onCloseFile: () => void
  onOpenSettings: () => void
}

let TopToolbar: React.FC<TopToolbarProps>

const renderToolbar = (props: Partial<TopToolbarProps> = {}) => {
  const defaultProps: TopToolbarProps = {
    currentFile: null,
    loading: false,
    onOpenFile: vi.fn(),
    onCloseFile: vi.fn(),
    onOpenSettings: vi.fn(),
  }

  return {
    ...render(
      <ThemeProvider theme={viewerTheme}>
        <TopToolbar {...defaultProps} {...props} />
      </ThemeProvider>,
    ),
    props: { ...defaultProps, ...props },
  }
}

describe('TopToolbar', () => {
  beforeAll(async () => {
    window.__vite_plugin_react_preamble_installed__ = true
    TopToolbar = (await import('./TopToolbar')).TopToolbar
  })

  it('未打开文件时显示状态并禁用更多文件操作', () => {
    renderToolbar()

    expect(screen.getByText('未打开文件')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '打开文件' })).toBeEnabled()
    expect(screen.getByRole('button', { name: '更多文件操作' })).toBeDisabled()
    expect(screen.queryByRole('button', { name: '收起属性表' })).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '展开属性表' })).not.toBeInTheDocument()
  })

  it('有当前文件时显示文件名并通过菜单关闭文件', async () => {
    const onOpenFile = vi.fn()
    const onCloseFile = vi.fn()

    renderToolbar({
      currentFile: '/tmp/SampleData.udbx',
      onOpenFile,
      onCloseFile,
    })

    expect(screen.getByText('SampleData.udbx')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '打开文件' }))
    fireEvent.click(screen.getByRole('button', { name: '更多文件操作' }))
    fireEvent.click(screen.getByRole('menuitem', { name: '关闭文件' }))

    expect(onOpenFile).toHaveBeenCalledTimes(1)
    expect(onCloseFile).toHaveBeenCalledTimes(1)
  })

  it('点击设置按钮会调用设置回调', () => {
    const onOpenSettings = vi.fn()

    renderToolbar({ onOpenSettings })

    fireEvent.click(screen.getByRole('button', { name: '设置' }))

    expect(onOpenSettings).toHaveBeenCalledTimes(1)
  })
})
