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
  tableOpen: boolean
  onOpenFile: () => void
  onCloseFile: () => void
  onToggleTable: () => void
}

let TopToolbar: React.FC<TopToolbarProps>

const renderToolbar = (props: Partial<TopToolbarProps> = {}) => {
  const defaultProps: TopToolbarProps = {
    currentFile: null,
    loading: false,
    tableOpen: true,
    onOpenFile: vi.fn(),
    onCloseFile: vi.fn(),
    onToggleTable: vi.fn(),
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

  it('未打开文件时显示状态并禁用关闭文件按钮', () => {
    renderToolbar()

    expect(screen.getByText('未打开文件')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '关闭文件' })).toBeDisabled()
  })

  it('有当前文件时显示文件名并触发工具栏操作', async () => {
    const onOpenFile = vi.fn()
    const onCloseFile = vi.fn()
    const onToggleTable = vi.fn()

    renderToolbar({
      currentFile: '/tmp/SampleData.udbx',
      onOpenFile,
      onCloseFile,
      onToggleTable,
    })

    expect(screen.getByText('SampleData.udbx')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '打开文件' }))
    fireEvent.click(screen.getByRole('button', { name: '关闭文件' }))
    fireEvent.click(screen.getByRole('button', { name: '收起属性表' }))

    expect(onOpenFile).toHaveBeenCalledTimes(1)
    expect(onCloseFile).toHaveBeenCalledTimes(1)
    expect(onToggleTable).toHaveBeenCalledTimes(1)
  })
})
