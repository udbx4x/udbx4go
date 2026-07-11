import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useUDBX } from './useUDBX'

const mocks = vi.hoisted(() => ({
  GetDatasetSpatialSummary: vi.fn(),
  LoadSpatialPreview: vi.fn(),
  OpenFileDialog: vi.fn(),
  OpenUDBXFile: vi.fn(),
  CloseUDBXFile: vi.fn(),
  ListDatasets: vi.fn(),
  LoadDatasetPage: vi.fn(),
  GetCurrentFile: vi.fn(),
  GetFeatureAttributes: vi.fn(),
}))

vi.mock('../../wailsjs/go/main/App', () => mocks)

describe('useUDBX spatial preview settings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mocks.LoadDatasetPage.mockResolvedValue({
      columns: ['SmID'],
      rows: [['1']],
      currentPage: 1,
      totalPages: 1,
    })
    mocks.GetDatasetSpatialSummary.mockResolvedValue({
      datasetName: 'BaseMap_P',
      kind: 'point',
      objectCount: 1,
      estimatedVertexCount: 1,
      previewSupported: true,
    })
    mocks.LoadSpatialPreview.mockResolvedValue({
      datasetName: 'BaseMap_P',
      kind: 'point',
      features: [],
      estimatedVertexCount: 0,
      sampled: false,
    })
  })

  it('加载空间预览时使用 settings 提供的 feature limit 和 vertex budget', async () => {
    const { result } = renderHook(() =>
      useUDBX({
        spatialPreviewFeatureLimit: 1234,
        spatialPreviewVertexBudget: 567890,
      }),
    )

    await act(async () => {
      await result.current.addDatasetToMap('BaseMap_P')
    })

    expect(mocks.LoadSpatialPreview).toHaveBeenCalledTimes(1)
    expect(mocks.LoadSpatialPreview).toHaveBeenCalledWith(
      'BaseMap_P',
      expect.objectContaining({
        limit: 1234,
        maxVertices: 567890,
        simplify: false,
      }),
    )
  })

  it('属性表加载失败时设置错误状态并保留当前表格状态', async () => {
    mocks.LoadDatasetPage.mockRejectedValue(new Error("dataset kind 'unknown' is not supported"))

    const { result } = renderHook(() =>
      useUDBX({
        spatialPreviewFeatureLimit: 1000,
        spatialPreviewVertexBudget: 100000,
      }),
    )

    await act(async () => {
      await expect(result.current.loadTableDataset('modeldt')).rejects.toThrow('not supported')
    })

    expect(result.current.error).toBe("dataset kind 'unknown' is not supported")
    expect(result.current.activeTableDataset).toBeNull()
    expect(result.current.pageData).toBeNull()
    expect(result.current.loading).toBe(false)
  })
})
