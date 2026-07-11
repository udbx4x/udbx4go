import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { AttributeTableDrawer } from './AttributeTableDrawer'
import type { PageData } from '../types'

const pageData: PageData = {
  columns: ['SmID', '名称'],
  rows: [
    ['1', '道路'],
    ['2', '河流'],
  ],
  currentPage: 1,
  totalPages: 3,
}

const baseProps = {
  pageData,
  datasetName: 'BaseMap_P',
  selectedFeature: null,
  onModeChange: vi.fn(),
  onFeatureSelect: vi.fn(),
  onPageChange: vi.fn(),
}

describe('AttributeTableDrawer', () => {
  it('collapsed 模式只显示标题摘要和控制按钮，不渲染表格', () => {
    const onModeChange = vi.fn()

    render(
      <AttributeTableDrawer
        {...baseProps}
        mode="collapsed"
        onModeChange={onModeChange}
      />,
    )

    expect(screen.getByText('BaseMap_P')).toBeInTheDocument()
    expect(screen.getByText('第 1 / 3 页 · 本页 2 条记录')).toBeInTheDocument()
    expect(screen.queryByRole('grid')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '半展开属性表' }))

    expect(onModeChange).toHaveBeenCalledWith('half')
  })

  it('half 模式渲染紧凑表格和头部分页', () => {
    const onPageChange = vi.fn()

    render(
      <AttributeTableDrawer
        {...baseProps}
        mode="half"
        onPageChange={onPageChange}
      />,
    )

    expect(screen.getByRole('grid')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '半展开属性表' })).toBeDisabled()
    const pagination = screen.getByLabelText('属性表分页')

    fireEvent.click(within(pagination).getByRole('button', { name: /go to page 2/i }))

    expect(onPageChange).toHaveBeenCalledWith(2)
  })

  it('full 模式禁用全展开按钮并渲染表格', () => {
    render(
      <AttributeTableDrawer
        {...baseProps}
        mode="full"
      />,
    )

    expect(screen.getByRole('button', { name: '全展开属性表' })).toBeDisabled()
    expect(screen.getByRole('grid')).toBeInTheDocument()
  })
})
